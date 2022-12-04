package chunkStorage

import (
	"log"

	"github.com/Tnze/go-mc/save"
)

func ConvFlexibleNBTtoSave(d []byte) (ret *save.Chunk, err error) {
	ret = &save.Chunk{}
	err = ret.Load(d)
	if err != nil {
		log.Print(err)
	}
	return
}
