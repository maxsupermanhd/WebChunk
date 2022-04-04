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
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/server"
	"github.com/Tnze/go-mc/server/command"
	"github.com/google/uuid"
	"github.com/maxsupermanhd/mcwebchunk/chunkStorage"
)

type timeUpdater struct {
	playersMutex sync.Mutex
	players      map[uuid.UUID]*server.Player
}

func (s *timeUpdater) Init(g *server.Game) {
	s.players = map[uuid.UUID]*server.Player{}
}
func (s *timeUpdater) AddPlayer(p *server.Player) {
	s.playersMutex.Lock()
	s.players[p.UUID] = p
	s.playersMutex.Unlock()
}
func (s *timeUpdater) RemovePlayer(p *server.Player) {
	s.playersMutex.Lock()
	delete(s.players, p.UUID)
	s.playersMutex.Unlock()
}
func (s *timeUpdater) Run(ctx context.Context) {
	chunkUpdateTick := time.NewTicker(time.Second / 20)
	for {
		select {
		case <-ctx.Done():
			log.Print("Chunk loader shuts down")
			chunkUpdateTick.Stop()
			return
		case <-chunkUpdateTick.C:
			s.playersMutex.Lock()
			for _, p := range s.players {
				p.WritePacket(server.Packet758(pk.Marshal(
					packetid.ClientboundSetTime,
					pk.Long(0),
					pk.Long(0),
				)))
			}
			s.playersMutex.Unlock()
		}
	}
}

type commandExecutor struct {
	players      map[uuid.UUID]*server.Player
	playersMutex sync.Mutex
}

func (s *commandExecutor) Init(g *server.Game) {
	s.players = map[uuid.UUID]*server.Player{}
}
func (s *commandExecutor) AddPlayer(p *server.Player) {
	s.playersMutex.Lock()
	s.players[p.UUID] = p
	s.playersMutex.Unlock()
}
func (s *commandExecutor) RemovePlayer(p *server.Player) {
	s.playersMutex.Lock()
	delete(s.players, p.UUID)
	s.playersMutex.Unlock()
}
func (s *commandExecutor) Run(ctx context.Context) {}
func (s *commandExecutor) ExecuteCommand(f func(player *server.Player, args []command.ParsedData) error) command.HandlerFunc {
	return func(ctx context.Context, args []command.ParsedData) error {
		s.playersMutex.Lock()
		pl := s.players[uuid.MustParse(ctx.Value("sender").(string))]
		s.playersMutex.Unlock()
		return f(pl, args)
	}
}

func StartReconstructor(storage chunkStorage.ChunkStorage) {
	if storage == nil {
		return
	}
	maxplayers, err := strconv.Atoi(os.Getenv("RECONSTRUCTOR_MAXPLAYERS"))
	if err != nil {
		log.Print("Failed to read RECONSTRUCTOR_MAXPLAYERS, setting to 20")
		maxplayers = 20
	}
	playerList := server.NewPlayerList(maxplayers)
	serverInfo, err := server.NewPingInfo(playerList, server.ProtocolName, server.ProtocolVersion, chat.Text("Hello world"), nil)
	if err != nil {
		log.Fatalf("Set server info error: %v", err)
	}
	keepAlive := server.NewKeepAlive()
	playerInfo := server.NewPlayerInfo(time.Second, keepAlive)
	chunkLoader := NewChunkLoader(storage, 15, "simply", "overworld", [2]int{4350, 8100})
	commands := command.NewGraph()
	executor := &commandExecutor{}
	commands.AppendLiteral(commands.Literal("tp").
		AppendArgument(commands.Argument("position", command.StringParser(2)).
			HandleFunc(executor.ExecuteCommand(func(player *server.Player, args []command.ParsedData) error {
				return nil
			}))).
		HandleFunc(executor.ExecuteCommand(func(player *server.Player, args []command.ParsedData) error {
			return nil
		})))
	dim := &dimensionProvider{"overworld", 0}
	timeUpd := &timeUpdater{}
	// dim2, err := loadAllRegions("/home/max/.local/share/multimc/instances/fabric-1.18.2-hax/.minecraft/saves/New World/region/")
	if err != nil {
		log.Fatal(err)
	}
	game := server.NewGame(
		dim,
		playerList,
		playerInfo,
		keepAlive,
		server.NewGlobalChat(),
		chunkLoader,
		timeUpd,
		commands,
		executor,
	)
	go game.Run(context.Background())

	s := server.Server{
		ListPingHandler: serverInfo,
		LoginHandler: &server.MojangLoginHandler{
			OnlineMode:   false,
			Threshold:    256,
			LoginChecker: playerList,
		},
		GamePlay: game,
	}

	if err := s.Listen(os.Getenv("MINECRAFT_LISTEN")); err != nil {
		log.Fatalf("Listen error: %v", err)
	}
}
