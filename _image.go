package main

import "context"

var (
	imageScaleProcess = make(chan imageTask, 32)
)

func imageScaleWorker() {
	
}

func imageScaleProcessor(ctx context.Context) {
	select {
	case <-ctx.Done():

	case task := <-imageCacheProcess:

	}
}
