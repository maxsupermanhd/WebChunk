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
	"errors"
	"fmt"
	"image"
	"log"
	"os"

	"github.com/Tnze/go-mc/bot"
	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	"github.com/Tnze/go-mc/level"
	"github.com/Tnze/go-mc/net"
	"github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/server"
	"github.com/google/uuid"
	"github.com/maxsupermanhd/WebChunk/credentials"
)

type ProxiedChunk struct {
	FromPlayer string
	FromServer string
	Pos        level.ChunkPos
	Data       level.Chunk
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

func RunProxy(routeHandler func(name string) string, conf *ProxyConfig, dump chan ProxiedChunk) {
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
			ModifyClientboundPacket: func(username, address string, p *packet.Packet) error {
				if p == nil {
					return nil
				}
				if p.ID == packetid.ClientboundLevelChunkWithLight {
					var pos level.ChunkPos
					var chunk level.Chunk
					err := p.Scan(&pos, &chunk)
					if err != nil {
						log.Printf("Error parsing chunk packet: %v", err)
						return err
					}
					dump <- ProxiedChunk{
						FromPlayer: username,
						FromServer: address,
						Pos:        pos,
						Data:       chunk,
					}
				}
				return nil
			},
			ModifyServerboundPacket: nil,
		},
	}
	log.Println("Started proxy on " + conf.Listen)
	log.Println(s.Listen(conf.Listen))
}

type SnifferProxy struct {
	Routing                 func(name string) string
	CredManager             *credentials.MicrosoftCredentialsManager
	ModifyClientboundPacket func(username, address string, p *packet.Packet) error
	ModifyServerboundPacket func(username, address string, p *packet.Packet) error
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
	// forward all packet from server to player
	c.Events.AddGeneric(bot.PacketHandler{
		Priority: 100,
		F: func(pk packet.Packet) error {
			if p.ModifyClientboundPacket != nil {
				err := p.ModifyClientboundPacket(name, dest, &pk)
				if err != nil {
					return err
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
		var disconnectErr *bot.DisconnectErr
		if errors.As(err, &disconnectErr) {
			_ = conn.WritePacket(packet.Marshal(
				packetid.ClientboundDisconnect,
				(*chat.Message)(disconnectErr),
			))
		}
		return
	}
	defer c.Close()
	go func() {
		// forward all packet from player to server
		var pk packet.Packet
		var err error
		for {
			err = conn.ReadPacket(&pk)
			if err != nil {
				break
			}
			if p.ModifyServerboundPacket != nil {
				err = p.ModifyServerboundPacket(name, dest, &pk)
				if err != nil {
					break
				}
			}
			err = c.Conn.WritePacket(pk)
			if err != nil {
				break
			}
		}
		log.Printf("Player [%s] left from server [%s]: %v", name, dest, err)
	}()
	if err := c.HandleGame(); err != nil {
		log.Printf("FPlayer [%s] left from server [%s]: %v", name, dest, err)
	}
}

func dissconnectWithError(conn *net.Conn, reason error) {
	dissconnectWithMessage(conn, &chat.Message{Text: fmt.Sprint(reason)})
}

func dissconnectWithMessage(conn *net.Conn, reason *chat.Message) {
	conn.WritePacket(packet.Marshal(packetid.ClientboundDisconnect, reason))
}
