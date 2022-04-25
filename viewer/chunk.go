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
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/server"
	"github.com/google/uuid"
	"github.com/maxsupermanhd/mcwebchunk/chunkStorage"
)

type playerPosition struct {
	x, y, z float64
}

type chunkLoader struct {
	g                    *server.Game
	s                    chunkStorage.ChunkStorage
	viewDistance         int
	enteranceCoordinates [2]int
	dbServer             string
	dbDim                string
	players              map[uuid.UUID]*server.Player
	positions            map[uuid.UUID]playerPosition
	viewingChunks        map[uuid.UUID]map[level.ChunkPos]bool
	playersMutex         sync.Mutex
}

func NewChunkLoader(s chunkStorage.ChunkStorage, viewDistance int, dbServer, dbDim string, ec [2]int) *chunkLoader {
	return &chunkLoader{
		s:                    s,
		viewDistance:         viewDistance,
		dbServer:             dbServer,
		dbDim:                dbDim,
		enteranceCoordinates: ec,
	}
}
func (s *chunkLoader) Init(g *server.Game) {
	s.g = g
	s.players = map[uuid.UUID]*server.Player{}
	s.positions = map[uuid.UUID]playerPosition{}
	s.viewingChunks = map[uuid.UUID]map[level.ChunkPos]bool{}
	g.AddHandler(&server.PacketHandler{
		ID: packetid.ServerboundMovePlayerPos,
		F: func(player *server.Player, packet server.Packet758) error {
			var x, y, z pk.Double
			var ground pk.Boolean
			if err := pk.Packet(packet).Scan(&x, &y, &z, &ground); err != nil {
				return err
			}
			newpos := playerPosition{x: float64(x), y: float64(y), z: float64(z)}
			if s.positions[player.UUID] != newpos {
				SendUpdateViewPosition(player, int32(float64(x)/16), int32(float64(z)/16))
			}
			s.positions[player.UUID] = newpos
			log.Printf("Player [%s] updated positon [%.1f %.1f %.1f]", player.Name, newpos.x, newpos.y, newpos.z)
			return nil
		},
	})
}
func (s *chunkLoader) AddPlayer(p *server.Player) {
	log.Printf("Player [%v] (%v) joined", p.Name, p.UUID)
	s.playersMutex.Lock()
	sendpos := func() {
		SendPlayerPositionAndLook(p, float32(s.enteranceCoordinates[0]), 180, float32(s.enteranceCoordinates[1]), 0, 0, 0, 69, false)
		SendUpdateViewPosition(p, int32(s.enteranceCoordinates[0]/16), int32(s.enteranceCoordinates[1]/16))
	}
	sendpos()
	s.players[p.UUID] = p
	s.positions[p.UUID] = playerPosition{x: float64(s.enteranceCoordinates[0]), y: 64.0, z: float64(s.enteranceCoordinates[0])}
	s.viewingChunks[p.UUID] = map[level.ChunkPos]bool{}
	s.playersMutex.Unlock()
}
func (s *chunkLoader) RemovePlayer(p *server.Player) {
	log.Printf("Player [%v] left", p.Name)
	s.playersMutex.Lock()
	delete(s.players, p.UUID)
	delete(s.positions, p.UUID)
	delete(s.viewingChunks, p.UUID)
	s.playersMutex.Unlock()
}
func (s *chunkLoader) sendChunk(pos level.ChunkPos, p *server.Player) {
	save, err := s.s.GetChunk(s.dbDim, s.dbServer, pos.X, pos.Z)
	if err != nil {
		log.Printf("Failed to get chunk %v: %v", pos, err.Error())
		return
	}
	var chunk *level.Chunk
	if save == nil {
		chunk = level.EmptyChunk(265)
	} else {
		chunk = level.ChunkFromSave(save, 256)
	}
	if chunk == nil {
		log.Printf("Failed to get chunk %v, nil conversion", pos)
		return
	}
	p.WritePacket(server.Packet758(pk.Marshal(
		packetid.ClientboundLevelChunkWithLight,
		pos, chunk,
	)))
	log.Printf("Sending chunk [%v] to [%v]", pos, p.Name)
}
func (s *chunkLoader) processChunkLoadingForPlayer(id uuid.UUID) {
	centerpos, ok := s.positions[id]
	if !ok {
		return
	}
	center := level.ChunkPos{X: int(centerpos.x) / 16, Z: int(centerpos.z) / 16}
	log.Printf("Updating player [%v] chunks, position %v", id, center)
	view, ok := s.viewingChunks[id]
	if !ok {
		return
	}
	for v := range view {
		view[v] = false
	}
	for x := center.X - (s.viewDistance/2 - 1); x < center.X+(s.viewDistance/2-1); x++ {
		for z := center.Z - (s.viewDistance/2 - 1); z < center.Z+(s.viewDistance/2-1); z++ {
			c := level.ChunkPos{X: x, Z: z}
			_, found := view[c]
			if !found {
				go s.sendChunk(c, s.players[id])
			}
			view[c] = true
		}
	}
	keys := []level.ChunkPos{}
	for v := range view {
		keys = append(keys, v)
	}
	for _, v := range keys {
		if !view[v] {
			SendUnloadChunk(s.players[id], int32(v.X), int32(v.Z))
			delete(view, v)
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
			for k := range s.players {
				s.processChunkLoadingForPlayer(k)
			}
			s.playersMutex.Unlock()
		}
	}
}
func (s *chunkLoader) TeleportPlayer(u uuid.UUID, pos BlockPositionData) {
	s.playersMutex.Lock()
	player, ok := s.players[u]
	if !ok {
		log.Println("Failed to teleport player " + u.String() + ", not in players map")
		s.playersMutex.Unlock()
		return
	}
	playerPos, ok := s.positions[u]
	if !ok {
		log.Println("Failed to teleport player " + u.String() + ", not in positions map")
		s.playersMutex.Unlock()
		return
	}
	s.playersMutex.Unlock()
	var X, Y, Z float32
	if pos.Relative[0] {
		X = float32(pos.X) + float32(playerPos.x)
	} else {
		X = float32(pos.X)
	}
	if pos.Relative[1] {
		Y = float32(pos.Y) + float32(playerPos.y)
	} else {
		Y = float32(pos.Y)
	}
	if pos.Relative[2] {
		Z = float32(pos.Z) + float32(playerPos.z)
	} else {
		Z = float32(pos.Z)
	}
	SendPlayerPositionAndLook(player, X, Y, Z, 0.0, 0.0, 0, 420, false)
	SendChatMessage(player, chat.Text(fmt.Sprintf("You were teleported to %.1f %.1f %.1f", X, Y, Z)).SetColor(chat.DarkAqua))
}
