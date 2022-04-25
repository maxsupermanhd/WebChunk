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
	"sync"
	"time"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	"github.com/Tnze/go-mc/level"
	"github.com/Tnze/go-mc/level/block"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/server"
	"github.com/google/uuid"
	"github.com/maxsupermanhd/mcwebchunk/chunkStorage"
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
		F: func(player *server.Player, packet server.Packet758) error {
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
			p.x = float64(x)
			p.y = float64(y)
			p.z = float64(z)
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
		save, err := storage.GetChunk(p.locWorld, p.locDimension, pos.X, pos.Z)
		if err != nil {
			log.Printf("Failed to get chunk %v: %v", pos, err.Error())
			return
		}
		if save == nil {
			log.Println("Chunk not found")
			chunk = level.EmptyChunk(256)
		} else {
			chunk = level.ChunkFromSave(save, 256)
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
func SetChunkBlock(c *level.Chunk, lx, ly, lz, bs int) {
	sid := (ly+64)/16 + 4
	if len(c.Sections) <= sid {
		log.Printf("Failed to set block %d at %d %d %d because there is no section %d (%d allocated)", bs, lx, ly, lz, sid, len(c.Sections))
		return
	}
	c.Sections[sid].SetBlock((ly%16)*16*16+lx*16+lz, bs)
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
