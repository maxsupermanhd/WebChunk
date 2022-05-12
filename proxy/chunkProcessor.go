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
	"bytes"
	"errors"
	"fmt"
	"io"
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
	"github.com/google/uuid"
	"github.com/maxsupermanhd/WebChunk/viewer"
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

func packetAcceptor(recv chan pk.Packet, conn *server.PacketQueue, resp chan *ProxiedChunk, username, serverip string, conf ProxyConfig) {
	type cachePos struct {
		pos level.ChunkPos
		dim string
	}
	type cacheChunk struct {
		chunk  level.Chunk
		tofind map[pk.Position]int32
	}
	c := map[cachePos]cacheChunk{}
	type loadedDim struct {
		id          int32
		minY        int32
		height      int32
		totalHeight int32
	}
	loadedDims := map[string]loadedDim{}
	currentDim := ""
	for p := range recv {
		switch {
		case p.ID == packetid.ClientboundLevelChunkWithLight:
			if currentDim == "" {
				log.Println("Recieved chunk without dimension")
				continue
			}
			cpos, cc, err := deserializeChunkPacket(p)
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
							bid, ok := viewer.BlockEntityTypes[strings.TrimPrefix(sta.ID(), "minecraft:")]
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
				log.Printf("Caching chunk %d:%d until missing %d block entities recieved or chunk unloaded", cpos.X, cpos.Z, len(missingbe))
			} else {
				// send directly to storage because ready
				resp <- &ProxiedChunk{
					Username:  username,
					Server:    serverip,
					Dimension: currentDim,
					Pos:       cpos,
					Data:      cc,
				}
			}
		case p.ID == packetid.ClientboundBlockEntityData:
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
			cpos := level.ChunkPos{X: loc.X / 16, Z: loc.Z / 16}
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
				log.Printf("Sending chunk %d:%d to storage because recieved all block entities", cpos.X, cpos.Z)
				resp <- &ProxiedChunk{
					Username:         username,
					Server:           serverip,
					Dimension:        currentDim,
					Pos:              cpos,
					Data:             cachedLevel.chunk,
					DimensionLowestY: dim.minY,
				}
			}
		case p.ID == packetid.ClientboundForgetLevelChunk:
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
			cpos := level.ChunkPos{X: int(x), Z: int(z)}
			cachedLevel, ok := c[cachePos{
				pos: cpos,
				dim: currentDim,
			}]
			if !ok {
				continue
			}
			log.Printf("Server tolad to unload chunk %d:%d, sending chunk as it is to storage", x, z)
			resp <- &ProxiedChunk{
				Username:         username,
				Server:           serverip,
				Dimension:        currentDim,
				Pos:              cpos,
				Data:             cachedLevel.chunk,
				DimensionLowestY: dim.minY,
			}
		case p.ID == packetid.ClientboundRespawn:
			var (
				dim        nbt.RawMessage
				dimName    pk.Identifier
				hashedSeed pk.Long
			)
			err := p.Scan(pk.NBT(&dim), &dimName, &hashedSeed)
			if err != nil {
				log.Printf("Failed to scan respawn packet: %s", err.Error())
				continue
			}
			log.Printf("respawn to %s (%s)", dimName, dim.String())
			currentDim = string(dimName)
		case p.ID == packetid.ClientboundLogin:
			var (
				eid              pk.Int
				isHardcore       pk.Boolean
				gamemode         pk.UnsignedByte
				previousGamemode pk.Byte
				dimNames         []pk.Identifier
				dimCodec         nbt.RawMessage
				dim              nbt.RawMessage
				dimName          pk.Identifier
				hashedSeed       pk.Long
				maxPlayers       pk.VarInt
				viewDistance     pk.VarInt
				simDistance      pk.VarInt
				reducedDebug     pk.Boolean
				respawnScreen    pk.Boolean
				isdebug          pk.Boolean
				isflat           pk.Boolean
			)
			err := p.Scan(
				&eid,
				&isHardcore,
				&gamemode,
				&previousGamemode,
				pk.Ary[pk.VarInt, *pk.VarInt]{Ary: &dimNames},
				pk.NBT(&dimCodec),
				pk.NBT(&dim),
				&dimName,
				&hashedSeed,
				&maxPlayers,
				&viewDistance,
				&simDistance,
				&reducedDebug,
				&respawnScreen,
				&isdebug,
				&isflat,
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
			packetid.ClientboundChat,
			chat.Text(fmt.Sprintf("Cached chunks: %d", len(c))),
			pk.Byte(2),
			pk.UUID(uuid.Nil),
		))
	}
}

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

func deserializeChunkPacket(p pk.Packet) (level.ChunkPos, level.Chunk, error) {
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
	// cc.HeightMaps.MotionBlocking = heightmaps.MotionBlocking
	d := bytes.NewReader(sectionsData)
	dl := int64(len(sectionsData))
	for {
		if dl == 0 {
			break
		}
		if dl < 200 { // whole chunk structure is 207 if completely empty?
			log.Printf("Leaving %d bytes behind while parsing chunk data!", dl)
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
