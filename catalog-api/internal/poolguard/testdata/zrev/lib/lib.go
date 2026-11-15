package lib

import "fmt"

type I interface{ fmt.Stringer }
type Deep struct{}

func (Deep) String() string { return "pure" }

type Mid struct{ Deep }
type Outer struct {
	I
	Mid
}
