package chunkStorage

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"io"
	"log"

	"github.com/Tnze/go-mc/save"
)

func ConvFlexibleNBTtoSave(d []byte) (ret *save.Chunk, err error) {
	var r io.Reader = bytes.NewReader(d[1:])
	switch d[0] {
	default:
		err = errors.New("unknown compression")
	case 1:
		r, err = gzip.NewReader(r)
	case 2:
		r, err = zlib.NewReader(r)
	}
	if err != nil {
		log.Println(err)
		return
	}
	// dat, err := io.ReadAll(r)
	// if err != nil {
	// 	log.Println(err)
	// 	return
	// }
	ret = &save.Chunk{}

	err = ret.Load(d)
	if err != nil {
		log.Print(err)
	}

	// var root map[string]nbt.RawMessage

	// err = nbt.Unmarshal(dat, &root)
	// if err != nil {
	// 	log.Println(err)
	// }
	// log.Println("Chunk provides:")
	// for k := range root {
	// 	log.Printf(" + %s", k)
	// }
	return
}
