package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var tasksProgressBroadcaster = NewBroadcaster()

func WSstatusHandler(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		HandshakeTimeout: 2 * time.Second,
		ReadBufferSize:   0,
		WriteBufferSize:  0,
		WriteBufferPool:  nil,
		Subprotocols:     nil,
		Error: func(w http.ResponseWriter, r *http.Request, status int, reason error) {
			log.Printf("Websocket error: %v %v", status, reason.Error())
		},
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		EnableCompression: true,
	}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("Websocket upgrade error:", err)
		return
	}
	defer c.Close()
	errChan := make(chan error)
	go func() {
		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
		}
	}()
	msgc := tasksProgressBroadcaster.Subscribe()
	defer tasksProgressBroadcaster.Unsubscribe(msgc)
	for {
		select {
		case m := <-msgc:
			b, err := json.Marshal(m)
			if err != nil {
				log.Printf("Failed to marshal progress: %v\n", err)
				return
			}
			err = c.WriteMessage(websocket.TextMessage, b)
			if err != nil {
				return
			}
		case <-errChan:
			return
		}
	}
}
