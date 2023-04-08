package main

import (
	"github.com/Tnze/go-mc/level"
	"github.com/Tnze/go-mc/save"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
)

type chunkWithNeighbours struct {
	topleft, top, topright          *save.Chunk
	left, center, right             *save.Chunk
	bottomleft, bottom, bottomright *save.Chunk
}

func getChunksRegionWithContext(cs chunkStorage.ChunkStorage, wname, dname string, cx0, cz0, cx1, cz1 int) ([]chunkStorage.ChunkData, error) {
	bunch := map[level.ChunkPos]*save.Chunk{}
	
	ret := 
	return ret, nil
}
