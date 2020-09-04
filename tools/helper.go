package tools

import (
	"go/token"

	mapset "github.com/deckarep/golang-set"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"
)


func NewBlockFlowController(firstBlock *ssa.BasicBlock) *blockFlowController {
	return &blockFlowController{
		nextBlock:    []*ssa.BasicBlock{firstBlock},
		doneBlock:    mapset.NewSet(),
		blockFlowIn:  make(map[*ssa.BasicBlock]mapset.Set),
		blockFlowOut: make(map[*ssa.BasicBlock]mapset.Set),
	}
}

type blockFlowController struct {
	nextBlock    []*ssa.BasicBlock
	doneBlock    mapset.Set
	blockFlowOut map[*ssa.BasicBlock]mapset.Set
	blockFlowIn  map[*ssa.BasicBlock]mapset.Set
}

// blockごとに見て、そのブロックまででその変数が使われているかどうかを見る
func (r *blockFlowController) AddBlockEdge(from *ssa.BasicBlock, to *ssa.BasicBlock) {
	if _, ok := r.blockFlowOut[from]; !ok {
		r.blockFlowOut[from] = mapset.NewSet()
	}
	if _, ok := r.blockFlowIn[to]; !ok {
		r.blockFlowIn[to] = mapset.NewSet()
	}
	r.blockFlowOut[from].Add(to)
	r.blockFlowIn[to].Add(from)
}

func (r *blockFlowController) GetNextBlock() *ssa.BasicBlock {
	if _, ok := r.blockFlowOut[r.nextBlock[0]]; !ok {
		return nil
	}

	for b := range r.blockFlowOut[r.nextBlock[0]].Iterator().C {
		b, ok := b.(*ssa.BasicBlock)
		if !ok {
			panic(nil)
		}
		r.nextBlock = append(r.nextBlock, b)
	}
	r.doneBlock.Add(r.nextBlock[0])
	r.nextBlock = r.nextBlock[1:]

	var nextIndex int
	for i, b := range r.nextBlock {
		canStart := true
		for inBlock := range r.blockFlowIn[b].Iterator().C {
			inBlock, ok := inBlock.(*ssa.BasicBlock)
			if !ok {
				panic(nil)
			}
			if !r.doneBlock.Contains(inBlock) {
				canStart = false
				break
			}
		}
		if canStart {
			nextIndex = i
			break
		}
	}
	// 長さ0になったら終了
	if len(r.nextBlock) == 0 {
		return nil
	}
	temp := r.nextBlock[nextIndex]
	r.nextBlock[nextIndex] = r.nextBlock[0]
	r.nextBlock[0] = temp
	return r.nextBlock[0]
}

func (r *blockFlowController) getCurrentBlock() *ssa.BasicBlock {
	return r.nextBlock[0]
}

func (r *blockFlowController) getBlocksPreds() []*ssa.BasicBlock {
	if _, ok := r.blockFlowIn[r.nextBlock[0]]; !ok {
		return nil
	}
	predsInterfaceSlice := r.blockFlowIn[r.nextBlock[0]].ToSlice()
	var predsBasicBlock []*ssa.BasicBlock
	for _, p := range predsInterfaceSlice {
		p, ok := p.(*ssa.BasicBlock)
		if !ok {
			panic("Basic Block以外が入ってしまっている？？？")
		}
		predsBasicBlock = append(predsBasicBlock, p)
	}
	return predsBasicBlock
}

type varInfo struct {
	pos                              token.Pos
	isUsedFromSameBlock              bool
	isUsedFromBeforeBlock            bool
	isReassignedBeforeUseInNextBlock bool
}

func (v *varInfo) isUnused() bool {
	return !(v.isUsedFromSameBlock || !v.isReassignedBeforeUseInNextBlock)
}

func (v varInfo) copy() varInfo {
	return varInfo{
		pos:                              v.pos,
		isUsedFromSameBlock:              v.isUsedFromSameBlock,
		isUsedFromBeforeBlock:            v.isUsedFromBeforeBlock,
		isReassignedBeforeUseInNextBlock: v.isReassignedBeforeUseInNextBlock,
	}
}

type unusedReport struct {
	pos     token.Pos
	message string
}

type unusedVarInfo struct {
	pos   token.Pos
	block *ssa.BasicBlock
}

func NewResubstitutionManager() *resubstitutionManager {
	return &resubstitutionManager{
		callMap:               make(map[*ssa.BasicBlock]map[string]varInfo),
		lastUnusedVar:         make(map[*ssa.BasicBlock]map[string][]unusedVarInfo),
		unusedrResubstitution: []unusedReport{},
	}
}

type resubstitutionManager struct {
	callMap map[*ssa.BasicBlock]map[string]varInfo
	// 各blockの直前までの使用状況.各ブロックの直前までのブロックで、unusedである変数の情報をもつ
	lastUnusedVar         map[*ssa.BasicBlock]map[string][]unusedVarInfo
	unusedrResubstitution []unusedReport
}

func (r *resubstitutionManager) StoreVarAt(block *ssa.BasicBlock, addrName *string, pos token.Pos, useAddrName *string, bf *blockFlowController) {
	// 方針: 直前のやつをreportする. 最後にunusedかつ未報告のものをまとめてreportする
	_, ok := r.callMap[block]
	// もしそのブロック初めての呼び出しであれば直前のブロックまでの情報の全てを統合したmapを作成する
	if !ok {
		r.callMap[block] = make(map[string]varInfo)
		r.lastUnusedVar[block] = make(map[string][]unusedVarInfo)
		preds := bf.getBlocksPreds()
		if preds == nil {
			for _, predBlock := range preds {
				for name := range r.lastUnusedVar[predBlock] {
					_, ok := r.lastUnusedVar[block][name]
					if !ok {
						r.lastUnusedVar[block][name] = make([]unusedVarInfo, 0)
					}
					if _, ok := r.callMap[predBlock][name]; ok {
						// １度は直前のブロックで使用されたがその後の再代入後まだ未使用
						if r.callMap[predBlock][name].isUsedFromSameBlock == false {
							r.lastUnusedVar[block][name] = append(r.lastUnusedVar[block][name], unusedVarInfo{pos: r.callMap[predBlock][name].pos, block: predBlock})
						}
					} else {
						// 直前のブロックで一度も使用されていないのでその直前ブロック開始時点での未使用情報を付加
						if _, ok := r.lastUnusedVar[predBlock][name]; ok {
							r.lastUnusedVar[block][name] = append(r.lastUnusedVar[block][name], r.lastUnusedVar[predBlock][name]...)
						}
					}
				}
			}
		}
	}
	if addrName != nil {
		v, ok := r.callMap[block][*addrName]
		// そのブロック内での最初のその変数の割り当ての場合
		if !ok {
			// TODO: storeVarAt の直前のblock全てのの*addrnameの使用状況を見てここのisUsedFromSameBlockを制御すべき

			// 直前のブロック見て、そこでunusedであれば直前のブロック内の変数に対してエラー出す
			preds := bf.getBlocksPreds()
			for _, predBlock := range preds {
				vInfo, ok := r.callMap[predBlock][*addrName]
				if !ok || vInfo.isUsedFromSameBlock {
					continue
				}
				r.unusedrResubstitution = append(r.unusedrResubstitution, unusedReport{vInfo.pos, "Resubstitution before used"})

				r.callMap[predBlock][*addrName] = varInfo{
					pos:                              vInfo.pos,
					isUsedFromSameBlock:              vInfo.isUsedFromSameBlock,
					isReassignedBeforeUseInNextBlock: true,
				}
			}
		} else {
			// 同一ブロック内すでに割り当てありの場合
			// 同一ブロック内で最後に呼び出された部分でisUsedFromSameBlock==falseであればその直前の変数部分もreportする
			if !v.isUsedFromSameBlock {
				r.unusedrResubstitution = append(r.unusedrResubstitution, unusedReport{v.pos, "Resubstitutioned before used"})
			}
		}
		r.callMap[block][*addrName] = varInfo{
			pos:                              pos,
			isUsedFromSameBlock:              false,
			isReassignedBeforeUseInNextBlock: false,
		}
	}

	if useAddrName != nil {
		v, ok := r.callMap[block][*useAddrName]
		if !ok {
			// 直前のブロックまでで1つでもunusedであれば
			preds := bf.getBlocksPreds()
			for _, predBlock := range preds {
				vInfo, ok := r.callMap[predBlock][*addrName]
				if !ok || vInfo.isUsedFromSameBlock {
					continue
				}
				r.unusedrResubstitution = append(r.unusedrResubstitution, unusedReport{vInfo.pos, "Resubstitution before used"})

				r.callMap[predBlock][*addrName] = varInfo{
					pos:                              vInfo.pos,
					isUsedFromSameBlock:              vInfo.isUsedFromSameBlock,
					isReassignedBeforeUseInNextBlock: true,
				}
			}
		}
		r.callMap[block][*useAddrName] = varInfo{
			pos:                              v.pos,
			isUsedFromSameBlock:              true,
			isReassignedBeforeUseInNextBlock: false,
		}
	}
}

func (r *resubstitutionManager) CalcUnusedrResubstitution() {
	// TODO: 最後にunusedのもので拾われていないものを全部持ってくる. 最後にassigneされたが一度も使用されてないものだとか
	for b := range r.callMap {
		for name := range r.callMap[b] {
			r.unusedrResubstitution = append(r.unusedrResubstitution, unusedReport{pos: r.callMap[b][name].pos, message: "Resubstitutioned before used"})
		}
	}
	temp := mapset.NewSet()
	for _, i := range r.unusedrResubstitution {
		temp.Add(i.pos)
	}
	ret := []unusedReport{}
	
	for i := range temp.Iterator().C {
		i, ok := i.(token.Pos)
		if !ok {
			panic("!!!!!!!!!!!!!!!")
		}
		ret = append(ret, unusedReport{pos: i,message: "Resubstitutioned before used"})
	}
	r.unusedrResubstitution = ret
}

func (r *resubstitutionManager) Report(pass *analysis.Pass) {
	// TODO: reportフィールドに入っているものをposでソートしてreportする
	for _, i := range r.unusedrResubstitution {
		pass.Reportf(i.pos, i.message)
	}
}
