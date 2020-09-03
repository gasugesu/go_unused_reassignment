package gounusedreassignment

import (
	"container/list"
	"fmt"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	mapset "github.com/deckarep/golang-set"
)

const doc = "go_unused_reassignment is ..."

// Analyzer is ...
var Analyzer = &analysis.Analyzer{
	Name: "go_unused_reassignment",
	Doc:  doc,
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {

	mode := ssa.BuilderMode(ssa.NaiveForm)
	// copied from golang.org/x/tools/go/analysis/passes/buildssa/buildssa.go
	prog := ssa.NewProgram(pass.Fset, mode)

	created := make(map[*types.Package]bool)
	var createAll func(pkgs []*types.Package)
	createAll = func(pkgs []*types.Package) {
		for _, p := range pkgs {
			if !created[p] {
				created[p] = true
				prog.CreatePackage(p, nil, nil, true)
				createAll(p.Imports())
			}
		}
	}
	createAll(pass.Pkg.Imports())

	ssapkg := prog.CreatePackage(pass.Pkg, pass.Files, pass.TypesInfo, false)
	ssapkg.Build()

	// TODO: これで全部取れているか検証必要
	srcFuncs := ssautil.AllFunctions(prog)

	for f := range srcFuncs {
		fmt.Println(f, f.Name())
		fmt.Println(f.Locals)
		// TODO: initを処理するかどうか検討
		if f.Name() == "init" {
			continue
		}

		mgr := newResubstitutionManager()

		for _, block := range f.Blocks {
			fmt.Printf("\tBlock %d\n", block.Index)
			for _, instr := range block.Instrs {
				fmt.Printf("\t\t%[1]T\t%[1]v(%[1]p)\n", instr)
				switch instr := instr.(type) {
				case *ssa.Store:
					storeToAddrName := new(string)
					useAddrName := new(string)
					for _, value := range instr.Operands(nil) {
						switch value := (*value).(type) {
						case *ssa.Alloc:
							*storeToAddrName = value.Name()
							fmt.Printf("\t\t\t%[1]T\t%[1]v(%[1]p)\n", value)
							fmt.Println("\t\t\tAddrName: ", value.Name())
						case *ssa.BinOp, *ssa.UnOp, *ssa.Function:
							// TODO: defaultへ移植？ 想定外のものがたくさんありそう,チャンネル周りとかphi周りとかClousureとか
							*useAddrName = value.Name()
							fmt.Printf("\t\t\t%[1]T\t%[1]v(%[1]p)\n", value)
							fmt.Println("\t\t\tAddrName: ", value.Name())
						default:
							fmt.Printf("\t\t\t%[1]T\t%[1]v(%[1]p)\n", value)
						}
					}
					mgr.storeVarAt(block, storeToAddrName, instr.Pos(), useAddrName)
				case *ssa.UnOp:
					fmt.Println("\t\t", instr.Name(), "Op: ", instr.Op, "Val1: ", instr.X)
					storeToAddrName := new(string)
					useAddrName := new(string)
					*storeToAddrName = instr.Name()
					// TODO: 「CommaOk and Op=ARROW」への対応. この時len(Operands)>1となる.
					for _, value := range instr.Operands(nil) {
						fmt.Printf("\t\t\t%[1]T\t%[1]v(%[1]p)\n", value)
						fmt.Println("\t\t\t", (*value).Name(), (*value).Referrers())
						// CommaOk and Op=ARROW の場合ここで前の値が握り潰される
						*useAddrName = (*value).Name()
					}
					mgr.storeVarAt(block, storeToAddrName, instr.Pos(), useAddrName)
				case *ssa.BinOp:
					fmt.Println("\t\t", instr.Name(), "Op: ", instr.Op, "Val1: ", instr.X, "Val2: ", instr.Y)
					storeToAddrName := new(string)
					useAddrName := new(string)
					*storeToAddrName = instr.Name()
					for _, value := range instr.Operands(nil) {
						fmt.Printf("\t\t\t%[1]T\t%[1]v(%[1]p)\n", value)
						fmt.Println("\t\t\t", (*value).Name(), (*value).Referrers())
						// TODO: 別の方法検討ssa.Const的なやつが何故かssa.Valueとして入ってくるのを防ぐ
						refPtr := (*value).Referrers()
						if refPtr == nil {
							continue
						}
						if len(*refPtr) > 0 {
							*useAddrName = (*value).Name()
							mgr.storeVarAt(block, storeToAddrName, instr.Pos(), useAddrName)
						}
					}
				case *ssa.If:
					for _, value := range instr.Operands(nil) {
						fmt.Printf("\t\t\t%[1]T\t%[1]v(%[1]p)\n", value)
						fmt.Println("\t\t\t", (*value).Name(), (*value).Referrers())
						// TODO: 別の方法検討ssa.Const的なやつが何故かssa.Valueとして入ってくるのを防ぐ
						useAddrName := new(string)
						refPtr := (*value).Referrers()
						if refPtr == nil {
							continue
						}
						if len(*refPtr) > 0 {
							*useAddrName = (*value).Name()
							mgr.storeVarAt(block, nil, instr.Pos(), useAddrName)
						}
					}
				case *ssa.Call:
					// TODO: to be implemented
					fmt.Println("\t\t", instr.Name())
					storeToAddrName := new(string)
					*storeToAddrName = instr.Name()
					for _, value := range instr.Operands(nil) {
						fmt.Printf("\t\t\t%[1]T\t%[1]v(%[1]p)\n", value)
						fmt.Println("\t\t\t", (*value).Name(), (*value).Referrers())
						// TODO: 別の方法検討ssa.Const的なやつが何故かssa.Valueとして入ってくるのを防ぐ
						useAddrName := new(string)
						refPtr := (*value).Referrers()
						if refPtr == nil {
							continue
						}
						if len(*refPtr) > 0 {
							*useAddrName = (*value).Name()
							mgr.storeVarAt(block, storeToAddrName, instr.Pos(), useAddrName)
						}
					}
				case *ssa.Return:
					for _, value := range instr.Operands(nil) {
						fmt.Printf("\t\t\t%[1]T\t%[1]v(%[1]p)\n", value)
						fmt.Println("\t\t\t", (*value).Name(), (*value).Referrers())
						useAddrName := new(string)
						refPtr := (*value).Referrers()
						if refPtr == nil {
							continue
						}
						if len(*refPtr) > 0 {
							*useAddrName = (*value).Name()
							mgr.storeVarAt(block, nil, instr.Pos(), useAddrName)
						}
					}
				default:
					for _, o := range instr.Operands(nil) {
						fmt.Println(*o)
					}
				}
			}
		}
		mgr.calcUnusedrResubstitution()
		mgr.report()
	}
	return nil, nil
}

func newResubstitutionManager() *ResubstitutionManager {
	return &ResubstitutionManager{
		unusedrResubstitution: []token.Pos{},
	}
}

type ResubstitutionManager struct {
	callMap               map[string]
	unusedrResubstitution []token.Pos
}

func (r *ResubstitutionManager) storeVarAt(block *ssa.BasicBlock, addrName *string, pos token.Pos, useAddrName *string) {
	// TODO: 未実装
	// ここでうまくグラフを構築したい

}

func (r *ResubstitutionManager) use(block *ssa.BasicBlock, addrName *string) {
	// TODO: storeでaddrNameにnilを許すとここ不要
}

func (r *ResubstitutionManager) calcUnusedrResubstitution() {
	// TODO: 構築されたグラフを後ろから見て依存関係のないところは全てreportに挿入&削除
}

func (r *ResubstitutionManager) report() {
	// TODO: reportフィールドに入っているものをposでソートしてreportする

}
