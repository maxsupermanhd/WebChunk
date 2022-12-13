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

package proxy

import (
	"fmt"
	"log"
	"strings"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	"github.com/Tnze/go-mc/level"
	"github.com/Tnze/go-mc/level/block"
	"github.com/Tnze/go-mc/nbt"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/server"
	"github.com/davecgh/go-spew/spew"
)

// 4 bits for x, 4 bits for z
// 1 byte: xxxxzzzz
func compactBlockEntityPos(x, z int) int8 {
	val := int8((uint8(uint8(x)&15) << 4) | uint8(uint8(z)&15))
	// log.Printf("  compact %x %x -> %x", x, z, val)
	return val
}
func uncompactBlockEntityPos(xz int8) (x, z int) {
	x, z = int(uint8(xz)>>4), int(uint8(xz)&15)
	// log.Printf("uncompact %x -> %x %x", xz, x, z)
	return x, z
}
func uncompactBlockEntityPosPk(xz int8, y int16) pk.Position {
	x, z := uncompactBlockEntityPos(xz)
	return pk.Position{
		X: int(x),
		Y: int(y),
		Z: int(z),
	}
}

type loadedDim struct {
	id          int32
	minY        int32
	height      int32
	totalHeight int32
}

func (sp SnifferProxy) packetAcceptor(recv chan pk.Packet, conn server.PacketQueue, cl clientinfo) {
	type cachePos struct {
		pos level.ChunkPos
		dim string
	}
	type cacheChunk struct {
		chunk  level.Chunk
		tofind map[pk.Position]int32
	}
	c := map[cachePos]cacheChunk{}
	loadedDims := map[string]loadedDim{}
	currentDim := ""
	for p := range recv {
		switch {
		case p.ID == int32(packetid.ClientboundLevelChunkWithLight):
			if currentDim == "" {
				log.Println("Recieved chunk without dimension")
				continue
			}
			dim, ok := loadedDims[currentDim]
			if !ok {
				log.Printf("Got chunk for not loaded dimension?! (%s)", currentDim)
				continue
			}
			cpos, cc, err := deserializeChunkPacket(p, dim)
			if err != nil {
				log.Printf("Failed to parse chunk data packet: %s", err.Error())
				continue
			}
			// verify all block entities are present
			missingbe := map[pk.Position]int32{}
			for _, sect := range cc.Sections {
				if sect.BlockCount == 0 {
					continue
				}
				// another way around this ugly loop will be to check palette but oh well...
				for y := 0; y < 16; y++ {
					for x := 0; x < 16; x++ {
						for z := 0; z < 16; z++ {
							blo := sect.GetBlock(y*16*16 + z*16 + x)
							bepos := pk.Position{
								X: x,
								Y: y,
								Z: z,
							}
							sta := block.StateList[blo]
							bid, ok := blockEntityTypes[strings.TrimPrefix(sta.ID(), "minecraft:")]
							if ok {
								log.Printf("Found block entity %s: %v", sta.ID(), bepos)
								missingbe[bepos] = bid
							}
						}
					}
				}
			}
			for _, be := range cc.BlockEntity {
				bepos := uncompactBlockEntityPosPk(be.XZ, be.Y)
				beid, ok := missingbe[bepos]
				if !ok {
					// log.Printf("Failed to find block entity: %v (from %d)", uncompactBlockEntityPosPk(be.XZ, be.Y), be.XZ)
					// servers can actually not send block entity as "block" and it will still work for some reason...
					// I would like to include it in the chunk for ease of drawing/operating with them but I am too lazy
					// TODO: maybe add entity to the chunk
					// type blockEntityDefault struct {
					// 	ID string `nbt:"id"`
					// 	X int32 `nbt:"x"`
					// 	Y int32 `nbt:"y"`
					// 	Z int32 `nbt:"z"`
					// }
					// var dubbe blockEntityDefault
					// err := be.Data.Unmarshal(&dubbe)
					// if err != nil {
					// 	log.Println("Failed to unmarshal nbt data of entity: "+err.Error())
					// 	continue
					// }
					// bl, ok := block.FromID[dubbe.ID]
					// if !ok {
					// 	log.Printf("Failed to find block entity blockstate")
					// 	continue
					// }
					// sectionIndex := (loadedDims[currentDim].minY + dubbe.Y)%16
					// if sectionIndex < 0 || len(cc.Sections) < int(sectionIndex)
					// cc.Sections[sectionIndex]
					continue
				}
				if beid == be.Type {
					delete(missingbe, bepos)
				} else {
					log.Printf("Found different block entity type on same spot %v (expected %d found %d)", bepos, be.Type, beid)
				}
			}
			if len(missingbe) > 0 { // what if there is more than in sections?
				// send to cache for completion
				c[cachePos{
					pos: cpos,
					dim: currentDim,
				}] = cacheChunk{
					chunk:  cc,
					tofind: missingbe,
				}
				log.Printf("Caching chunk %d:%d until missing %d block entities recieved or chunk unloaded", cpos[0], cpos[1], len(missingbe))
			} else {
				// send directly to storage because ready
				sp.SaveChannel <- &ProxiedChunk{
					Username:            cl.name,
					Server:              cl.dest,
					Dimension:           currentDim,
					Pos:                 cpos,
					Data:                cc,
					DimensionLowestY:    dim.minY,
					DimensionBuildLimit: int(dim.height),
				}
			}
		case p.ID == int32(packetid.ClientboundBlockEntityData):
			dim, ok := loadedDims[currentDim]
			if !ok {
				log.Printf("Recieved block entity data without dimension?!")
				continue
			}
			var (
				loc  pk.Position
				t    pk.VarInt
				data nbt.RawMessage
			)
			err := p.Scan(&loc, &t, pk.NBT(&data))
			if err != nil {
				log.Printf("Failed to parse block entity data packet: %s", err.Error())
				continue
			}
			if data.Type == 0x0 {
				continue // block entity removed
			}
			cpos := level.ChunkPos{int32(loc.X / 16), int32(loc.Z / 16)}
			cachedLevel, ok := c[cachePos{
				pos: cpos, dim: currentDim,
			}]
			if !ok {
				log.Printf("No cached chunk for block entity %v", loc)
				continue
			}
			beid, ok := cachedLevel.tofind[loc]
			if !ok {
				continue // already exists, ignore for now
				// TODO: update block entity data
			}
			if beid != int32(t) {
				continue // wrong block entity type, must be replaced by someone else before we got it
				// TODO: have a config to change between latest and first data
			}
			delete(cachedLevel.tofind, loc)
			cachedLevel.chunk.BlockEntity = append(cachedLevel.chunk.BlockEntity, level.BlockEntity{
				XZ:   compactBlockEntityPos(loc.X, loc.Z), // int8(((loc.X & 15) << 4) | (loc.Z & 15)),
				Y:    int16(loc.Y),
				Type: int32(t),
				Data: data,
			})
			log.Printf("Recieved block entity %d at %v", t, loc)
			if len(cachedLevel.tofind) == 0 {
				log.Printf("Sending chunk %d:%d to storage because recieved all block entities", cpos[0], cpos[1])
				sp.SaveChannel <- &ProxiedChunk{
					Username:            cl.name,
					Server:              cl.dest,
					Dimension:           currentDim,
					Pos:                 cpos,
					Data:                cachedLevel.chunk,
					DimensionLowestY:    dim.minY,
					DimensionBuildLimit: int(dim.height),
				}
			}
		case p.ID == int32(packetid.ClientboundForgetLevelChunk):
			dim, ok := loadedDims[currentDim]
			if !ok {
				log.Printf("Recieved block entity data without dimension?!")
				continue
			}
			var x, z pk.Int
			err := p.Scan(&x, &z)
			if err != nil {
				log.Printf("Failed to parse unload chunk packet: %s", err.Error())
				continue
			}
			cpos := level.ChunkPos{int32(x), int32(z)}
			cachedLevel, ok := c[cachePos{
				pos: cpos,
				dim: currentDim,
			}]
			if !ok {
				continue
			}
			log.Printf("Server tolad to unload chunk %d:%d, sending chunk as it is to storage", x, z)
			sp.SaveChannel <- &ProxiedChunk{
				Username:            cl.name,
				Server:              cl.dest,
				Dimension:           currentDim,
				Pos:                 cpos,
				Data:                cachedLevel.chunk,
				DimensionLowestY:    dim.minY,
				DimensionBuildLimit: int(dim.height),
			}
		case p.ID == int32(packetid.ClientboundRespawn):
			var (
				dim        pk.Identifier
				dimName    pk.Identifier
				hashedSeed pk.Long
			)
			err := p.Scan(&dim, &dimName, &hashedSeed)
			if err != nil {
				log.Printf("Failed to scan respawn packet: %s", err.Error())
				continue
			}
			log.Printf("respawn to %s (%s)", dimName, dim)
			currentDim = string(dimName)
		case p.ID == int32(packetid.ClientboundLogin):
			var (
				eid              pk.Int
				isHardcore       pk.Boolean
				gamemode         pk.UnsignedByte
				previousGamemode pk.Byte
				dimNames         []pk.Identifier
				dimCodec         nbt.RawMessage
				dim              pk.Identifier
				dimName          pk.Identifier
				hashedSeed       pk.Long
				maxPlayers       pk.VarInt
				viewDistance     pk.VarInt
				simDistance      pk.VarInt
				reducedDebug     pk.Boolean
				respawnScreen    pk.Boolean
				isdebug          pk.Boolean
				isflat           pk.Boolean
				hasDeathLoc      pk.Boolean
				deathDimName     pk.Identifier
				deathLoc         pk.Position
			)
			err := p.Scan(
				&eid,
				&isHardcore,
				&gamemode,
				&previousGamemode,
				pk.Ary[pk.VarInt]{Ary: &dimNames},
				pk.NBT(&dimCodec),
				&dim,
				&dimName,
				&hashedSeed,
				&maxPlayers,
				&viewDistance,
				&simDistance,
				&reducedDebug,
				&respawnScreen,
				&isdebug,
				&isflat,
				&hasDeathLoc,
				&deathDimName,
				&deathLoc,
			)
			if err != nil {
				log.Printf("Failed to parse sniffed packet: %v", err.Error())
				continue
			}
			currentDim = string(dimName)
			cod := map[string]interface{}{}
			err = dimCodec.Unmarshal(&cod)
			if err != nil {
				log.Printf("Failed to unmarshal dim codec: %v", err.Error())
				continue
			}
			dtypes, ok := cod["minecraft:dimension_type"].(map[string]interface{})
			if !ok {
				log.Println("Failed to get dimensions type registry")
				spew.Dump(cod)
				continue
			}
			dims, ok := dtypes["value"].([]interface{})
			if !ok {
				log.Println("Failed to get dimension registry value")
				spew.Dump(dtypes)
				continue
			}
			for di := range dims {
				dd, ok := dims[di].(map[string]interface{})
				if !ok {
					log.Println("Dimension registry value is not a msi")
					spew.Dump(dims)
					continue
				}
				dimname, ok := dd["name"].(string)
				if !ok {
					log.Println("Dimension registry value name error")
					spew.Dump(dd)
					continue
				}
				dimid, ok := dd["id"].(int32)
				if !ok {
					log.Println("Dimension registry value id error")
					spew.Dump(dd)
					continue
				}
				de, ok := dd["element"].(map[string]interface{})
				if !ok {
					log.Println("Dimension registry value element error")
					spew.Dump(dd)
					continue
				}
				miny, ok := de["min_y"].(int32)
				if !ok {
					log.Println("Dimension registry value miny error")
					spew.Dump(de)
					continue
				}
				height, ok := de["height"].(int32)
				if !ok {
					log.Println("Dimension registry value miny error")
					spew.Dump(de)
					continue
				}
				loadedDims[dimname] = loadedDim{
					id:          dimid,
					minY:        miny,
					height:      height,
					totalHeight: height - miny,
				}
			}
		}
		conn.Push(pk.Marshal(
			packetid.ClientboundSystemChat,
			chat.Text(fmt.Sprintf("Cached chunks: %d", len(c))),
			pk.Byte(0),
		))
	}
}
