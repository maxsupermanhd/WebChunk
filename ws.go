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

package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	imagecache "github.com/maxsupermanhd/WebChunk/imageCache"
	"github.com/mitchellh/mapstructure"
)

var (
	wsUpgrader = websocket.Upgrader{
		HandshakeTimeout: 2 * time.Second,
		ReadBufferSize:   0,
		WriteBufferSize:  0,
		WriteBufferPool:  nil,
		Subprotocols:     nil,
		Error: func(_ http.ResponseWriter, r *http.Request, status int, reason error) {
			log.Printf("Websocket error from client %v: %v %v", r.RemoteAddr, status, reason.Error())
		},
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		EnableCompression: true,
	}
	wsClients sync.WaitGroup
)

type wsmessage struct {
	msgType int
	msgData []byte
}

func wsClientHandlerWrapper(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wsClientHandler(w, r, ctx)
	}
}

func wsClientHandler(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	c, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Websocket upgrade error: %s", err)
		return
	}
	wsClients.Add(1)
	defer wsClients.Done()
	defer c.Close()

	log.Printf("Websocket %s connected", r.RemoteAddr)

	pingTicker := time.NewTicker(2 * time.Second)

	e := globalEventRouter.Connect()
	defer globalEventRouter.Disconnect(e)

	e <- mapEvent{
		Action: "updateLayers",
		Data:   listttypes(),
	}
	e <- mapEvent{
		Action: "updateWorldsAndDims",
		Data:   listNamesWnD(),
	}

	eQ := make(chan error, 2)
	wQ := make(chan wsmessage, 32)
	var wQdidClose atomic.Bool
	rQ := make(chan wsmessage, 32)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		asyncWriter(c, wQ, eQ)
		wg.Done()
	}()
	go func() {
		go asyncReader(c, rQ, eQ)
		wg.Done()
	}()

	subbedTiles := map[imagecache.ImageLocation]bool{}

	asyncTileRequestor := func(loc imagecache.ImageLocation) {
		ret := marshalBinaryTileUpdate(loc, imageCacheGetBlockingLoc(loc))
		if !wQdidClose.Load() {
			wQ <- wsmessage{
				msgType: websocket.BinaryMessage,
				msgData: ret,
			}
		}
	}

clientLoop:
	for {
		select {
		case t := <-pingTicker.C:
			wQ <- wsmessage{
				msgType: websocket.PingMessage,
				msgData: []byte(fmt.Sprint(t.Unix())),
			}
		case m := <-e:
			log.Printf("Websocket %s relaying message %#+v", r.RemoteAddr, m.Action)
			b, err := json.Marshal(m)
			if err != nil {
				log.Printf("Failed to marshal progress: %v\n", err)
				break clientLoop
			}
			wQ <- wsmessage{
				msgType: websocket.TextMessage,
				msgData: b,
			}
		case err := <-eQ:
			wQdidClose.Store(true)
			close(wQ)
			c.Close()
			if err == nil {
				log.Printf("Websocket %s disconnected with nil err", r.RemoteAddr)
				break clientLoop
			} else {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseMessage) {
					log.Printf("Websocket %s error: %s", r.RemoteAddr, err)
				} else {
					log.Printf("Websocket %s disconnected", r.RemoteAddr)
				}
			}
			break clientLoop
		case <-ctx.Done():
			log.Printf("Shutting down websocket %s", r.RemoteAddr)
			wQ <- wsmessage{
				msgType: websocket.TextMessage,
				msgData: []byte(`{"Action": "message", "Data": "Disconnecting because WebChunk server is shutting down"}`),
			}
			wQ <- wsmessage{
				msgType: websocket.CloseMessage,
				msgData: []byte{},
			}
			break clientLoop
		case m := <-rQ:
			if m.msgType == websocket.TextMessage {
				var msg mapEvent
				err := json.Unmarshal(m.msgData, &msg)
				if err != nil {
					log.Printf("Failed to decode websocket client %s message: %s", r.RemoteAddr, err.Error())
				}
				switch msg.Action {
				case "tileSubscribe":
					var loc imagecache.ImageLocation
					err := mapstructure.Decode(msg.Data, &loc)
					if err != nil {
						log.Printf("Websocket %s sent malformed tile sub: %s", r.RemoteAddr, err.Error())
						break
					}
					_, ok := subbedTiles[loc]
					if ok {
						log.Printf("Websocket %s tileSub already subbed %s", r.RemoteAddr, loc)
					} else {
						subbedTiles[loc] = true
						log.Printf("Websocket %s tileSub %s", r.RemoteAddr, loc)
					}
					go asyncTileRequestor(loc)
				case "tileUnsubscribe":
					var loc imagecache.ImageLocation
					err := mapstructure.Decode(msg.Data, &loc)
					if err != nil {
						log.Printf("Websocket %s sent malformed tile unsub: %s", r.RemoteAddr, err.Error())
						break
					}
					_, ok := subbedTiles[loc]
					if ok {
						delete(subbedTiles, loc)
						log.Printf("Websocket %s tileUnsub %s", r.RemoteAddr, loc)
					} else {
						log.Printf("Websocket %s tileUnsub does not exist %s", r.RemoteAddr, loc)
					}
				case "resubWorldDimension":
					data, ok := msg.Data.(map[string]any)
					if !ok {
						log.Printf("Websocket %s sent malformed tile unsub: data not map", r.RemoteAddr)
						break
					}
					nWorld, ok := data["World"].(string)
					if !ok {
						log.Printf("Websocket %s sent malformed tile unsub: failed to read World name", r.RemoteAddr)
						break
					}
					nDimension, ok := data["Dimension"].(string)
					if !ok {
						log.Printf("Websocket %s sent malformed tile unsub: failed to read Dimension name", r.RemoteAddr)
						break
					}
					oldSubbed := subbedTiles
					subbedTiles = map[imagecache.ImageLocation]bool{}
					for k := range oldSubbed {
						k.World = nWorld
						k.Dimension = nDimension
						subbedTiles[k] = true
						go asyncTileRequestor(k)
					}
				default:
					log.Printf("Websocket %s wrong action %#+v", r.RemoteAddr, msg.Action)
				}
			} else {
				log.Printf("Websocket %s message %d len %d", r.RemoteAddr, m.msgType, len(m.msgData))
			}
		}
	}
	log.Printf("Websocket %s loop exited", r.RemoteAddr)

	wg.Wait()
	log.Printf("Websocket handler %s exited", r.RemoteAddr)
}

func marshalBinaryTileUpdate(loc imagecache.ImageLocation, img *image.RGBA) []byte {
	buf := bytes.NewBuffer([]byte{})
	binary.Write(buf, binary.BigEndian, uint8(0x01))
	binary.Write(buf, binary.BigEndian, uint32(len(loc.World)))
	buf.WriteString(loc.World)
	binary.Write(buf, binary.BigEndian, uint32(len(loc.Dimension)))
	buf.WriteString(loc.Dimension)
	binary.Write(buf, binary.BigEndian, uint32(len(loc.Variant)))
	buf.WriteString(loc.Variant)
	binary.Write(buf, binary.BigEndian, uint8(loc.S))
	binary.Write(buf, binary.BigEndian, int32(loc.X))
	binary.Write(buf, binary.BigEndian, int32(loc.Z))
	if img != nil {
		png.Encode(buf, img)
	}
	return buf.Bytes()
}

func asyncWriter(c *websocket.Conn, q chan wsmessage, e chan error) {
	for m := range q {
		err := c.WriteMessage(m.msgType, m.msgData)
		if err != nil {
			e <- err
			return
		}
		if m.msgType == websocket.CloseMessage {
			c.Close()
			return
		}
	}
}

func asyncReader(c *websocket.Conn, q chan wsmessage, e chan error) {
	for {
		msgt, msgd, err := c.ReadMessage()
		if err != nil {
			e <- err
			return
		}
		if msgt == websocket.PongMessage || msgt == websocket.PingMessage {
			continue
		}
		if msgt == websocket.CloseMessage {
			e <- nil
			return
		}
		q <- wsmessage{
			msgType: msgt,
			msgData: msgd,
		}
	}
}
