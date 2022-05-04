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
	"image"
	"log"
	"os"
	"sync"
	"time"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/server"
	"github.com/Tnze/go-mc/server/command"
	"github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
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
	timeUpdateTick := time.NewTicker(time.Second / 20)
	for {
		select {
		case <-ctx.Done():
			timeUpdateTick.Stop()
			return
		case <-timeUpdateTick.C:
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

type ReconstructorConfig struct {
	MOTD                chat.Message `json:"motd"`
	MaxPlayers          int          `json:"maxplayers"`
	IconPath            string       `json:"icon"`
	Listen              string       `json:"listen"`
	DefaultViewDistance int          `json:"default_view_distance"`
	CompressThreshold   int          `json:"compress_threshold"`
	OnlineMode          bool         `json:"online_mode"`
}

func StartReconstructor(storage []chunkStorage.Storage, conf *ReconstructorConfig) {
	var icon image.Image
	if conf.IconPath != "" {
		f, err := os.Open(conf.IconPath)
		if err != nil {
			log.Println("Failed to open proxy server icon: " + err.Error())
		} else {
			icon, _, err = image.Decode(f)
			if err != nil {
				log.Println("Failed to decode proxy server icon: " + err.Error())
				icon = nil
			}
			f.Close()
		}
	}
	if storage == nil {
		return
	}
	serverInfo, err := server.NewPingInfo(server.NewPlayerList(conf.MaxPlayers), server.ProtocolName, server.ProtocolVersion, conf.MOTD, icon)
	if err != nil {
		log.Fatalf("Set server info error: %v", err)
	}
	keepAlive := server.NewKeepAlive()
	playerInfo := server.NewPlayerInfo(time.Second, keepAlive)
	chunkLoader := NewChunkLoader(storage, conf.DefaultViewDistance)
	commands := command.NewGraph()
	// executor := &commandExecutor{}
	commands.AppendLiteral(commands.Literal("tp").
		AppendArgument(commands.Argument("position", BlockPosParser{}).
			HandleFunc(func(ctx context.Context, args []command.ParsedData) error {
				chunkLoader.TeleportPlayer(ctx.Value("sender").(*server.Player).UUID, args[2].(BlockPositionData))
				return nil
			})).
		HandleFunc(func(ctx context.Context, args []command.ParsedData) error {
			spew.Dump(args)
			return nil
		}))
	commands.AppendLiteral(commands.Literal("renderdistance").
		AppendArgument(commands.Argument("distance", NewIntegerParser(6, 32)).
			HandleFunc(func(ctx context.Context, args []command.ParsedData) error {
				v := int(args[2].(int64))
				chunkLoader.SetPlayerRenderDistance(ctx.Value("sender").(*server.Player).UUID, v)
				SendSystemMessage(ctx.Value("sender").(*server.Player), chat.Text(fmt.Sprintf("Render distance is set to %d", v)))
				return nil
			})).
		HandleFunc(func(ctx context.Context, args []command.ParsedData) error {
			spew.Dump(args)
			return nil
		}))
	commands.AppendLiteral(commands.Literal("worlds").
		HandleFunc(func(ctx context.Context, args []command.ParsedData) error {
			player := ctx.Value("sender").(*server.Player)
			worlds := chunkStorage.ListWorlds(storage)
			msg := chat.Text(fmt.Sprintf("Worlds: %d\n", len(worlds)))
			for i := range worlds {
				msg.Extra = append(msg.Extra, chat.Text(fmt.Sprintf("%s (%s)\n", worlds[i].Name, worlds[i].IP)))
				dims, err := chunkStorage.ListDimensions(storage, worlds[i].Name)
				if err != nil {
					log.Println("Failed to list dimensions: " + err.Error())
				}
				for j := range dims {
					sym := "┣"
					if len(dims) == 1 || j == len(dims)-1 {
						sym = "┗"
					}
					d := chat.Text(fmt.Sprintf("%s━━ %s (%s)\n", sym, dims[j].Name, dims[j].Alias))
					d.ClickEvent = chat.RunCommand(fmt.Sprintf("/go %s %s", worlds[i].Name, dims[j].Name))
					msg.Extra = append(msg.Extra, d)
				}
			}
			// m, _ := json.MarshalIndent(msg, "", "    ")
			// log.Println(string(m))
			SendSystemMessage(player, msg)
			return nil
		}))
	commands.AppendLiteral(commands.Literal("go").
		AppendArgument(commands.Argument("world", command.StringParser(1)).
			AppendArgument(commands.Argument("dimension", command.StringParser(1)).
				HandleFunc(func(ctx context.Context, args []command.ParsedData) error {
					pl := ctx.Value("sender").(*server.Player)
					world := args[2].(string)
					dim := args[3].(string)
					SendSystemMessage(pl, chat.Text(fmt.Sprintf("Moving you to [%s] [%s]", world, dim)))
					chunkLoader.SetPlayerWorldDim(ctx.Value("sender").(*server.Player).UUID, world, dim)
					return nil
				})).
			HandleFunc(func(ctx context.Context, args []command.ParsedData) error {
				return nil
			})).
		HandleFunc(func(ctx context.Context, args []command.ParsedData) error {
			pl := ctx.Value("sender").(*server.Player)
			SendSystemMessage(pl, chat.Text("Moving you to back to lobby"))
			chunkLoader.SetPlayerWorldDim(ctx.Value("sender").(*server.Player).UUID, "", "")
			return nil
		}))
	dim := &dimensionProvider{"overworld", 0}
	timeUpd := &timeUpdater{}
	// dim2, err := loadAllRegions("/home/max/.local/share/multimc/instances/fabric-1.18.2-hax/.minecraft/saves/New World/region/")
	if err != nil {
		log.Fatal(err)
	}
	game := server.NewGame(
		dim,
		playerInfo,
		keepAlive,
		server.NewGlobalChat(),
		chunkLoader,
		timeUpd,
		// executor,
		commands,
	)
	go game.Run(context.Background())

	s := server.Server{
		ListPingHandler: serverInfo,
		LoginHandler: &server.MojangLoginHandler{
			OnlineMode:   conf.OnlineMode,
			Threshold:    conf.CompressThreshold,
			LoginChecker: nil,
		},
		GamePlay: game,
	}

	if err := s.Listen(conf.Listen); err != nil {
		log.Fatalf("Listen error: %v", err)
	}
}

// type commandExecutor struct {
// 	players      map[uuid.UUID]*server.Player
// 	playersMutex sync.Mutex
// }

// func (s *commandExecutor) Init(g *server.Game) {
// 	s.players = map[uuid.UUID]*server.Player{}
// }
// func (s *commandExecutor) AddPlayer(p *server.Player) {
// 	s.playersMutex.Lock()
// 	s.players[p.UUID] = p
// 	s.playersMutex.Unlock()
// }
// func (s *commandExecutor) RemovePlayer(p *server.Player) {
// 	s.playersMutex.Lock()
// 	delete(s.players, p.UUID)
// 	s.playersMutex.Unlock()
// }
// func (s *commandExecutor) Run(ctx context.Context) {}
// func (s *commandExecutor) ExecuteCommand(f func(player *server.Player, args []command.ParsedData) error) command.HandlerFunc {
// 	return func(ctx context.Context, args []command.ParsedData) error {
// 		log.Println("Command arrived!")
// 		s.playersMutex.Lock()
// 		pl := s.players[uuid.MustParse(ctx.Value("sender").(string))]
// 		s.playersMutex.Unlock()
// 		return f(pl, args)
// 	}
// }
