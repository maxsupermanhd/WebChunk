package proxy

import (
	"bytes"
	"errors"
	"io"
	"log"

	"github.com/maxsupermanhd/go-vmc/v764/level"
	pk "github.com/maxsupermanhd/go-vmc/v764/net/packet"
)

// and now we need to guess how long is sections array in this packet...

// var cc level.Chunk
// always EOF

// cc := level.Chunk{
// 	Sections: []level.Section{},
// 	HeightMaps: level.HeightMaps{
// 		MotionBlocking: &level.BitStorage{},
// 		WorldSurface:   &level.BitStorage{},
// 	},
// 	BlockEntity: []level.BlockEntity{},
// }
// always EOF

// cc := *level.EmptyChunk(16)
// we must now length, wiki says
// > The number of elements in the array is calculated based on the world's height
// except server sends how many sections he wants instead of actual required section count

// dim, ok := loadedDims[currentDim]
// if !ok {
// 	log.Printf("Recieved chunk data without dimension?!")
// 	continue
// }
// log.Printf("%s: % 4d % 4d % 4d", currentDim, dim.minY, dim.height, dim.totalHeight)
// cc := *level.EmptyChunk(int(dim.totalHeight) / 16)
// seems to be correct to do such calculation but oh well, it does not work this way...

// cclen := 64 // MAGIC: theoretical maximum of world height
// cc := *level.EmptyChunk(cclen)
// err := p.Scan(&cpos, &cc)
// for errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
// 	cclen--
// 	cc = *level.EmptyChunk(cclen)
// 	err = p.Scan(&cpos, &cc)
// }
// this might seem to be a complete rofl and not be like this but I don't have choice
// completely tanks performance with allocations, not a solution...
// log.Printf("Scanned with len of %d", cclen)

func deserializeChunkPacket(p pk.Packet, _ loadedDim) (level.ChunkPos, level.Chunk, error) {
	var (
		heightmaps struct {
			MotionBlocking []uint64 `nbt:"MOTION_BLOCKING"`
			WorldSurface   []uint64 `nbt:"WORLD_SURFACE"`
		}
		sectionsData pk.ByteArray
		cc           level.Chunk
		cpos         level.ChunkPos
	)
	err := p.Scan(&cpos, &pk.Tuple{
		pk.NBT(&heightmaps),
		&sectionsData,
		pk.Array(&cc.BlockEntity),
		&lightData{
			SkyLightMask:   make(pk.BitSet, (16*16*16-1)>>6+1),
			BlockLightMask: make(pk.BitSet, (16*16*16-1)>>6+1),
			SkyLight:       []pk.ByteArray{},
			BlockLight:     []pk.ByteArray{},
		},
	})
	if err != nil {
		return cpos, cc, err
	}
	if len(heightmaps.MotionBlocking) == 37 {
		cc.HeightMaps.MotionBlocking = level.NewBitStorage(9, 16*16, heightmaps.MotionBlocking)
	}
	if len(heightmaps.WorldSurface) == 37 {
		cc.HeightMaps.WorldSurface = level.NewBitStorage(9, 16*16, heightmaps.WorldSurface)
	}
	d := bytes.NewReader(sectionsData)
	dl := int64(len(sectionsData))
	for {
		if dl == 0 {
			break
		}
		if dl < 200 { // whole chunk structure is 207 if completely empty?
			// log.Printf("Leaving %d bytes behind while parsing chunk data!", dl)
			break
		}
		ss := &level.Section{
			BlockCount: 0,
			States:     level.NewStatesPaletteContainer(16*16*16, 0),
			Biomes:     level.NewBiomesPaletteContainer(4*4*4, 0),
		}
		n, err := ss.ReadFrom(d)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			log.Printf("EOF while decoding chunk data while reading section %d", len(cc.Sections))
			break
		}
		if n == 0 {
			log.Printf("Failed to read from buffer? 0 readed")
			continue
		}
		dl -= n
		cc.Sections = append(cc.Sections, *ss)
	}
	// cc.HeightMaps.MotionBlocking = level.NewBitStorage(int(math.Log2(float64(dim.totalHeight+1))), len(heightmaps.MotionBlocking), heightmaps.MotionBlocking)
	return cpos, cc, err
}

type lightData struct {
	SkyLightMask   pk.BitSet
	BlockLightMask pk.BitSet
	SkyLight       []pk.ByteArray
	BlockLight     []pk.ByteArray
}

func bitSetRev(set pk.BitSet) pk.BitSet {
	rev := make(pk.BitSet, len(set))
	for i := range rev {
		rev[i] = ^set[i]
	}
	return rev
}

func (l *lightData) WriteTo(w io.Writer) (int64, error) {
	return pk.Tuple{
		pk.Boolean(true), // Trust Edges
		l.SkyLightMask,
		l.BlockLightMask,
		bitSetRev(l.SkyLightMask),
		bitSetRev(l.BlockLightMask),
		pk.Array(l.SkyLight),
		pk.Array(l.BlockLight),
	}.WriteTo(w)
}

func (l *lightData) ReadFrom(r io.Reader) (int64, error) {
	var TrustEdges pk.Boolean
	var RevSkyLightMask, RevBlockLightMask pk.BitSet
	return pk.Tuple{
		&TrustEdges, // Trust Edges
		&l.SkyLightMask,
		&l.BlockLightMask,
		&RevSkyLightMask,
		&RevBlockLightMask,
		pk.Array(&l.SkyLight),
		pk.Array(&l.BlockLight),
	}.ReadFrom(r)
}

var blockEntityTypes = map[string]int32{
	"furnace":           0,
	"chest":             1,
	"trapped_chest":     2,
	"ender_chest":       3,
	"jukebox":           4,
	"dispenser":         5,
	"dropper":           6,
	"sign":              7,
	"mob_spawner":       8,
	"piston":            9,
	"brewing_stand":     10,
	"enchanting_table":  11,
	"end_portal":        12,
	"beacon":            13,
	"skull":             14,
	"daylight_detector": 15,
	"hopper":            16,
	"comparator":        17,
	"banner":            18,
	"structure_block":   19,
	"end_gateway":       20,
	"command_block":     21,
	"shulker_box":       22,
	"bed":               23,
	"conduit":           24,
	"barrel":            25,
	"smoker":            26,
	"blast_furnace":     27,
	"lectern":           28,
	"bell":              29,
	"jigsaw":            30,
	"campfire":          31,
	"beehive":           32,
}
