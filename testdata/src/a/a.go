package a

func temp(a int) int {
	return a
}

var f = func(p int) int{
	tt := temp
	a := 987 + tt(p)
	for i:=0;i<10;i++ {
		a += i
	}
	b := 12345
	a,b = b,a
	a += 10
	if p == 10 {
		p = 54321
	}else {
		a = 39475
	}
	p = a + b
	a = 12345
	a += 20 // want "unused"
	a += 20 // want "unused"
	a = a + 5124 // want "unused"
	a += 30 // want "unused"
	a += 40 // want "unused"
	return p
}
