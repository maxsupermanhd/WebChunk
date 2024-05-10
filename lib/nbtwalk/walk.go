package nbtwalk

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/maxsupermanhd/go-vmc/v764/nbt"
)

var (
	ErrUnknownTag       = errors.New("unknown tag")
	ErrUnexpectedEndTag = errors.New("unexpected TagEnd")
	ErrNegativeSize     = errors.New("negative size")
	ErrOutOfBounds      = errors.New("out of bounds")
)

type ConextedError struct {
	E            error
	ReadingStage string
	Offset       int
}

func (err ConextedError) Error() string {
	return fmt.Sprintf("%s at %d: %s", err.E.Error(), err.Offset, err.ReadingStage)
}

type NBTnode struct {
	T      byte
	N      string
	S      int
	toRead int
}

func ByteTagName(b byte) string {
	names := []string{
		"TagEnd",
		"TagByte",
		"TagShort",
		"TagInt",
		"TagLong",
		"TagFloat",
		"TagDouble",
		"TagByteArray",
		"TagString",
		"TagList",
		"TagCompound",
		"TagIntArray",
		"TagLongArray",
	}
	if int(b) >= len(names) {
		return fmt.Sprintf("unknown tag 0x%02x", b)
	}
	return names[b]
}

func PrintNodeSlice(p []NBTnode) string {
	ret := ""
	for _, v := range p {
		if v.T == nbt.TagList {
			ret += fmt.Sprintf(".%q[%d]", v.N, v.S)
		} else {
			ret += fmt.Sprintf(".%q", v.N)
		}
	}
	return ret
}

type WalkerCallbacks struct {
	CbEnd       func(p []NBTnode)
	CbByte      func(p []NBTnode, n string, val byte)
	CbShort     func(p []NBTnode, n string, val uint16)
	CbInt       func(p []NBTnode, n string, val uint32)
	CbLong      func(p []NBTnode, n string, val uint64)
	CbFloat     func(p []NBTnode, n string, val float32)
	CbDouble    func(p []NBTnode, n string, val float64)
	CbByteArray func(p []NBTnode, n string, val []byte)
	CbString    func(p []NBTnode, n string, val string)
	CbList      func(p []NBTnode, n string, t byte, l int)
	CbCompound  func(p []NBTnode, n string)
	CbIntArray  func(p []NBTnode, n string, val []uint32)
	CbLongArray func(p []NBTnode, n string, val []uint64)
}

// inspired by github.com/rmmh/cubeographer
// reflectless nbt "parser" that shits on branch predictors
// tldr callbacks get entered and exited from nbt tree
// keep tree-poition based callbacks and swap them out
// in runtime to avoid checking trees all the time
// ps tag end in node tree means end (to not reallocate)
func WalkNBT(data []byte, cb *WalkerCallbacks) error {
	p := make([]NBTnode, 0, 32)
	for i := 0; i < len(data); {
		if len(p) > 0 && p[len(p)-1].T == nbt.TagList {
			if p[len(p)-1].toRead == 0 {
				if len(p) == 1 {
					return nil
				}
				p = p[:len(p)-1]
				continue
			}
			p[len(p)-1].toRead--
		}
		t := data[i]
		i += 1
		if t == nbt.TagEnd {
			if len(p) == 0 {
				return ConextedError{
					E:            ErrUnexpectedEndTag,
					ReadingStage: "end at the root",
					Offset:       i,
				}
			}
			if p[len(p)-1].T != nbt.TagCompound {
				return ConextedError{
					E:            ErrUnexpectedEndTag,
					ReadingStage: "TagEnd that has no compound",
					Offset:       i,
				}
			}
			if cb.CbEnd != nil {
				cb.CbEnd(p)
			}
			p = p[:len(p)-1]
			if len(p) == 0 {
				return nil
			}
			continue
		}
		ns := int(binary.BigEndian.Uint16(data[i : i+2]))
		if i+2+ns >= len(data) {
			return ConextedError{
				E:            ErrOutOfBounds,
				ReadingStage: fmt.Sprintf("name too big (%d >= %d)", i+2+ns, len(data)),
				Offset:       i,
			}
		}
		i += 2
		n := string(data[i : i+ns])
		i += ns
		switch t {
		default:
			return ErrUnknownTag
		case nbt.TagByte:
			if i >= len(data) {
				return ConextedError{
					E:            ErrOutOfBounds,
					ReadingStage: "payload absent",
					Offset:       i,
				}
			}
			if cb.CbByte != nil {
				cb.CbByte(p, n, data[i])
			}
			i += 1
		case nbt.TagShort:
			if i+2 >= len(data) {
				return ConextedError{
					E:            ErrOutOfBounds,
					ReadingStage: "payload absent",
					Offset:       i,
				}
			}
			if cb.CbShort != nil {
				cb.CbShort(p, n, binary.BigEndian.Uint16(data[i:]))
			}
			i += 2
		case nbt.TagInt:
			if i+4 >= len(data) {
				return ConextedError{
					E:            ErrOutOfBounds,
					ReadingStage: "payload absent",
					Offset:       i,
				}
			}
			if cb.CbInt != nil {
				cb.CbInt(p, n, binary.BigEndian.Uint32(data[i:]))
			}
			i += 4
		case nbt.TagLong:
			if i+8 >= len(data) {
				return ConextedError{
					E:            ErrOutOfBounds,
					ReadingStage: "payload absent",
					Offset:       i,
				}
			}
			if cb.CbLong != nil {
				cb.CbLong(p, n, binary.BigEndian.Uint64(data[i:]))
			}
			i += 8
		case nbt.TagFloat:
			if i+4 >= len(data) {
				return ConextedError{
					E:            ErrOutOfBounds,
					ReadingStage: "payload absent",
					Offset:       i,
				}
			}
			if cb.CbFloat != nil {
				cb.CbFloat(p, n, math.Float32frombits(binary.BigEndian.Uint32(data[i:])))
			}
			i += 4
		case nbt.TagDouble:
			if i+8 >= len(data) {
				return ConextedError{
					E:            ErrOutOfBounds,
					ReadingStage: "payload absent",
					Offset:       i,
				}
			}
			if cb.CbDouble != nil {
				cb.CbDouble(p, n, math.Float64frombits(binary.BigEndian.Uint64(data[i:])))
			}
			i += 8
		case nbt.TagByteArray:
			if i+4 >= len(data) {
				return ConextedError{
					E:            ErrOutOfBounds,
					ReadingStage: "payload length absent",
					Offset:       i,
				}
			}
			s := int(binary.BigEndian.Uint32(data[i:]))
			if i+4+s >= len(data) {
				return ConextedError{
					E:            ErrOutOfBounds,
					ReadingStage: "array size too big",
					Offset:       i,
				}
			}
			cb.CbByteArray(p, n, data[i+4:i+4+s])
			i += 4 + s
		case nbt.TagString:
			if i+2 >= len(data) {
				return ConextedError{
					E:            ErrOutOfBounds,
					ReadingStage: "sting size absent",
					Offset:       i,
				}
			}
			s := int(binary.BigEndian.Uint16(data[i:]))
			if i+2+s >= len(data) {
				return ConextedError{
					E:            ErrOutOfBounds,
					ReadingStage: "string size too big",
					Offset:       i,
				}
			}
			cb.CbString(p, n, string(data[i+2:i+2+s]))
			i += 2 + s
		case nbt.TagList:
			if i+4+1 >= len(data) {
				return ConextedError{
					E:            ErrOutOfBounds,
					ReadingStage: "length absent",
					Offset:       i,
				}
			}
			lt := data[i]
			if lt > 12 {
				return ConextedError{
					E:            ErrUnknownTag,
					ReadingStage: "list type is weird",
					Offset:       i,
				}
			}
			ls := int(binary.BigEndian.Uint32(data[i+1:]))
			i += 4 + 1
			cb.CbList(p, n, lt, ls)
			p = append(p, NBTnode{
				T:      nbt.TagList,
				N:      n,
				S:      ls,
				toRead: ls,
			})
		case nbt.TagCompound:
			cb.CbCompound(p, n)
			p = append(p, NBTnode{
				T: nbt.TagCompound,
				N: n,
			})
		case nbt.TagIntArray:
			if i+4 >= len(data) {
				return ConextedError{
					E:            ErrOutOfBounds,
					ReadingStage: "array length absent",
					Offset:       i,
				}
			}
			s := int32(binary.BigEndian.Uint32(data[i:]))
			if s < 0 {
				return ErrNegativeSize
			}
			i += 4
			if i+int(s)*4 >= len(data) {
				return ConextedError{
					E:            ErrOutOfBounds,
					ReadingStage: "array too big",
					Offset:       i,
				}
			}
			// does not work because endianness
			// cb.CbIntArray(p, n, unsafe.Slice((*uint32)(unsafe.Pointer(&data[i+4])), s))
			arr := make([]uint32, s)
			for ii := 0; ii < int(s); ii++ {
				arr[ii] = binary.BigEndian.Uint32(data[i+ii*4:])
			}
			cb.CbIntArray(p, n, arr)
			i += int(s) * 4
		case nbt.TagLongArray:
			if i+4 >= len(data) {
				return ConextedError{
					E:            ErrOutOfBounds,
					ReadingStage: "array length absent",
					Offset:       i,
				}
			}
			s := int32(binary.BigEndian.Uint32(data[i:]))
			if s < 0 {
				return ErrNegativeSize
			}
			i += 4
			if i+int(s)*8 >= len(data) {
				return ConextedError{
					E:            ErrOutOfBounds,
					ReadingStage: "array too big",
					Offset:       i,
				}
			}
			arr := make([]uint64, s)
			for ii := 0; ii < int(s); ii++ {
				arr[ii] = binary.BigEndian.Uint64(data[i+ii*8:])
			}
			cb.CbLongArray(p, n, arr)
			i += int(s) * 8
		}
	}
	return nil
}
