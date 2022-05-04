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
		found := false
		for _, c := range loadedConfig.Routes {
			if c.Username != r.Username {
				continue
			}
			found = true
			if c.World == "" || c.Dimension == "" {
				log.Printf("Got chunk [%v] from [%v] by [%v]", r.Pos, r.Server, r.Username)
			} else {
				_, _, err := chunkStorage.GetWorldStorage(storages, c.World)
				if err != nil {
					log.Println("Failed to lookup world storage: ", err)
					// } else {
					// s.AddChunk(c.World, c.Dimension, r.Pos.X, r.Pos.Z, save.Chunk{
					// 	DataVersion:   r.Data.,
					// 	XPos:          int32(r.Pos.X),
					// 	YPos:          -4,
					// 	ZPos:          int32(r.Pos.Z),
					// 	BlockEntities: ,
					// 	Structures:    nbt.RawMessage{},
					// 	Heightmaps:    struct{MotionBlocking []int64 "nbt:\"MOTION_BLOCKING\""; MotionBlockingNoLeaves []int64 "nbt:\"MOTION_BLOCKING_NO_LEAVES\""; OceanFloor []int64 "nbt:\"OCEAN_FLOOR\""; WorldSurface []int64 "nbt:\"WORLD_SURFACE\""}{},
					// 	Sections:      []save.Section{},
					// })
				}
			}
		}
		if !found {
			log.Printf("Got UNKNOWN chunk [%v] from [%v] by [%v]", r.Pos, r.Server, r.Username)
		}
	}
}
