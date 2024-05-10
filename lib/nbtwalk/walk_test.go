package nbtwalk

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"testing"

	"github.com/maxsupermanhd/go-vmc/v764/nbt"
)

func TestBasicCompound(t *testing.T) {
	appendName := func(data []byte, name string) []byte {
		ret := binary.BigEndian.AppendUint16(data, uint16(len(name)))
		ret = append(ret, []byte(name)...)
		return ret
	}

	data := []byte{nbt.TagCompound}
	data = appendName(data, "Testing")

	data = append(data, nbt.TagString)
	data = appendName(data, "Hello")
	data = appendName(data, "World")

	data = append(data, nbt.TagCompound)
	data = appendName(data, "Arrays")

	data = append(data, nbt.TagByteArray)
	data = appendName(data, "The112233")
	data = append(data, 0, 0, 0, 3)
	data = append(data, 11, 22, 33)

	data = append(data, nbt.TagIntArray)
	data = appendName(data, "NumberNine")
	data = append(data, 0, 0, 0, 1)
	data = binary.BigEndian.AppendUint32(data, 9)

	data = append(data, nbt.TagLongArray)
	data = appendName(data, "NumberNineNine")
	data = append(data, 0, 0, 0, 1)
	data = binary.BigEndian.AppendUint64(data, 99)

	data = append(data, nbt.TagEnd)

	data = append(data, nbt.TagByte)
	data = appendName(data, "Nice")
	data = append(data, 0x69)

	data = append(data, nbt.TagEnd)

	fmt.Print(hex.Dump(data))

	err := WalkNBT(data, &WalkerCallbacks{
		CbEnd: func(p []NBTnode) {
			log.Printf("%s.End", PrintNodeSlice(p))
		},
		CbByte: func(p []NBTnode, n string, val byte) {
			log.Printf("%s.Byte %q %#+v", PrintNodeSlice(p), n, val)
		},
		CbShort: func(p []NBTnode, n string, val uint16) {
			log.Printf("%s.Short %q %#+v", PrintNodeSlice(p), n, val)
		},
		CbInt: func(p []NBTnode, n string, val uint32) {
			log.Printf("%s.Int %q %#+v", PrintNodeSlice(p), n, val)
		},
		CbLong: func(p []NBTnode, n string, val uint64) {
			log.Printf("%s.Long %q %#+v", PrintNodeSlice(p), n, val)
		},
		CbFloat: func(p []NBTnode, n string, val float32) {
			log.Printf("%s.Float %q %#+v", PrintNodeSlice(p), n, val)
		},
		CbDouble: func(p []NBTnode, n string, val float64) {
			log.Printf("%s.Double %q %#+v", PrintNodeSlice(p), n, val)
		},
		CbByteArray: func(p []NBTnode, n string, val []byte) {
			log.Printf("%s.ByteArray %q %#+v", PrintNodeSlice(p), n, val)
		},
		CbString: func(p []NBTnode, n string, val string) {
			log.Printf("%s.String %q %q", PrintNodeSlice(p), n, val)
		},
		CbList: func(p []NBTnode, n string, t byte, l int) {
			log.Printf("%s.List %q %s of length %d", PrintNodeSlice(p), n, ByteTagName(t), l)
		},
		CbCompound: func(p []NBTnode, n string) {
			log.Printf("%s.Compound: %#+v", PrintNodeSlice(p), n)
		},
		CbIntArray: func(p []NBTnode, n string, val []uint32) {
			log.Printf("%s.IntArray: %#+v, %#+v", PrintNodeSlice(p), n, val)
		},
		CbLongArray: func(p []NBTnode, n string, val []uint64) {
			log.Printf("%s.LongArray: %#+v, %#+v", PrintNodeSlice(p), n, val)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}
