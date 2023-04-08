/*
	WebChunk, web server for block game maps
	Copyright (C) 2022 Maxim Zhuchkov

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU Affero General Public License as published
	by the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU Affero General Public License for more details.

	You should have received a copy of the GNU Affero General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.

	Contact me via mail: q3.max.2011@yandex.ru or Discord: MaX#6717
*/

package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Tnze/go-mc/level"
	"github.com/Tnze/go-mc/nbt"
	"github.com/Tnze/go-mc/save"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
	"github.com/maxsupermanhd/WebChunk/chunkStorage/filesystemChunkStorage"
	"github.com/maxsupermanhd/WebChunk/chunkStorage/postgresChunkStorage"
	"github.com/maxsupermanhd/WebChunk/proxy"
	"github.com/maxsupermanhd/lac"
)

var (
	errStorageTypeNotImplemented = errors.New("storage type not implemented")
	storages                     map[string]chunkStorage.Storage
	storagesLock                 sync.Mutex
)

func initStorages() error {
	log.Println("Initializing storages...")
	err := cfg.GetToStruct(&storages, "storages")
	if err != nil && !errors.Is(err, lac.ErrNoKey) {
		return err
	}
	if len(storages) == 0 {
		log.Println("No storages to initialize")
		cfg.Set(map[string]any{}, "storages")
		return nil
	}
	for k, v := range storages {
		d, err := initStorage(storages[k].Type, storages[k].Address)
		if err != nil {
			log.Println("Failed to initialize storage: " + err.Error())
			continue
		}
		ver, err := d.GetStatus()
		if err != nil {
			log.Println("Error getting storage status: " + err.Error())
			continue
		}
		v.Driver = d
		storages[k] = v
		log.Println("Storage initialized: " + ver)
	}
	return nil
}

func initStorage(storageStype, address string) (driver chunkStorage.ChunkStorage, err error) {
	switch storageStype {
	case "postgres":
		driver, err = postgresChunkStorage.NewPostgresChunkStorage(context.Background(), address)
		if err != nil {
			return nil, err
		}
		return driver, nil
	case "filesystem":
		driver, err = filesystemChunkStorage.NewFilesystemChunkStorage(address)
		if err != nil {
			return nil, err
		}
		return driver, nil
	default:
		return nil, errStorageTypeNotImplemented
	}
}

func findCapableStorage(storages map[string]chunkStorage.Storage, pref string) chunkStorage.ChunkStorage {
	p, ok := storages[pref]
	if ok {
		return p.Driver
	}
	for sn, s := range storages {
		if s.Driver == nil || sn == "" {
			continue
		}
		a := s.Driver.GetAbilities()
		if a.CanCreateWorldsDimensions && a.CanAddChunks {
			return s.Driver
		}
	}
	return nil
}

func chunkConsumer(ctx context.Context, c chan *proxy.ProxiedChunk) {
	for {
		select {
		case <-ctx.Done():
			return
		case r := <-c:
			if r.Dimension == "" || r.Server == "" {
				log.Printf("Got chunk [%v](%v) from [%v] by [%v] with empty params, DROPPING", r.Pos, r.Dimension, r.Server, r.Username)
				continue
			}
			log.Printf("Got chunk %v %#v from [%v] by [%v] (%2d s) (%3d be)", r.Pos, r.Dimension, r.Server, r.Username, len(r.Data.Sections), len(r.Data.BlockEntity))
			r.Dimension = strings.TrimPrefix(r.Dimension, "minecraft:")
			w, s, err := chunkStorage.GetWorldStorage(storages, r.Server)
			if err != nil {
				log.Println("Failed to lookup world storage: ", err)
				break
			}
			var d *chunkStorage.SDim
			if w == nil || s == nil {
				pref := cfg.GetDSString("", "preferred_storage")
				s = findCapableStorage(storages, pref)
				if s == nil {
					log.Printf("Failed to find storage that has world [%s], named [%s] or has ability to add chunks, chunk [%v] from [%v] by [%v] is LOST.", r.Server, pref, r.Pos, r.Server, r.Username)
					continue
				}
				w = &chunkStorage.SWorld{
					Name:       r.Server,
					Alias:      "",
					IP:         r.Server,
					CreatedAt:  time.Now(),
					ModifiedAt: time.Now(),
					Data:       chunkStorage.CreateDefaultLevelData(r.Server),
				}
				err = s.AddWorld(*w)
				if err != nil {
					log.Printf("Failed to add world: %s", err.Error())
					continue
				}
			}
			d, err = s.GetDimension(w.Name, r.Dimension)
			if err != nil && !errors.Is(err, chunkStorage.ErrNoDim) {
				log.Printf("Failed to get dim: %s", err.Error())
				continue
			}
			if d == nil {
				d = &chunkStorage.SDim{
					Name:       r.Dimension,
					World:      w.Name,
					CreatedAt:  time.Now(),
					ModifiedAt: time.Now(),
					Data:       chunkStorage.GuessDimTypeFromName(r.Dimension),
				}
				err = s.AddDimension(w.Name, *d)
				if err != nil {
					log.Printf("Failed to add dim: %s", err.Error())
					continue
				}
			}
			if d == nil {
				log.Println("d is nill")
				continue
			}
			if w == nil {
				log.Println("w is nill")
				continue
			}
			if d.World != w.Name {
				log.Printf("SUS dim's wname != world's name [%s] [%s]", d.World, w.Name)
				continue
			}
			nbtEmptyList := nbt.RawMessage{
				Type: nbt.TagList,
				Data: []byte{
					nbt.TagEnd, // type
					0, 0, 0, 0, // length (4 bytes)
				},
			}
			nbtEmptyCompound := nbt.RawMessage{
				Type: nbt.TagCompound,
				Data: []byte{0}, // tag end
			}
			var data save.Chunk
			data = save.Chunk{
				DataVersion:   3218,
				XPos:          r.Pos[0],
				YPos:          r.DimensionLowestY / 16,
				ZPos:          r.Pos[1],
				BlockEntities: []nbt.RawMessage{},
				Structures:    nbtEmptyCompound,
				Heightmaps: struct {
					MotionBlocking         []uint64 "nbt:\"MOTION_BLOCKING\""
					MotionBlockingNoLeaves []uint64 "nbt:\"MOTION_BLOCKING_NO_LEAVES\""
					OceanFloor             []uint64 "nbt:\"OCEAN_FLOOR\""
					WorldSurface           []uint64 "nbt:\"WORLD_SURFACE\""
				}{
					MotionBlocking:         []uint64{},
					MotionBlockingNoLeaves: []uint64{},
					OceanFloor:             []uint64{},
					WorldSurface:           []uint64{},
					// MotionBlockingNoLeaves: r.Data.HeightMaps.MotionBlockingNoLeaves.Raw(),
					// OceanFloor:             r.Data.HeightMaps.OceanFloor.Raw(),
					// WorldSurface:           r.Data.HeightMaps.WorldSurface.Raw(),
				},
				Sections:       []save.Section{},
				BlockTicks:     nbtEmptyList,
				FluidTicks:     nbtEmptyList,
				PostProcessing: nbtEmptyList,
				InhabitedTime:  0,
				IsLightOn:      0,
				LastUpdate:     time.Now().Unix(),
				Status:         "proxied",
			}
			if r.DimensionLowestY%16 > 0 {
				data.YPos++
			}
			level.ChunkToSave(&r.Data, &data)

			var chunkBytes bytes.Buffer
			chunkBytes.WriteByte(1) // compression type
			chunkBytesWriter := gzip.NewWriter(&chunkBytes)
			err = nbt.NewEncoder(chunkBytesWriter).Encode(data, "")
			if err != nil {
				log.Printf("Failed to marshal chunk: %s", err.Error())
				continue
			}
			err = chunkBytesWriter.Close()
			if err != nil {
				log.Printf("Failed to flush chunk buffer: %s", err.Error())
				continue
			}
			err = s.AddChunkRaw(w.Name, d.Name, int(r.Pos[0]), int(r.Pos[1]), chunkBytes.Bytes())
			if err != nil {
				log.Printf("Failed to save chunk: %s", err.Error())
			}
			if cfg.GetDSBool(true, "render_received") {
				go func() {
					i := drawChunk(&data)
					imageCacheSave(i, w.Name, d.Name, "terrain", 0, int(r.Pos[0]), int(r.Pos[1]))
				}()
			}
		}
	}
}

// func chunkLevelToSave(in *level.Chunk, lowestY int32, cx, cz int32) (*save.Chunk, error) {
// 	spew.Dump(in)
// 	out := save.Chunk{
// 		DataVersion:   2865, // was at the moment of writing
// 		XPos:          int32(cx),
// 		YPos:          lowestY / 16,
// 		ZPos:          int32(cz),
// 		BlockEntities: nbt.RawMessage{},
// 		Structures:    nbt.RawMessage{}, // we will never get those
// 		Heightmaps: struct {
// 			MotionBlocking         []int64 "nbt:\"MOTION_BLOCKING\""
// 			MotionBlockingNoLeaves []int64 "nbt:\"MOTION_BLOCKING_NO_LEAVES\""
// 			OceanFloor             []int64 "nbt:\"OCEAN_FLOOR\""
// 			WorldSurface           []int64 "nbt:\"WORLD_SURFACE\""
// 		}{
// 			MotionBlocking:         []int64{}, //*(*[]int64)(unsafe.Pointer(in.HeightMaps.MotionBlocking)),
// 			MotionBlockingNoLeaves: []int64{},
// 			OceanFloor:             []int64{},
// 			WorldSurface:           []int64{}, //*(*[]int64)(unsafe.Pointer(in.HeightMaps.WorldSurface)),
// 		},
// 		Sections: []save.Section{},
// 	}
// 	for y, s := range in.Sections {
// 		o := save.Section{
// 			Y: int8(y + int(out.YPos)),
// 			BlockStates: struct {
// 				Palette []save.BlockState "nbt:\"palette\""
// 				Data    []int64           "nbt:\"data\""
// 			}{
// 				Palette: []save.BlockState{},
// 				Data:    []int64{},
// 			},
// 			Biomes: struct {
// 				Palette []string "nbt:\"palette\""
// 				Data    []int64  "nbt:\"data\""
// 			}{
// 				Palette: []string{},
// 				Data:    []int64{},
// 			},
// 			SkyLight:   []byte{0},
// 			BlockLight: []byte{0},
// 		}

// 		convbuf := bytes.NewBuffer([]byte{})
// 		s.States.WriteTo(convbuf)

// 		// blockstates
// 		// if s.BlockCount == 0 {
// 		// 	o.BlockStates.Palette = append(o.BlockStates.Palette, save.BlockState{
// 		// 		Name: "minecraft:air",
// 		// 		Properties: nbt.RawMessage{
// 		// 			Type: nbt.TagCompound,
// 		// 			Data: []byte{nbt.TagEnd},
// 		// 		},
// 		// 	})
// 		// } else {
// 		// 	statesPalette := []int{}
// 		// 	statesIndexes := [4096]int64{}
// 		// 	for i := 0; i < 16*16*16; i++ {
// 		// 		addState := s.States.Get(i)
// 		// 		foundstate := -1
// 		// 		for ii := range statesPalette {
// 		// 			if statesPalette[ii] == addState {
// 		// 				foundstate = ii
// 		// 				break
// 		// 			}
// 		// 		}
// 		// 		if foundstate == -1 {
// 		// 			statesPalette = append(statesPalette, addState)
// 		// 			foundstate = len(statesPalette) - 1
// 		// 		}
// 		// 		statesIndexes[i] = int64(foundstate)
// 		// 	}
// 		// 	for i := range statesPalette {
// 		// 		b := block.StateList[statesPalette[i]]
// 		// 		addPalette := save.BlockState{
// 		// 			Name: b.ID(),
// 		// 			Properties: nbt.RawMessage{
// 		// 				Type: nbt.TagCompound,
// 		// 				Data: []byte{nbt.TagEnd},
// 		// 			},
// 		// 		}
// 		// 		dat, err := nbt.Marshal(b)
// 		// 		if err != nil {
// 		// 			return nil, err
// 		// 		}
// 		// 		if len(dat) == 4 {
// 		// 			dat = []byte{0x0a, 0x00}
// 		// 		}
// 		// 		addPalette.Properties.Data = dat
// 		// 		o.BlockStates.Palette = append(o.BlockStates.Palette, addPalette)
// 		// 	}
// 		// 	if len(o.BlockStates.Palette) > 1 {
// 		// 		sizeBits := int64(0)
// 		// 		for i := int64(0); i < 32; i++ {
// 		// 			if len(statesPalette)&(1<<i) != 0 {
// 		// 				sizeBits = i + 1
// 		// 			}
// 		// 		}
// 		// 		if sizeBits < 4 {
// 		// 			sizeBits = 4
// 		// 		}
// 		// 		haveBits := int64(64)
// 		// 		currData := int64(0)
// 		// 		for i := range statesIndexes {
// 		// 			if haveBits < sizeBits {
// 		// 				if haveBits == 0 {
// 		// 					o.BlockStates.Data = append(o.BlockStates.Data, int64(currData))
// 		// 					haveBits = 64
// 		// 					currData = 0
// 		// 				} else {
// 		// 					leftBits := sizeBits - haveBits
// 		// 					currData = currData | (statesIndexes[i] >> leftBits)
// 		// 					o.BlockStates.Data = append(o.BlockStates.Data, int64(currData))
// 		// 					haveBits = 64 + haveBits
// 		// 					currData = 0
// 		// 				}
// 		// 			}
// 		// 			currData = currData | (statesIndexes[i])<<(haveBits-sizeBits)
// 		// 			haveBits -= sizeBits
// 		// 		}
// 		// 		if haveBits != 64 {
// 		// 			o.BlockStates.Data = append(o.BlockStates.Data, currData)
// 		// 		}
// 		// 	}
// 		// }
// 		// biomes
// 		// heightmaps
// 		out.Sections = append(out.Sections, o)
// 	}
// 	log.Printf("Saved %d sections", len(out.Sections))
// 	spew.Dump(out)
// 	return &out, nil
// }
