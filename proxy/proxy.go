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
	"strings"
	"time"

	"github.com/Tnze/go-mc/bot"
	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	"github.com/Tnze/go-mc/level"
	mcnet "github.com/Tnze/go-mc/net"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/server"
	"github.com/google/uuid"
	"github.com/maxsupermanhd/WebChunk/credentials"
)

type ProxiedPacket struct {
	Username string
	Server   string
	Packet   pk.Packet
}

type ProxiedChunk struct {
	Username            string
	Server              string
	Dimension           string
	DimensionLowestY    int32
	DimensionBuildLimit int
	Pos                 level.ChunkPos
	Data                level.Chunk
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
			Conf:        *conf,
		},
	}
	log.Println("Started proxy on " + conf.Listen)
	log.Println(s.Listen(conf.Listen))
}

type SnifferProxy struct {
	Routing     func(name string) string
	CredManager *credentials.MicrosoftCredentialsManager
	SaveChannel chan *ProxiedChunk
	Conf        ProxyConfig
}

func (p SnifferProxy) AcceptPlayer(name string, id uuid.UUID, _ int32, conn *mcnet.Conn) {
	log.Printf("Accepting new player [%s] (%s), getting route...", name, id.String())
	dest := p.Routing(name)
	if dest == "" {
		log.Printf("Unable to find route for [%s]", name)
		dissconnectWithMessage(conn, &chat.Message{Text: "Dissconnected before login: Routing failed"})
		return
	}
	log.Printf("Accepting new player [%s] (%s), adding events...", name, id.String())
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
	c := bot.NewClient()
	c.Auth = *auth
	log.Printf("Accepting new player [%s] (%s), dialing [%s]...", name, id.String(), dest)
	if err := c.JoinServer(dest); err != nil {
		log.Printf("Failed to accept new player [%s] (%s), error connecting to [%s]: %v", name, id.String(), dest, err)
		dissconnectWithMessage(conn, &chat.Message{Text: strings.TrimPrefix(err.Error(), "bot: disconnect error: disconnect because: ")})
		return
	}
	log.Printf("Player [%s] accepted to [%s]", name, dest)

	acceptorChannel := make(chan pk.Packet, 2048)
	defer close(acceptorChannel)
	connQueue := server.NewPacketQueue()
	go packetAcceptor(acceptorChannel, connQueue, p.SaveChannel, name, dest, p.Conf)

	closeChannel := make(chan byte)
	defer close(closeChannel)

	go func() {
		<-closeChannel
		conn.Socket.SetDeadline(time.UnixMilli(0))
		c.Conn.Socket.SetDeadline(time.UnixMilli(0))
		connQueue.Close()
	}()

	go func() {
		var pk pk.Packet
		var err error
		for {
			err = conn.ReadPacket(&pk)
			if err != nil {
				break
			}
			// log.Printf("c->s (pump) %x", pk.ID)
			err = c.Conn.WritePacket(pk)
			if err != nil {
				break
			}
		}
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			log.Printf("Player [%s] left from server [%s] (s->c): %v", name, dest, err)
			closeChannel <- 0
		}
	}()

	go func() {
		var pack pk.Packet
		var err error
		for {
			err = c.Conn.ReadPacket(&pack)
			if err != nil {
				break
			}
			topack := pk.Packet{
				ID:   pack.ID,
				Data: make([]byte, len(pack.Data)),
			}
			copy(topack.Data, pack.Data)
			for i := 0; i < len(collectPackets); i++ {
				if collectPackets[i] == int(pack.ID) {
					acceptorChannel <- topack
					break
				}
			}
			// log.Printf("s->c (queuePush) %x", pack.ID)
			connQueue.Push(topack)
		}
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			log.Printf("Player [%s] left from server [%s] (c->s): %v", name, dest, err)
			closeChannel <- 0
		}
	}()

	var pack pk.Packet
	var ok bool
	for {
		pack, ok = connQueue.Pull()
		if !ok {
			break
		}
		// log.Printf("s->c (queuePull) %x", pack.ID)
		err = conn.WritePacket(pack)
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				break
			} else {
				closeChannel <- 0
			}
		}
	}
	// connQueue.Close()
}

func dissconnectWithError(conn *mcnet.Conn, reason error) {
	dissconnectWithMessage(conn, &chat.Message{Text: fmt.Sprint(reason)})
}

func dissconnectWithMessage(conn *mcnet.Conn, reason *chat.Message) {
	conn.WritePacket(pk.Marshal(packetid.ClientboundDisconnect, reason))
}
