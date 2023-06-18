package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"path"
	"sort"
	"sync"
	"syscall"

	"github.com/Tnze/go-mc/level"
	"github.com/Tnze/go-mc/level/block"
	"github.com/Tnze/go-mc/save"
	"github.com/Tnze/go-mc/save/region"
	"github.com/maxsupermanhd/WebChunk/chunkStorage/filesystemChunkStorage"
)

var (
	fspath = flag.String("path", "../../storage/constspawn/constantiam.net/region/", "Path to region folder")
)

func main() {

	// var c save.Chunk
	// // b, err := os.ReadFile("/home/max/Desktop/chunkNotFull.bin")
	// f, err := os.Open("/home/max/p/0mc/WebChunk/storage/constspawn/constantiam.net/region/r.-1.-1.mca")
	// must(err)
	// r, err := region.Load(f)
	// must(err)
	// d, err := r.ReadSector(0, 0)
	// must(err)
	// must(c.Load(d))
	// genHeightmap(&c)
	// var buf bytes.Buffer
	// must(nbt.NewEncoder(&buf).Encode(c, ""))
	// fmt.Println(hex.Dump(buf.Bytes()))
	// os.Exit(1)

	// var buf bytes.Buffer
	// c := struct {
	// 	FieldA []nbt.RawMessage
	// 	FieldB nbt.RawMessage
	// 	FieldC nbt.RawMessage
	// 	FieldD int8
	// 	FieldF string
	// }{
	// 	FieldA: []nbt.RawMessage{},
	// 	FieldB: nbt.RawMessage{Type: 9, Data: []byte{0, 0, 0, 0, 0}},
	// 	FieldC: nbt.RawMessage{Type: 10, Data: []byte{0}},
	// 	FieldD: 42,
	// 	FieldF: "test",
	// }
	// spew.Dump(c)
	// must(nbt.NewEncoder(&buf).Encode(c, ""))
	// fmt.Println(hex.Dump(buf.Bytes()))
	// must(nbt.Unmarshal(buf.Bytes(), &c))
	// spew.Dump(c)
	// spew.Dump(region.At(-23, -104))
	// os.Exit(0)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		regen(ctx)
		wg.Done()
	}()
	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt, syscall.SIGTERM)
	<-s
	log.Println("got signal, shutting down")
	cancel()
	log.Println("waiting for exit")
	wg.Wait()
	log.Println("bye")
}

func regen(ctx context.Context) {
	d, err := os.ReadDir(*fspath)
	must(err)
	type chunkWithPos struct {
		x, z   int
		rx, rz int
		raw    []byte
		ret    []byte
	}
	taskch := make(chan chunkWithPos, 32*32)
	retch := make(chan chunkWithPos, 32*32)
	const threads = 3
	var wg sync.WaitGroup
	wg.Add(threads)
	for i := 0; i < threads; i++ {
		go func() {
			for t := range taskch {
				var c save.Chunk
				err = c.Load(t.raw)
				if err != nil {
					log.Printf("Failed to load chunk %2d:%2d of region %3d:%3d because %v", t.x, t.z, t.rx, t.rz, err)
					continue
				}
				genHeightmap(&c)
				t.ret, err = c.Data(1)
				if err != nil {
					log.Printf("Failed to marshal chunk %2d:%2d of region %3d:%3d because %v", t.x, t.z, t.rx, t.rz, err)
					continue
				}
				retch <- t
			}
			wg.Done()
		}()
	}
	for fileNum, i := range d {
		if i.IsDir() {
			continue
		}
		var rx, rz int
		if !filesystemChunkStorage.ExtractRegionPath(i.Name(), &rx, &rz) {
			continue
		}
		log.Println("Region", rx, rz, "pogress", fileNum, "/", len(d))
		r, err := region.Open(path.Join(*fspath, i.Name()))
		must(err)
		chunks := 0
		for x := 0; x < 32; x++ {
			for z := 0; z < 32; z++ {
				if !r.ExistSector(x, z) {
					continue
				}
				cd, err := r.ReadSector(x, z)
				if err != nil {
					log.Printf("Failed to read chunk %2d:%2d of region %3d:%3d because %v", x, z, rx, rz, err)
					continue
				}
				chunks++
				taskch <- chunkWithPos{x: x, z: z, rx: rx, rz: rz, raw: cd}
				if errors.Is(ctx.Err(), context.Canceled) {
					log.Println("regen op shutdown")
					r.Close()
					close(taskch)
					wg.Wait()
					return
				}
			}
		}
		log.Println("region read done waiting for heightmaps")
		for chunks > 0 {
			select {
			case <-ctx.Done():
				log.Println("regen op shutdown")
				r.Close()
				close(taskch)
				wg.Wait()
				return
			case t := <-retch:
				chunks--
				if len(t.ret) == 11 {
					log.Printf("Actually failed to marshal chunk %2d:%2d of region %3d:%3d because it is fucking empty", t.x, t.z, rx, rz)
					os.WriteFile("/home/max/Desktop/failedChunkRet.bin", t.ret, 0666)
					os.WriteFile("/home/max/Desktop/failedChunkRaw.bin", t.raw, 0666)
					os.Exit(1)
				}
				err = r.WriteSector(t.x, t.z, t.ret)
				if err != nil {
					log.Printf("Failed to write back chunk %2d:%2d of region %3d:%3d because %v", t.x, t.z, rx, rz, err)
					continue
				}
			}
		}
		r.Close()
		if errors.Is(ctx.Err(), context.Canceled) {
			log.Println("regen op shutdown")
			r.Close()
			close(taskch)
			wg.Wait()
			return
		}
	}
	log.Println("regen done, closing workeres")
	close(taskch)
	wg.Wait()
}

func genHeightmap(chunk *save.Chunk) {
	sort.Slice(chunk.Sections, func(i, j int) bool {
		return int8(chunk.Sections[i].Y) > int8(chunk.Sections[j].Y)
	})
	var set [16 * 16]bool
	ws := level.NewBitStorage(9, 16*16, nil)
	for _, s := range chunk.Sections {
		if len(s.BlockStates.Data) == 0 {
			continue
		}
		states := prepareSectionBlockstates(&s)
		if states == nil {
			log.Printf("Chunk %d:%d section %d has broken pallete", chunk.XPos, chunk.YPos, s.Y)
			return
		}
		for y := 15; y >= 0; y-- {
			for i := 16*16 - 1; i >= 0; i-- {
				if set[i] {
					continue
				}
				state := states.Get(y*16*16 + i)
				if !isAirState(state) {
					ws.Set(i, int(s.Y)*16+y+64)
					set[i] = true
				}
			}
		}
	}
	if chunk.Heightmaps == nil {
		chunk.Heightmaps = map[string][]uint64{}
	}
	chunk.Heightmaps["WORLD_SURFACE"] = ws.Raw()
}

func isAirState(s block.StateID) bool {
	switch block.StateList[s].(type) {
	case block.Air, block.CaveAir, block.VoidAir:
		return true
	default:
		return false
	}
}

func prepareSectionBlockstates(s *save.Section) *level.PaletteContainer[block.StateID] {
	statePalette := s.BlockStates.Palette
	stateRawPalette := make([]block.StateID, len(statePalette))
	for i, v := range statePalette {
		b, ok := block.FromID[v.Name]
		if !ok {
			b, ok = block.FromID["minecraft:"+v.Name]
			if !ok {
				log.Printf("Can not find block from id [%v]", v.Name)
				return nil
			}
		}
		if v.Properties.Data != nil {
			err := v.Properties.Unmarshal(&b)
			if err != nil {
				log.Printf("Error unmarshling properties of block [%v] from [%v]: %v", v.Name, v.Properties.String(), err.Error())
				return nil
			}
		}
		s := block.ToStateID[b]
		stateRawPalette[i] = s
	}
	return level.NewStatesPaletteContainerWithData(16*16*16, s.BlockStates.Data, stateRawPalette)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
