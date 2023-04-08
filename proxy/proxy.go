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
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Tnze/go-mc/bot"
	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/chat/sign"
	"github.com/Tnze/go-mc/data/packetid"
	"github.com/Tnze/go-mc/level"
	"github.com/Tnze/go-mc/net"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/server"
	"github.com/Tnze/go-mc/server/auth"
	"github.com/google/uuid"
	"github.com/maxsupermanhd/WebChunk/credentials"
	"github.com/maxsupermanhd/lac"
)

type ProxyRoute struct {
	Address   string `json:"address"`
	World     string `json:"world"`
	Dimension string `json:"dimension"`
	Storage   string `json:"storage"`
}

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

var collectPackets = []packetid.ClientboundPacketID{
	packetid.ClientboundLevelChunkWithLight,
	packetid.ClientboundBlockEntityData,
	packetid.ClientboundForgetLevelChunk,
	packetid.ClientboundLogin,
	packetid.ClientboundRespawn,
}

func RunProxy(ctx context.Context, cfg *lac.ConfSubtree, dump chan *ProxiedChunk) {
	var icon image.Image
	if iconpath := cfg.GetDSString("", "icon_path"); iconpath != "" {
		f, err := os.Open(iconpath)
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
	playerList := server.NewPlayerList(cfg.GetDSInt(999, "max_players"))
	var motd chat.Message
	if err := cfg.GetToStruct(&motd, "motd"); err != nil {
		log.Println("Using default MOTD because failed to parse one from config: ", err.Error())
		motd = chat.Text("WebChunk proxy")
		cfg.Set(map[string]any{"text": "WebChunk proxy"}, "motd")
	}
	serverInfo := server.NewPingInfo(server.ProtocolName, server.ProtocolVersion, motd, icon)
	s := server.Server{
		ListPingHandler: struct {
			*server.PlayerList
			*server.PingInfo
		}{playerList, serverInfo},
		LoginHandler: &server.MojangLoginHandler{
			OnlineMode:   cfg.GetDSBool(true, "online_mode"),
			Threshold:    cfg.GetDSInt(-1, "compress_threshold"),
			LoginChecker: nil,
		},
		GamePlay: SnifferProxy{
			Routing: func(name string) string {
				r, _ := cfg.GetString("routes", name)
				return r
			},
			CredManager: credentials.NewMicrosoftCredentialsManager(cfg.GetDSString("./cmd/auth/", "credentials_path"), "88650e7e-efee-4857-b9a9-cf580a00ef43"),
			SaveChannel: dump,
			Conf:        cfg,
			Ctx:         ctx,
		},
	}
	listener, err := net.ListenMC(cfg.GetDSString("localhost:25566", "listen_addr"))
	if err != nil {
		log.Println("Proxy startup error: ", err)
		return
	}
	log.Println("Proxy started on " + cfg.GetDSString("localhost:25566", "listen_addr"))
	var wg sync.WaitGroup
	lstCloseChan := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-lstCloseChan:
					return
				default:
					log.Println("Proxy listener error: ", err)
				}
			} else {
				wg.Add(1)
				go func() {
					s.AcceptConn(&conn)
					wg.Done()
				}()
			}
		}
	}()
	<-ctx.Done()
	close(lstCloseChan)
	err = listener.Close()
	if err != nil {
		log.Println("Proxy listener close error: ", err)
	}
	wg.Wait()
}

type SnifferProxy struct {
	Routing     func(name string) string
	CredManager *credentials.MicrosoftCredentialsManager
	SaveChannel chan *ProxiedChunk
	Conf        *lac.ConfSubtree
	Ctx         context.Context
}

type clientinfo struct {
	name          string
	id            uuid.UUID
	profilePubKey *auth.PublicKey
	properties    []auth.Property
	proto         int32
	conn          *net.Conn
	dest          string
}

func (p SnifferProxy) AcceptPlayer(name string, id uuid.UUID, profilePubKey *auth.PublicKey, properties []auth.Property, proto int32, conn *net.Conn) {
	dest := p.Routing(name)
	cl := clientinfo{
		name:          name,
		id:            id,
		profilePubKey: profilePubKey,
		properties:    properties,
		proto:         proto,
		conn:          conn,
		dest:          dest,
	}
	if cl.dest == "" {
		log.Printf("Accepting new player [%s] (%s), protocol %v, unable to find route...", cl.name, cl.id.String(), cl.proto)
		dissconnectWithMessage(conn, &chat.Message{Text: "Dissconnected before login: no defined route for specified username"})
		return
	}
	log.Printf("Accepting new player [%s] (%s), protocol %v, routing to [%s], getting auth...", cl.name, cl.id.String(), cl.proto, dest)
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
	if err := c.JoinServerWithOptions(dest, bot.JoinOptions{
		Dialer:      nil,
		Context:     nil,
		NoPublicKey: true,
		KeyPair:     nil,
	}); err != nil {
		log.Printf("Failed to accept new player [%s] (%s), error connecting to [%s]: %v", name, id.String(), dest, err)
		dissconnectWithMessage(conn, &chat.Message{Text: strings.TrimPrefix(err.Error(), "bot: disconnect error: disconnect because: ")})
		return
	}
	log.Printf("Player [%s] accepted to [%s]", name, dest)

	var wg sync.WaitGroup

	acceptorChannel := make(chan pk.Packet, 2048)
	connQueue := server.NewPacketQueue()
	wg.Add(1)
	go func() {
		p.packetAcceptor(acceptorChannel, connQueue, cl)
		wg.Done()
	}()

	closeChannel := make(chan byte, 5)
	defer close(closeChannel)

	wg.Add(1)
	go func() {
		select {
		case <-closeChannel:
		case <-p.Ctx.Done():
		}
		conn.Socket.SetDeadline(time.UnixMilli(0))
		c.Conn.Socket.SetDeadline(time.UnixMilli(0))
		connQueue.Close()
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		var p pk.Packet
		var err error
		for {
			err = conn.ReadPacket(&p)
			if err != nil {
				break
			}
			// log.Printf("c->s (pump) %x", pk.ID)
			if p.ID == int32(packetid.ServerboundChat) {
				var (
					msg pk.String
				)
				err := p.Scan(
					&msg,
				)
				if err != nil {
					log.Println("Error scanning message:", err)
				}
				sendout := pk.Marshal(
					packetid.ServerboundChat,
					pk.String(msg),
					pk.Long(time.Now().UnixMilli()),
					pk.Long(rand.Int63()),
					pk.ByteArray{},
					pk.Boolean(false),
					pk.Array([]sign.HistoryMessage{}),
					pk.Option[sign.HistoryMessage, *sign.HistoryMessage]{
						Has: false,
					},
				)
				err = c.Conn.WritePacket(sendout)
				if err != nil {
					log.Println("Failed to unmarshal packet:", err)
				}
			} else {
				err = c.Conn.WritePacket(p)
				if err != nil {
					break
				}
			}
		}
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			log.Printf("Player [%s] left from server [%s] (s->c): %v", name, dest, err)
			closeChannel <- 0
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		var err error
		for {
			var pack pk.Packet
			err = c.Conn.ReadPacket(&pack)
			if err != nil {
				break
			}
			// topack := pk.Packet{
			// 	ID:   pack.ID,
			// 	Data: make([]byte, len(pack.Data)),
			// }
			// copy(topack.Data, pack.Data)
			for i := 0; i < len(collectPackets); i++ {
				if collectPackets[i] == packetid.ClientboundPacketID(pack.ID) {
					acceptorChannel <- pack
					break
				}
			}
			// log.Printf("s->c (queuePush) %x", pack.ID)
			connQueue.Push(pack)
		}
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			log.Printf("Player [%s] left from server [%s] (c->s): %v", name, dest, err)
			closeChannel <- 0
		}
		wg.Done()
		close(acceptorChannel)
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
	wg.Wait()
	// connQueue.Close()
}

func dissconnectWithError(conn *net.Conn, reason error) {
	dissconnectWithMessage(conn, &chat.Message{Text: fmt.Sprint(reason)})
}

func dissconnectWithMessage(conn *net.Conn, reason *chat.Message) {
	conn.WritePacket(pk.Marshal(packetid.ClientboundDisconnect, reason))
}
