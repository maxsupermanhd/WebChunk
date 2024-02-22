package main

import (
	"log"
	"sync"
)

func startBackgroundRoutine(name string, workfn func(<-chan struct{})) func() {
	log.Printf("Starting %s routine", name)
	closechan := make(chan struct{}, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		workfn(closechan)
		wg.Done()
	}()
	return sync.OnceFunc(func() {
		log.Printf("Shutting down %s routine", name)
		closechan <- struct{}{}
		log.Printf("Waiting for routine %s to exit", name)
		wg.Wait()
		log.Printf("Routine %s done", name)
	})
}
