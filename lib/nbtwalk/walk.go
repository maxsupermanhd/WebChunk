package nbtwalk

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"unsafe"

	"github.com/maxsupermanhd/go-vmc/v764/nbt"
)

var (
	ErrUnknownTag   = errors.New("unknown tag")
	ErrNegativeSize = errors.New("negative size")
)

type NBTnode struct {
	T byte
	N string
	S int
}

type NBTidentifier struct {
	N string
	I int
}

type WalkerCallbacks struct {
	CbEnd       func(p []NBTnode)
	CbByte      func(p []NBTnode, n *NBTidentifier, val byte)
	CbShort     func(p []NBTnode, n *NBTidentifier, val uint16)
	CbInt       func(p []NBTnode, n *NBTidentifier, val uint32)
	CbLong      func(p []NBTnode, n *NBTidentifier, val uint64)
	CbFloat     func(p []NBTnode, n *NBTidentifier, val float32)
	CbDouble    func(p []NBTnode, n *NBTidentifier, val float64)
	CbByteArray func(p []NBTnode, n *NBTidentifier, val []byte)
	CbString    func(p []NBTnode, n *NBTidentifier, val string)
	CbList      func(p []NBTnode, n *NBTidentifier, t byte, l int)
	CbCompound  func(p []NBTnode, n *NBTidentifier)
	CbIntArray  func(p []NBTnode, n *NBTidentifier, val []uint32)
	CbLongArray func(p []NBTnode, n *NBTidentifier, val []uint64)
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
		t := data[i]
		i += 1
		if t == nbt.TagEnd {
			if len(p) == 0 {
				return io.ErrUnexpectedEOF
			}
			if cb.CbEnd != nil {
				cb.CbEnd(p)
			}
			p = p[:len(p)-1]
			continue
		}
		ns := int(binary.BigEndian.Uint16(data[i : i+2]))
		i += 2
		n := &NBTidentifier{N: string(data[i : i+ns])}
		switch t {
		default:
			return ErrUnknownTag
		case nbt.TagByte:
			if i >= len(data) {
				return io.ErrUnexpectedEOF
			}
			if cb.CbByte != nil {
				cb.CbByte(p, n, data[i])
			}
			i += 1
		case nbt.TagShort:
			if i+2 >= len(data) {
				return io.ErrUnexpectedEOF
			}
			if cb.CbShort != nil {
				cb.CbShort(p, n, binary.BigEndian.Uint16(data[i:i+2]))
			}
			i += 2
		case nbt.TagInt:
			if i+4 >= len(data) {
				return io.ErrUnexpectedEOF
			}
			if cb.CbInt != nil {
				cb.CbInt(p, n, binary.BigEndian.Uint32(data[i:i+4]))
			}
			i += 4
		case nbt.TagLong:
			if i+8 >= len(data) {
				return io.ErrUnexpectedEOF
			}
			if cb.CbLong != nil {
				cb.CbLong(p, n, binary.BigEndian.Uint64(data[i:i+8]))
			}
			i += 8
		case nbt.TagFloat:
			if i+4 >= len(data) {
				return io.ErrUnexpectedEOF
			}
			if cb.CbFloat != nil {
				cb.CbFloat(p, n, math.Float32frombits(binary.BigEndian.Uint32(data[i:i+4])))
			}
			i += 4
		case nbt.TagDouble:
			if i+8 >= len(data) {
				return io.ErrUnexpectedEOF
			}
			if cb.CbDouble != nil {
				cb.CbDouble(p, n, math.Float64frombits(binary.BigEndian.Uint64(data[i:i+8])))
			}
			i += 8
		case nbt.TagByteArray:
			if i+4 >= len(data) {
				return io.ErrUnexpectedEOF
			}
			s := int(binary.BigEndian.Uint32(data[i : i+4]))
			if i+4+s >= len(data) {
				return io.ErrUnexpectedEOF
			}
			cb.CbByteArray(p, n, data[i+4:i+s])
			i += 4 + s
		case nbt.TagString:
			if i+2 >= len(data) {
				return io.ErrUnexpectedEOF
			}
			s := int(binary.BigEndian.Uint16(data[i : i+2]))
			if i+2+s >= len(data) {
				return io.ErrUnexpectedEOF
			}
			cb.CbString(p, n, string(data[i+2:i+2+s]))
			i += 2 + s
		case nbt.TagList:
			if i+4+1 >= len(data) {
				return io.ErrUnexpectedEOF
			}
			lt := data[i]
			if lt > 12 {
				return ErrUnknownTag
			}
			ls := int(binary.BigEndian.Uint32(data[i+1 : i+1+4]))
			i += 4 + 1
			cb.CbList(p, n, lt, ls)
			p = append(p, NBTnode{
				T: nbt.TagList,
				N: n.N,
				S: ls,
			})
		case nbt.TagCompound:
			cb.CbCompound(p, n)
			p = append(p, NBTnode{
				T: nbt.TagList,
				N: n.N,
			})
		case nbt.TagIntArray:
			if i+4 >= len(data) {
				return io.ErrUnexpectedEOF
			}
			s := int32(binary.BigEndian.Uint32(data[i : i+4]))
			if s < 0 {
				return ErrNegativeSize
			}
			if i+4+int(s)*4 >= len(data) {
				return io.ErrUnexpectedEOF
			}
			// most cursed thing I've done yet
			cb.CbIntArray(p, n, unsafe.Slice((*uint32)(unsafe.Pointer(&data[i+4])), s))
		case nbt.TagLongArray:
			if i+4 >= len(data) {
				return io.ErrUnexpectedEOF
			}
			s := int32(binary.BigEndian.Uint32(data[i : i+4]))
			if s < 0 {
				return ErrNegativeSize
			}
			if i+4+int(s)*8 >= len(data) {
				return io.ErrUnexpectedEOF
			}
			cb.CbLongArray(p, n, unsafe.Slice((*uint64)(unsafe.Pointer(&data[i+4])), s))
		}
	}
	return nil
}
