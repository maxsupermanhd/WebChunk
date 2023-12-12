package primitives

import "fmt"

type ImageLocation struct {
	World, Dimension, Variant string
	S, X, Z                   int
}

func (i ImageLocation) String() string {
	return fmt.Sprintf("{%s:%s:%s at %ds %dx %dz}", i.World, i.Dimension, i.Variant, i.S, i.X, i.Z)
}
