# go_unused_reassignment(未完成)

再代入後に使用されていないものを指摘する静的解析ツール


```golang
var f = func(p int) int{
	a := 987 + temp(p)
	for i:=0;i<10;i++ {
		a += i // want "Resubstitutioned before used"
	}
	b := 12345
	a,b = b,a
	a += 10
	if p == 10 {
		p = 54321 // want "Resubstitutioned before used"
	}else {
		a = 39475
	}

	if p == 100 {
		p = 54321 // want "Resubstitutioned before used"
	}else {
		a = 39475
	}

	p = 12340000 // want "Resubstitutioned before used"
	p = a + b
	a = 12345 // want "Resubstitutioned before used"
	a += 20 // want "Resubstitutioned before used"
	a += 20 // want "Resubstitutioned before used"
	a = a + 5124 // want "Resubstitutioned before used"
	a += 30 // want "Resubstitutioned before used"
	a += 40 // want "Resubstitutioned before used"
	return p
}
```
