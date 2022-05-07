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
	"log"

	"github.com/Tnze/go-mc/level"
	"github.com/Tnze/go-mc/level/block"
	"github.com/Tnze/go-mc/nbt"
	"github.com/Tnze/go-mc/save"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
	"github.com/maxsupermanhd/WebChunk/proxy"
)

func closeStorages(s []chunkStorage.Storage) {
	for _, s2 := range s {
		if s2.Driver != nil {
			s2.Driver.Close()
		}
	}
}

func chunkConsumer(c chan *proxy.ProxiedChunk) {
	for r := range c {
		route, ok := loadedConfig.Routes[r.Username]
		if !ok {
			log.Printf("Got UNKNOWN chunk [%v] from [%v] by [%v]", r.Pos, r.Server, r.Username)
		}
		log.Printf("Got chunk [%v] from [%v] by [%v] (%d sections) (%d block entities)", r.Pos, r.Server, r.Username, len(r.Data.Sections), len(r.Data.BlockEntity))
		if route.World == "" {
			route.World = r.Server
		}
		if route.Dimension == "" {
			route.Dimension = r.Dimension
		}
		w, s, err := chunkStorage.GetWorldStorage(storages, route.World)
		if err != nil {
			log.Println("Failed to lookup world storage: ", err)
			break
		}
		var d *chunkStorage.DimStruct
		if w == nil || s == nil {
			var sf chunkStorage.ChunkStorage
			for i := range storages {
				if storages[i].Name == route.Storage {
					s = storages[i].Driver
					break
				}
				a := storages[i].Driver.GetAbilities()
				if a.CanCreateWorldsDimensions &&
					a.CanAddChunks &&
					a.CanPreserveOldChunks {
					sf = storages[i].Driver
				}
			}
			if s == nil {
				s = sf
			}
			if s == nil {
				log.Printf("Failed to find storage that has world [%s], named [%s] or has ability to add chunks, chunk [%v] from [%v] by [%v] is LOST.", route.World, route.Storage, r.Pos, r.Server, r.Username)
				continue
			}
			w, err = s.AddWorld(route.World, r.Server)
			if err != nil {
				log.Printf("Failed to add world: %s", err.Error())
				continue
			}
			d, err = s.AddDimension(w.Name, route.Dimension, route.Dimension)
			if err != nil {
				log.Printf("Failed to add dim: %s", err.Error())
				continue
			}
		} else {
			d, err = s.GetDimension(w.Name, route.Dimension)
			if err != nil {
				log.Printf("Failed to get dim: %s", err.Error())
				continue
			}
		}
		if d.World != w.Name {
			log.Printf("SUS dim's wname != world's name [%s] [%s]", d.World, w.Name)
			continue
		}
		data, err := chunkLevelToSave(&r.Data, r.DimensionLowestY, int32(r.Pos.X), int32(r.Pos.Z))
		if err != nil {
			log.Printf("Error convering level to save: %s", err.Error())
		}
		err = s.AddChunk(w.Name, d.Name, r.Pos.X, r.Pos.Z, *data)
		if err != nil {
			log.Printf("Failed to save chunk: %s", err.Error())
		}
	}
}

func chunkLevelToSave(in *level.Chunk, lowestY int32, cx, cz int32) (*save.Chunk, error) {
	out := save.Chunk{
		DataVersion:   2865, // was at the moment of writing
		XPos:          int32(cx),
		YPos:          lowestY / 16,
		ZPos:          int32(cz),
		BlockEntities: nbt.RawMessage{},
		Structures:    nbt.RawMessage{}, // we will never get those
		Heightmaps: struct {
			MotionBlocking         []int64 "nbt:\"MOTION_BLOCKING\""
			MotionBlockingNoLeaves []int64 "nbt:\"MOTION_BLOCKING_NO_LEAVES\""
			OceanFloor             []int64 "nbt:\"OCEAN_FLOOR\""
			WorldSurface           []int64 "nbt:\"WORLD_SURFACE\""
		}{},
		Sections: []save.Section{},
	}
	for y, s := range in.Sections {
		o := save.Section{
			Y: int8(y + int(out.YPos)),
			BlockStates: struct {
				Palette []save.BlockState "nbt:\"palette\""
				Data    []int64           "nbt:\"data\""
			}{},
			Biomes: struct {
				Palette []string "nbt:\"palette\""
				Data    []int64  "nbt:\"data\""
			}{},
			SkyLight:   []byte{},
			BlockLight: []byte{},
		}

		// blockstates
		if s.BlockCount == 0 {
			o.BlockStates.Palette = append(o.BlockStates.Palette, save.BlockState{
				Name:       "minecraft:air",
				Properties: nbt.RawMessage{},
			})
		} else {
			statesPalette := []int{}
			statesIndexes := [4096]int64{}
			for i := 0; i < 16*16*16; i++ {
				addState := s.States.Get(i)
				foundstate := -1
				for ii := range statesPalette {
					if statesPalette[ii] == addState {
						foundstate = ii
						break
					}
				}
				if foundstate == -1 {
					statesPalette = append(statesPalette, addState)
					foundstate = len(statesPalette) - 1
				}
				statesIndexes[i] = int64(foundstate)
			}
			for i := range statesPalette {
				b := block.StateList[statesPalette[i]]
				addPalette := save.BlockState{
					Name:       b.ID(),
					Properties: nbt.RawMessage{},
				}
				dat, err := nbt.Marshal(b)
				if err != nil {
					return nil, err
				}
				addPalette.Properties.Data = dat
				addPalette.Properties.Type = nbt.TagCompound
				o.BlockStates.Palette = append(o.BlockStates.Palette, addPalette)
			}
			sizeBits := int64(0)
			for i := int64(0); i < 32; i++ {
				if len(statesPalette)&(1<<i) != 0 {
					sizeBits = i + 1
				}
			}
			if sizeBits < 4 {
				sizeBits = 4
			}
			haveBits := int64(64)
			currData := int64(0)
			for i := range statesIndexes {
				if haveBits < sizeBits {
					if haveBits == 0 {
						o.BlockStates.Data = append(o.BlockStates.Data, int64(currData))
						haveBits = 64
						currData = 0
					} else {
						leftBits := sizeBits - haveBits
						currData = currData | (statesIndexes[i] >> leftBits)
						o.BlockStates.Data = append(o.BlockStates.Data, int64(currData))
						haveBits = 64 + haveBits
						currData = 0
					}
				}
				currData = currData | (statesIndexes[i])<<(haveBits-sizeBits)
				haveBits -= sizeBits
			}
			if haveBits != 64 {
				o.BlockStates.Data = append(o.BlockStates.Data, currData)
			}
		}
		// biomes
		// heightmaps
		out.Sections = append(out.Sections, o)
	}
	return &out, nil
}
