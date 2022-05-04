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
	"image"
	"log"
	"os"
	"strings"

	"github.com/Tnze/go-mc/bot"
	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	"github.com/Tnze/go-mc/level"
	"github.com/Tnze/go-mc/level/block"
	"github.com/Tnze/go-mc/nbt"
	"github.com/Tnze/go-mc/net"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/server"
	"github.com/davecgh/go-spew/spew"
	"github.com/google/uuid"
	"github.com/maxsupermanhd/WebChunk/credentials"
	"github.com/maxsupermanhd/WebChunk/viewer"
)

type ProxiedPacket struct {
	Username string
	Server   string
	Packet   pk.Packet
}

type ProxiedChunk struct {
	Username  string
	Server    string
	Dimension string
	Pos       level.ChunkPos
	Data      level.Chunk
}

type MessageFeedback struct {
	To   string
	Type string // "chat", "system" or "info"
	Msg  chat.Message
}

type ProxyConfig struct {
	MOTD              chat.Message `json:"motd"`
	MaxPlayers        int          `json:"maxplayers"`
	IconPath          string       `json:"icon"`
	Listen            string       `json:"listen"`
	CredentialsPath   string       `json:"credentials"`
	CompressThreshold int          `json:"compress_threshold"`
	OnlineMode        bool         `json:"online_mode"`
}

var collectPackets = []int{
	packetid.ClientboundLevelChunkWithLight,
	packetid.ClientboundBlockEntityData,
	packetid.ClientboundForgetLevelChunk,
	packetid.ClientboundLogin,
	packetid.ClientboundRespawn,
}

func RunProxy(routeHandler func(name string) string, conf *ProxyConfig, dump chan *ProxiedChunk) {
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
	serverInfo, err := server.NewPingInfo(server.NewPlayerList(conf.MaxPlayers), server.ProtocolName, server.ProtocolVersion, conf.MOTD, icon)
	if err != nil {
		log.Fatalf("Failed to create server ping information: %v", err)
		return
	}
	s := server.Server{
		ListPingHandler: serverInfo,
		LoginHandler: &server.MojangLoginHandler{
			OnlineMode:   conf.OnlineMode,
			Threshold:    conf.CompressThreshold,
			LoginChecker: nil,
		},
		GamePlay: SnifferProxy{
			Routing:     routeHandler,
			CredManager: credentials.NewMicrosoftCredentialsManager(conf.CredentialsPath, "88650e7e-efee-4857-b9a9-cf580a00ef43"),
			SaveChannel: dump,
		},
	}
	log.Println("Started proxy on " + conf.Listen)
	log.Println(s.Listen(conf.Listen))
}

type SnifferProxy struct {
	Routing     func(name string) string
	CredManager *credentials.MicrosoftCredentialsManager
	SaveChannel chan *ProxiedChunk
}

func (p SnifferProxy) AcceptPlayer(name string, id uuid.UUID, _ int32, conn *net.Conn) {
	log.Printf("Accepting new player [%s] (%s), getting route...", name, id.String())
	dest := p.Routing(name)
	if dest == "" {
		log.Printf("Unable to find route for [%s]", name)
		dissconnectWithMessage(conn, &chat.Message{Text: "Dissconnected before login: Routing failed"})
		return
	}
	log.Printf("Accepting new player [%s] (%s), adding events...", name, id.String())
	c := bot.NewClient()
	a := make(chan pk.Packet, 2048)
	defer close(a)
	go packetAcceptor(a, p.SaveChannel, name, dest)
	// forward all packet from server to player
	c.Events.AddGeneric(bot.PacketHandler{
		Priority: 100,
		F: func(pk pk.Packet) error {
			// log.Printf("s->c  %d", pk.ID)
			for i := 0; i < len(collectPackets); i++ {
				if collectPackets[i] == int(pk.ID) {
					a <- pk
					break
				}
			}
			return conn.WritePacket(pk)
		},
	})
	log.Printf("Accepting new player [%s] (%s), getting auth...", name, id.String())
	auth, err := p.CredManager.GetAuthForUsername(name)
	if err != nil {
		log.Printf("Error preparing auth for player [%s]: %v", name, err)
		dissconnectWithError(conn, err)
		return
	}
	if auth == nil {
		log.Printf("Error preparing auth for player [%s]: auth is nil", name)
		dissconnectWithMessage(conn, &chat.Message{Text: "Dissconnected before login: Auth is nil"})
		return
	}
	c.Auth = *auth
	log.Printf("Accepting new player [%s] (%s), dialing [%s]...", name, id.String(), dest)
	if err := c.JoinServer(dest); err != nil {
		log.Printf("Failed to accept new player [%s] (%s), error connecting to [%s]: %v", name, id.String(), dest, err)
		dissconnectWithMessage(conn, &chat.Message{Text: strings.TrimPrefix(err.Error(), "bot: disconnect error: disconnect because: ")})
		return
	}
	defer c.Close()
	go func() {
		var pk pk.Packet
		var err error
		for {
			err = conn.ReadPacket(&pk)
			if err != nil {
				break
			}
			// log.Printf("c->s  %d", pk.ID)
			err = c.Conn.WritePacket(pk)
			if err != nil {
				break
			}
		}
		log.Printf("Player [%s] left from server [%s] (s->c): %v", name, dest, err)
	}()
	if err := c.HandleGame(); err != nil {
		log.Printf("Player [%s] left from server [%s] (c->s): %v", name, dest, err)
	}
}

func dissconnectWithError(conn *net.Conn, reason error) {
	dissconnectWithMessage(conn, &chat.Message{Text: fmt.Sprint(reason)})
}

func dissconnectWithMessage(conn *net.Conn, reason *chat.Message) {
	conn.WritePacket(pk.Marshal(packetid.ClientboundDisconnect, reason))
}

func packetAcceptor(recv chan pk.Packet, resp chan *ProxiedChunk, username, server string) {
	c := map[struct {
		pos level.ChunkPos
		dim string
	}]struct {
		chunk level.Chunk
	}{}
	loadedDims := map[string]struct {
		id   int32
		minY int32
	}{}
	currentDim := ""
	// nextpacket:
	for p := range recv {
		switch {
		case p.ID == packetid.ClientboundLevelChunkWithLight:
			if currentDim == "" {
				log.Println("Recieved chunk without dimension")
				continue
			}
			var cpos level.ChunkPos
			var cc level.Chunk
			err := p.Scan(&cpos, &cc)
			if err != nil {
				log.Printf("Failed to scan chunk packet: %v", err.Error())
				continue
			}
			// verify all block entities are present
			blockEntities := map[string]string{}
			for _, sect := range cc.Sections {
				if sect.BlockCount == 0 {
					continue
				}
				// another way around this ugly loop will be to check palette but oh well...
				for y := 0; y < 16; y++ {
					for x := 0; x < 16; x++ {
						for z := 0; z < 16; z++ {
							blo := sect.GetBlock(y*16*16 + z*16 + x)
							sta := block.StateList[blo]
							for b := range viewer.BlockEntityTypes {
								if b == strings.TrimPrefix(sta.ID(), "minecraft:") {
									blockEntities[fmt.Sprintf("%d %d %d", x, y, z)] = sta.ID()
								}
							}
						}
					}
				}
			}
			if len(blockEntities) < len(cc.BlockEntity) { // what if there is more than in sections?
				// send to cache for completion
				c[struct {
					pos level.ChunkPos
					dim string
				}{
					pos: cpos,
					dim: currentDim,
				}] = struct{ chunk level.Chunk }{cc}
				log.Printf("Sending out chunk %d:%d to cache since missing %d block entities", cpos.X, cpos.Z, len(cc.BlockEntity)-len(blockEntities))
			} else {
				// send directly to storage because ready
				resp <- &ProxiedChunk{
					Username:  username,
					Server:    server,
					Dimension: currentDim,
					Pos:       cpos,
					Data:      cc,
				}
			}
		case p.ID == packetid.ClientboundBlockEntityData:
		case p.ID == packetid.ClientboundForgetLevelChunk:
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
				loadedDims[dimname] = struct {
					id   int32
					minY int32
				}{
					id:   dimid,
					minY: miny,
				}
			}
		}
	}
}
