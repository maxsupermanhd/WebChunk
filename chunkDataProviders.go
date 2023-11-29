package main

import (
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
	"github.com/maxsupermanhd/go-vmc/v764/save"
)

type ContextedChunkData struct {
	center                   *save.Chunk
	top, bottom, left, right *save.Chunk
}

func getChunksRegionWithContextFN(cs chunkStorage.ChunkStorage) chunkDataProviderFunc {
	return func(wname, dname string, cx0, cz0, cx1, cz1 int) ([]chunkStorage.ChunkData, error) {
		return getChunksRegionWithContext(cs, wname, dname, cx0, cz0, cx1, cz1)
	}
}

func getChunksRegionWithContext(cs chunkStorage.ChunkStorage, wname, dname string, cx0, cz0, cx1, cz1 int) ([]chunkStorage.ChunkData, error) {
	type chunkpos struct {
		X, Z int
	}
	bunch := map[chunkpos]*save.Chunk{}
	unsortedBunch, err := cs.GetChunksRegion(wname, dname, cx0-1, cz0-1, cx1+1, cz1+1)
	if err != nil {
		return []chunkStorage.ChunkData{}, err
	}
	for _, v := range unsortedBunch {
		c, ok := v.Data.(save.Chunk)
		if !ok {
			continue
		}
		bunch[chunkpos{v.X, v.Z}] = &c
	}
	ret := []chunkStorage.ChunkData{}
	for k, v := range bunch {
		if k.X < cx0 || k.X >= cx1 || k.Z < cz0 || k.Z >= cz1 {
			continue
		}
		ret = append(ret, chunkStorage.ChunkData{
			X: k.X,
			Z: k.Z,
			Data: ContextedChunkData{
				center: v,
				top:    bunch[chunkpos{X: k.X, Z: k.Z - 1}],
				bottom: bunch[chunkpos{X: k.X, Z: k.Z + 1}],
				left:   bunch[chunkpos{X: k.X - 1, Z: k.Z}],
				right:  bunch[chunkpos{X: k.X + 1, Z: k.Z}],
			},
		})
	}
	return ret, nil
}
