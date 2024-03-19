package main

import (
	"log"
)

type mapEvent struct {
	Action string
	Data   any
}

type mapEventRouter struct {
	connect    chan chan mapEvent
	disconnect chan chan mapEvent
	events     chan mapEvent
}

var (
	globalEventRouter = newMapEventRouter()
)

func newMapEventRouter() *mapEventRouter {
	return &mapEventRouter{
		connect:    make(chan chan mapEvent, 16),
		disconnect: make(chan chan mapEvent, 16),
		events:     make(chan mapEvent, 256),
	}
}

func (router *mapEventRouter) Run(exitchan <-chan struct{}) {
	clients := map[chan mapEvent]bool{}
	for {
		select {
		case <-exitchan:
			for c := range clients {
				close(c)
			}
			return
		case c := <-router.connect:
			clients[c] = true
		case c := <-router.disconnect:
			delete(clients, c)
			close(c)
		case e := <-router.events:
			for c := range clients {
				select {
				case c <- e:
				default:
					log.Printf("Event %v dropped!", e.Action)
				}
			}
		}
	}
}

func (router *mapEventRouter) Connect() chan mapEvent {
	c := make(chan mapEvent, 256)
	router.connect <- c
	return c
}

func (router *mapEventRouter) Disconnect(c chan mapEvent) {
	router.disconnect <- c
}

func (router *mapEventRouter) Broadcast(e mapEvent) {
	router.events <- e
}
