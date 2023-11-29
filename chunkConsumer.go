package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/maxsupermanhd/WebChunk/chunkStorage"
	"github.com/maxsupermanhd/WebChunk/proxy"
	"github.com/maxsupermanhd/go-vmc/v764/level"
	"github.com/maxsupermanhd/go-vmc/v764/nbt"
	"github.com/maxsupermanhd/go-vmc/v764/save"
)

func chunkConsumer(ctx context.Context, c chan *proxy.ProxiedChunk) {
	for {
		select {
		case <-ctx.Done():
			return
		case r := <-c:
			if r.Dimension == "" || r.Server == "" {
				log.Printf("Got chunk [%v](%v) from [%v] by [%v] with empty params, DROPPING", r.Pos, r.Dimension, r.Server, r.Username)
				continue
			}
			log.Printf("Got chunk %v %#v from [%v] by [%v] (%2d s) (%3d be)", r.Pos, r.Dimension, r.Server, r.Username, len(r.Data.Sections), len(r.Data.BlockEntity))
			r.Dimension = strings.TrimPrefix(r.Dimension, "minecraft:")
			w, s, err := chunkStorage.GetWorldStorage(storages, r.Server)
			if err != nil {
				log.Println("Failed to lookup world storage: ", err)
				break
			}
			var d *chunkStorage.SDim
			if w == nil || s == nil {
				pref := cfg.GetDSString("", "preferred_storage")
				s = findCapableStorage(storages, pref)
				if s == nil {
					log.Printf("Failed to find storage that has world [%s], named [%s] or has ability to add chunks, chunk [%v] from [%v] by [%v] is LOST.", r.Server, pref, r.Pos, r.Server, r.Username)
					continue
				}
				w = &chunkStorage.SWorld{
					Name:       r.Server,
					Alias:      "",
					IP:         r.Server,
					CreatedAt:  time.Now(),
					ModifiedAt: time.Now(),
					Data:       chunkStorage.CreateDefaultLevelData(r.Server),
				}
				err = s.AddWorld(*w)
				if err != nil {
					log.Printf("Failed to add world: %s", err.Error())
					continue
				}
			}
			d, err = s.GetDimension(w.Name, r.Dimension)
			if err != nil && !errors.Is(err, chunkStorage.ErrNoDim) {
				log.Printf("Failed to get dim: %s", err.Error())
				continue
			}
			if d == nil {
				d = &chunkStorage.SDim{
					Name:       r.Dimension,
					World:      w.Name,
					CreatedAt:  time.Now(),
					ModifiedAt: time.Now(),
					Data:       chunkStorage.GuessDimTypeFromName(r.Dimension),
				}
				err = s.AddDimension(w.Name, *d)
				if err != nil {
					log.Printf("Failed to add dim: %s", err.Error())
					continue
				}
			}
			if d == nil {
				log.Println("d is nill")
				continue
			}
			if w == nil {
				log.Println("w is nill")
				continue
			}
			if d.World != w.Name {
				log.Printf("SUS dim's wname != world's name [%s] [%s]", d.World, w.Name)
				continue
			}
			nbtEmptyList := nbt.RawMessage{
				Type: nbt.TagList,
				Data: []byte{
					nbt.TagEnd, // type
					0, 0, 0, 0, // length (4 bytes)
				},
			}
			nbtEmptyCompound := nbt.RawMessage{
				Type: nbt.TagCompound,
				Data: []byte{0}, // tag end
			}
			gethm := func(hm *level.BitStorage) []uint64 {
				if hm != nil {
					return hm.Raw()
				}
				return nil
			}
			var data save.Chunk
			data = save.Chunk{
				DataVersion:   3120,
				XPos:          r.Pos[0],
				YPos:          r.DimensionLowestY / 16,
				ZPos:          r.Pos[1],
				BlockEntities: []nbt.RawMessage{},
				Structures:    nbtEmptyCompound,
				Heightmaps: map[string][]uint64{
					"MOTION_BLOCKING":           gethm(r.Data.HeightMaps.MotionBlocking),
					"MOTION_BLOCKING_NO_LEAVES": gethm(r.Data.HeightMaps.MotionBlockingNoLeaves),
					"OCEAN_FLOOR":               gethm(r.Data.HeightMaps.OceanFloor),
					"WORLD_SURFACE":             gethm(r.Data.HeightMaps.WorldSurface),
				},
				Sections:       []save.Section{},
				BlockTicks:     nbtEmptyList,
				FluidTicks:     nbtEmptyList,
				PostProcessing: nbtEmptyList,
				InhabitedTime:  0,
				IsLightOn:      0,
				LastUpdate:     time.Now().Unix(),
				Status:         "proxied",
			}
			if r.DimensionLowestY%16 > 0 {
				data.YPos++
			}
			level.ChunkToSave(&r.Data, &data)

			var chunkBytes bytes.Buffer
			chunkBytes.WriteByte(1) // compression type
			chunkBytesWriter := gzip.NewWriter(&chunkBytes)
			err = nbt.NewEncoder(chunkBytesWriter).Encode(data, "")
			if err != nil {
				log.Printf("Failed to marshal chunk: %s", err.Error())
				continue
			}
			err = chunkBytesWriter.Close()
			if err != nil {
				log.Printf("Failed to flush chunk buffer: %s", err.Error())
				continue
			}
			err = s.AddChunkRaw(w.Name, d.Name, int(r.Pos[0]), int(r.Pos[1]), chunkBytes.Bytes())
			if err != nil {
				log.Printf("Failed to save chunk: %s", err.Error())
			}
			if cfg.GetDSBool(true, "render_received") {
				go func() {
					i := drawChunk(&data)
					imageCacheSave(i, w.Name, d.Name, "terrain", 0, int(r.Pos[0]), int(r.Pos[1]))
				}()
			}
		}
	}
}
