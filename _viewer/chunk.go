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

package viewer

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/bits"
	"strings"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
	"github.com/maxsupermanhd/go-vmc/v764/chat"
	"github.com/maxsupermanhd/go-vmc/v764/data/packetid"
	"github.com/maxsupermanhd/go-vmc/v764/level"
	"github.com/maxsupermanhd/go-vmc/v764/level/block"
	"github.com/maxsupermanhd/go-vmc/v764/nbt"
	pk "github.com/maxsupermanhd/go-vmc/v764/net/packet"
	"github.com/maxsupermanhd/go-vmc/v764/save"
	"github.com/maxsupermanhd/go-vmc/v764/server"
)

type playerData struct {
	x, y, z       float64
	player        *server.Player
	viewingChunks map[level.ChunkPos]bool
	viewDistance  int
	locWorld      string
	locDimension  string
}

type chunkLoader struct {
	g                   *server.Game
	s                   []chunkStorage.Storage
	defaultViewDistance int
	players             map[uuid.UUID]*playerData
	playersMutex        sync.Mutex
}

func NewChunkLoader(s []chunkStorage.Storage, viewDistance int) *chunkLoader {
	return &chunkLoader{
		s:                   s,
		defaultViewDistance: viewDistance,
	}
}
func (s *chunkLoader) Init(g *server.Game) {
	s.g = g
	s.players = map[uuid.UUID]*playerData{}
	g.AddHandler(&server.PacketHandler{
		ID: packetid.ServerboundMovePlayerPos,
		F: func(client *server.Client, player *server.Player, packet server.Packet758) error {
			var x, y, z pk.Double
			var ground pk.Boolean
			if err := pk.Packet(packet).Scan(&x, &y, &z, &ground); err != nil {
				return err
			}
			p, ok := s.players[player.UUID]
			if !ok {
				log.Printf("Player %s not found in players map, position packet ignored.", player.UUID)
			}
			if p.x != float64(x) || p.y != float64(y) || p.z != float64(z) {
				SendUpdateViewPosition(player, int32(float64(x)/16), int32(float64(z)/16))
			}
			s.playersMutex.Lock()
			p.x = float64(x)
			p.y = float64(y)
			p.z = float64(z)
			s.playersMutex.Unlock()
			log.Printf("Player [%s] updated positon [%.1f %.1f %.1f]", player.Name, x, y, z)
			return nil
		},
	})
}
func (s *chunkLoader) AddPlayer(p *server.Player) {
	log.Printf("Player [%v] (%v) joined", p.Name, p.UUID)
	s.playersMutex.Lock()
	sendpos := func() {
		SendPlayerPositionAndLook(p, 0, 64, 0, 0, 0, 0, 69, false)
		SendUpdateViewPosition(p, 0, 0)
	}
	sendpos()
	SendPlayerAbilities(p, true, true, true, true, 0.1, 0.1)
	s.players[p.UUID] = &playerData{
		player:        p,
		x:             0,
		y:             64,
		z:             0,
		viewingChunks: map[level.ChunkPos]bool{},
		viewDistance:  s.defaultViewDistance,
	}
	s.playersMutex.Unlock()
}
func (s *chunkLoader) RemovePlayer(p *server.Player) {
	log.Printf("Player [%v] left", p.Name)
	s.playersMutex.Lock()
	delete(s.players, p.UUID)
	s.playersMutex.Unlock()
}
func (s *chunkLoader) sendChunk(pos level.ChunkPos, p *playerData) {
	var chunk *level.Chunk
	if p.locWorld == "" {
		chunk = s.generateHubChunk(pos.X, pos.Z)
	} else {
		_, storage, err := chunkStorage.GetWorldStorage(s.s, p.locWorld)
		if err != nil {
			log.Println("Failed to get world storage: " + err.Error())
		}
		dim, err := storage.GetDimension(p.locWorld, p.locDimension)
		if err != nil {
			log.Printf("Failed to get dimension [%s] [%s] info: %s", p.locWorld, p.locDimension, err.Error())
			return
		}
		save, err := storage.GetChunk(p.locWorld, p.locDimension, pos.X, pos.Z)
		if err != nil {
			log.Printf("Failed to get chunk %v: %v", pos, err.Error())
			return
		}
		if save == nil {
			log.Println("Chunk not found")
			chunk = level.EmptyChunk(256)
		} else {
			// chunk = level.ChunkFromSave(save)
			chunk = ActualChunkFromSave(save, int(math.Abs(float64(dim.BuildLimit))+math.Abs(float64(dim.LowestY))))
		}
	}
	if chunk == nil {
		log.Printf("Failed to get chunk %v, nil conversion", pos)
		return
	}
	p.player.WritePacket(server.Packet758(pk.Marshal(
		packetid.ClientboundLevelChunkWithLight,
		pos, chunk,
	)))
	// for i := range chunk.Sections {
	// 	for y := 0; y < 16; y++ {
	// 		for x := 0; x < 16; x++ {
	// 			for z := 0; z < 16; z++ {
	// 				b := chunk.Sections[i].GetBlock(y*16*16 + z*16 + x)
	// 				n := strings.TrimPrefix(block.StateList[b].ID(), "minecraft:")
	// 			}
	// 		}
	// 	}
	// }
	log.Printf("Sending chunk [%v] [%s] [%s] to [%v]", pos, p.locWorld, p.locDimension, p.player.Name)
}
func (s *chunkLoader) processChunkLoadingForPlayer(p *playerData) {
	center := level.ChunkPos{X: int(p.x) / 16, Z: int(p.z) / 16}
	log.Printf("Updating player [%v] chunks, position %v", p.player.Name, center)
	for v := range p.viewingChunks {
		p.viewingChunks[v] = false
	}
	vd := p.viewDistance
	for x := center.X - (vd/2 - 1); x < center.X+(vd/2-1); x++ {
		for z := center.Z - (vd/2 - 1); z < center.Z+(vd/2-1); z++ {
			c := level.ChunkPos{X: x, Z: z}
			_, found := p.viewingChunks[c]
			if !found {
				go s.sendChunk(c, p)
			}
			p.viewingChunks[c] = true
		}
	}
	keys := []level.ChunkPos{}
	for v := range p.viewingChunks {
		keys = append(keys, v)
	}
	for _, v := range keys {
		if !p.viewingChunks[v] {
			SendUnloadChunk(p.player, int32(v.X), int32(v.Z))
			delete(p.viewingChunks, v)
		}
	}

}
func (s *chunkLoader) Run(ctx context.Context) {
	chunkUpdateTick := time.NewTicker(1 * time.Second)
	log.Print("Started chunk provider")
	for {
		select {
		case <-ctx.Done():
			log.Print("Chunk loader shuts down")
			chunkUpdateTick.Stop()
			return
		case <-chunkUpdateTick.C:
			s.playersMutex.Lock()
			for _, k := range s.players {
				s.processChunkLoadingForPlayer(k)
			}
			s.playersMutex.Unlock()
		}
	}
}
func (s *chunkLoader) TeleportPlayer(u uuid.UUID, pos BlockPositionData) {
	s.playersMutex.Lock()
	player, ok := s.players[u]
	s.playersMutex.Unlock()
	if !ok {
		log.Println("Failed to teleport player " + u.String() + ", not in players map")
		return
	}
	var X, Y, Z float32
	if pos.Relative[0] {
		X = float32(pos.X) + float32(player.x)
	} else {
		X = float32(pos.X)
	}
	if pos.Relative[1] {
		Y = float32(pos.Y) + float32(player.y)
	} else {
		Y = float32(pos.Y)
	}
	if pos.Relative[2] {
		Z = float32(pos.Z) + float32(player.z)
	} else {
		Z = float32(pos.Z)
	}
	SendPlayerPositionAndLook(player.player, X, Y, Z, 0.0, 0.0, 0, 420, false)
	SendSystemMessage(player.player, chat.Text(fmt.Sprintf("You were teleported to %.1f %.1f %.1f", X, Y, Z)).SetColor(chat.DarkAqua))
}

func (s *chunkLoader) generateHubChunk(x, z int) *level.Chunk {
	c := level.EmptyChunk(32)
	bid := block.ToStateID[block.AncientDebris{}]
	// SetChunkBlock(c, 4, 64, 8, bid)
	// SetChunkBlock(c, 4, 64, 8, block.ToStateID[block.AncientDebris{}])
	for i := 0; i < 16*16*16; i++ {
		c.Sections[0].SetBlock(i, bid)
	}
	return c
}

// considers lowest section is y -64
func SetChunkBlock(c *level.Chunk, lx, ly, lz int, bs block.StateID) {
	sid := (ly+64)/16 + 4
	if len(c.Sections) <= int(sid) {
		log.Printf("Failed to set block %d at %d %d %d because there is no section %d (%d allocated)", bs, lx, ly, lz, sid, len(c.Sections))
		return
	}
	c.Sections[sid].SetBlock((ly%16)*16*16+lz*16+lx, bs)
}

func (s *chunkLoader) SetPlayerWorldDim(u uuid.UUID, world, dim string) {
	s.playersMutex.Lock()
	p, ok := s.players[u]
	s.playersMutex.Unlock()
	if !ok {
		log.Println("Failed to set player world dimension, not found in players map")
		return
	}
	_, st, err := chunkStorage.GetWorldStorage(s.s, world)
	if err != nil {
		log.Println("Failed to get world [" + world + "] storage: " + err.Error())
		SendSystemMessage(p.player, chat.Text("Failed to get world ["+world+"] storage: "+err.Error()).SetColor("red"))
		return
	}
	if st == nil {
		SendSystemMessage(p.player, chat.Text("World '"+world+"' not found").SetColor("red"))
		return
	}
	d, err := st.GetDimension(world, dim)
	if err != nil {
		log.Println("Failed to get world [" + world + "] dimension [" + dim + "]: " + err.Error())
		SendSystemMessage(p.player, chat.Text("Failed to get world ["+world+"] dimension ["+dim+"]: "+err.Error()).SetColor("red"))
		return
	}
	if d == nil {
		SendSystemMessage(p.player, chat.Text("Dimension '"+dim+"' of world '"+world+"' not found").SetColor("red"))
		return
	}
	p.locWorld = world
	p.locDimension = dim
	p.viewingChunks = map[level.ChunkPos]bool{}
}

func (s *chunkLoader) SetPlayerRenderDistance(u uuid.UUID, distance int) {
	s.playersMutex.Lock()
	p, ok := s.players[u]
	s.playersMutex.Unlock()
	if !ok {
		return
	}
	p.viewDistance = distance
}

func ActualChunkFromSave(c *save.Chunk, totalWorldHight int) *level.Chunk {
	ret := *level.ChunkFromSave(c)
	b := bits.Len(uint(totalWorldHight / 16))
	ret.HeightMaps.MotionBlocking = buildMBHeightmap(&ret, b)
	// sections := make([]level.Section, len(c.Sections))
	// for _, v := range c.Sections {
	// 	var blockCount int16
	// 	stateData := *(*[]uint64)((unsafe.Pointer)(&v.BlockStates.Data))
	// 	statePalette := v.BlockStates.Palette
	// 	stateRawPalette := make([]block.StateID, len(statePalette))
	// 	for i, v := range statePalette {
	// 		b, ok := block.FromID[v.Name]
	// 		if !ok {
	// 			return nil
	// 		}
	// 		if v.Properties.Data != nil {
	// 			err := v.Properties.Unmarshal(&b)
	// 			if err != nil {
	// 				return nil
	// 			}
	// 		}
	// 		s := block.ToStateID[b]
	// 		if !isAir(int(s)) {
	// 			blockCount++
	// 		}
	// 		stateRawPalette[i] = s
	// 	}

	// 	biomesData := *(*[]uint64)((unsafe.Pointer)(&v.Biomes.Data))
	// 	biomesPalette := v.Biomes.Palette
	// 	biomesRawPalette := make([]level.BiomesState, len(biomesPalette))
	// 	for i, v := range biomesPalette {
	// 		biomesRawPalette[i] = level.BiomesState(biomesIDs[strings.TrimPrefix(v, "minecraft:")])
	// 	}

	// 	i := int32(v.Y) - c.YPos
	// 	sections[i].BlockCount = blockCount
	// 	sections[i].States = level.NewStatesPaletteContainerWithData(16*16*16, stateData, stateRawPalette)
	// 	sections[i].Biomes = level.NewBiomesPaletteContainerWithData(4*4*4, biomesData, biomesRawPalette)
	// }
	// for i := range sections {
	// 	if sections[i].States == nil {
	// 		sections[i] = level.Section{
	// 			BlockCount: 0,
	// 			States:     level.NewStatesPaletteContainer(16*16*16, 0),
	// 			Biomes:     level.NewBiomesPaletteContainer(4*4*4, 0),
	// 		}
	// 	}
	// }
	// motionBlocking := *(*[]uint64)(unsafe.Pointer(&c.Heightmaps.MotionBlocking))
	// worldSurface := *(*[]uint64)(unsafe.Pointer(&c.Heightmaps.WorldSurface))
	// ret := level.Chunk{
	// 	Sections: sections,
	// 	HeightMaps: level.HeightMaps{
	// 		MotionBlocking: level.NewBitStorage(bits.Len(uint(len(c.Sections))), 16*16, motionBlocking),
	// 		WorldSurface:   level.NewBitStorage(bits.Len(uint(len(c.Sections))), 16*16, worldSurface),
	// 	},
	// }
	var blockEntitiesData []nbt.RawMessage
	err := c.BlockEntities.Unmarshal(&blockEntitiesData)
	if err != nil {
		log.Println("Error unmarshling block entities: " + err.Error())
		return &ret
	}
	for _, rawdata := range blockEntitiesData {
		var be map[string]interface{}
		err = rawdata.Unmarshal(&be)
		if err != nil {
			log.Printf("Failed to unmarshal raw block entity data: %s", err.Error())
			continue
		}
		var x, y, z int32
		var ok bool
		var id string
		if x, ok = be["x"].(int32); !ok {
			log.Printf("Invalid block entity, x wrong: %##v", be)
			spew.Dump(be)
			continue
		}
		if y, ok = be["y"].(int32); !ok {
			log.Printf("Invalid block entity, y wrong: %##v", be)
			spew.Dump(be)
			continue
		}
		if z, ok = be["z"].(int32); !ok {
			log.Printf("Invalid block entity, z wrong: %##v", be)
			spew.Dump(be)
			continue
		}
		if id, ok = be["id"].(string); !ok {
			log.Printf("Invalid block entity, id wrong: %##v", be)
			spew.Dump(be)
			continue
		}
		ret.BlockEntity = append(ret.BlockEntity, level.BlockEntity{
			XZ:   int8(((x & 15) << 4) | (z & 15)),
			Y:    int16(y - 64),
			Type: BlockEntityTypes[strings.TrimPrefix(id, "minecraft:")],
			Data: rawdata,
		})
		log.Printf("Block entity stored: %s, %s", id, rawdata.String())
	}
	for iii := range ret.BlockEntity {
		log.Printf("Block entity sent: %d, %s", ret.BlockEntity[iii].Type, ret.BlockEntity[iii].Data.String())
	}
	_ = ret.BlockEntity
	return &ret
}

var biomesIDs = map[string]int{
	"the_void":                 0,
	"plains":                   1,
	"sunflower_plains":         2,
	"snowy_plains":             3,
	"ice_spikes":               4,
	"desert":                   5,
	"swamp":                    6,
	"forest":                   7,
	"flower_forest":            8,
	"birch_forest":             9,
	"dark_forest":              10,
	"old_growth_birch_forest":  11,
	"old_growth_pine_taiga":    12,
	"old_growth_spruce_taiga":  13,
	"taiga":                    14,
	"snowy_taiga":              15,
	"savanna":                  16,
	"savanna_plateau":          17,
	"windswept_hills":          18,
	"windswept_gravelly_hills": 19,
	"windswept_forest":         20,
	"windswept_savanna":        21,
	"jungle":                   22,
	"sparse_jungle":            23,
	"bamboo_jungle":            24,
	"badlands":                 25,
	"eroded_badlands":          26,
	"wooded_badlands":          27,
	"meadow":                   28,
	"grove":                    29,
	"snowy_slopes":             30,
	"frozen_peaks":             31,
	"jagged_peaks":             32,
	"stony_peaks":              33,
	"river":                    34,
	"frozen_river":             35,
	"beach":                    36,
	"snowy_beach":              37,
	"stony_shore":              38,
	"warm_ocean":               39,
	"lukewarm_ocean":           40,
	"deep_lukewarm_ocean":      41,
	"ocean":                    42,
	"deep_ocean":               43,
	"cold_ocean":               44,
	"deep_cold_ocean":          45,
	"frozen_ocean":             46,
	"deep_frozen_ocean":        47,
	"mushroom_fields":          48,
	"dripstone_caves":          49,
	"lush_caves":               50,
	"nether_wastes":            51,
	"warped_forest":            52,
	"crimson_forest":           53,
	"soul_sand_valley":         54,
	"basalt_deltas":            55,
	"the_end":                  56,
	"end_highlands":            57,
	"end_midlands":             58,
	"small_end_islands":        59,
	"end_barrens":              60,
}

func buildMBHeightmap(c *level.Chunk, b int) *level.BitStorage {
	ret := level.NewBitStorage(b, 16*16, nil)
	for i, s := range c.Sections {
		if s.BlockCount == 0 {
			continue
		}
		for y := 0; y < 16; y++ {
			ay := i*16 + y
			for i := 0; i < 16*16; i++ {
				if !block.IsAir(s.States.Get(i)) {
					if ret.Get(i) < ay {
						ret.Set(i, ay)
					}
				}
			}
		}
	}
	return ret
}
