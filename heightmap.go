package main

import (
	"log"
	"sort"

	"github.com/maxsupermanhd/go-vmc/v762/save"
)

func genHeightmap(chunk *save.Chunk) []int {
	// TODO: this is a crutch, should be using MOTION_BLOCKING or WORLD_SURFACE heightmap from server if available
	sort.Slice(chunk.Sections, func(i, j int) bool {
		return int8(chunk.Sections[i].Y) > int8(chunk.Sections[j].Y)
	})
	var height [16 * 16]int
	var set [16 * 16]bool
	for _, s := range chunk.Sections {
		if len(s.BlockStates.Data) == 0 {
			continue
		}
		states := prepareSectionBlockstates(&s)
		if states == nil {
			log.Printf("Chunk %d:%d section %d has broken pallete", chunk.XPos, chunk.YPos, s.Y)
			continue
		}
		for y := 15; y >= 0; y-- {
			for i := 16*16 - 1; i >= 0; i-- {
				if set[i] {
					continue
				}
				state := states.Get(y*16*16 + i)
				if !isAirState(state) {
					height[i] = int(s.Y)*16 + y
					set[i] = true
				}
			}
		}
	}
	return height[:]
}
