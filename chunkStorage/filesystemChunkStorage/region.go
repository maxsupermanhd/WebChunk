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

package filesystemChunkStorage

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Tnze/go-mc/save"
	"github.com/Tnze/go-mc/save/region"
	"github.com/hashicorp/go-multierror"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
)

type regionLocator struct {
	world     string
	dimension string
	rx, rz    int
}

type regionRequest struct {
	op                 string
	world              string
	dimension          string
	cx1, cx2, cz1, cz2 int // 1 top left 2 bottom right
	data               []byte
	result             chan interface{}
}

// region router will recieve requests for operations
// and start a gorutine worker for each different
// region file and route requests to them
func (s *FilesystemChunkStorage) regionRouter() {
	log.Println("Region router started for storage", s.Root)
	defer s.wg.Done()
	type regionInterface struct {
		exists      bool
		c           chan regionRequest
		lastRequest time.Time
	}
	w := map[regionLocator]regionInterface{}
	autocloseTicker := time.NewTicker(30 * time.Second)
	closeTicker := make(chan struct{})
	go func() {
		for {
			select {
			case <-autocloseTicker.C:
				s.requests <- regionRequest{
					op: "closeInactive",
				}
			case <-closeTicker:
				return
			}
		}
	}()
	getOrCreateWorker := func(world, dimension string, rx, rz int, r regionRequest) {
		l := regionLocator{
			world:     world,
			dimension: dimension,
			rx:        rx,
			rz:        rz,
		}
		c, ok := w[l]
		if !ok {
			_, err := os.Stat(s.getRegionPath(l))
			c = regionInterface{
				exists:      true,
				c:           make(chan regionRequest, 32),
				lastRequest: time.Now(),
			}
			if os.IsNotExist(err) {
				if r.op == "set" {
					s.wg.Add(1)
					go func() {
						s.regionWorker(l, c.c, true)
						s.wg.Done()
					}()
				} else {
					c.exists = false
					close(c.c)
				}
			} else {
				s.wg.Add(1)
				go func() {
					s.regionWorker(l, c.c, false)
					s.wg.Done()
				}()
			}
		}
		if !c.exists {
			r.result <- nil
		} else {
			deadlockTimer := time.After(5 * time.Second)
			select {
			case c.c <- r:
			case <-deadlockTimer:
				log.Println("Deadlock on region", l)
			}
		}
		c.lastRequest = time.Now()
		w[l] = c
	}
	for r := range s.requests {
		l := regionLocator{
			world:     r.world,
			dimension: r.dimension,
		}
		// log.Println("Region router", s.Root, r.op, r.cx1, r.cz1, r.cx2, r.cz2)
		switch r.op {
		case "closeInactive":
			toclose := []regionLocator{}
			for k, v := range w {
				if time.Since(v.lastRequest) > 1*time.Minute {
					toclose = append(toclose, k)
				}
			}
			for _, k := range toclose {
				v, ok := w[k]
				if !ok {
					log.Printf("Region auto-close found a ghost! (%#v)", k)
				} else {
					if v.exists {
						close(v.c)
					}
					delete(w, k)
				}
			}
		case "regionClose":
			l.rx, l.rz = r.cx1, r.cz1
			c, ok := w[l]
			if !ok {
				log.Printf("Region router got a command to close region but there is no open region (%#v)", l)
			} else {
				close(c.c)
				delete(w, l)
			}
		case "get":
			fallthrough
		case "set":
			rx1, rz1 := region.At(r.cx1, r.cz1)
			getOrCreateWorker(r.world, r.dimension, rx1, rz1, r)
		case "countRegion":
			rx1, rz1 := region.At(r.cx1, r.cz1)
			rx2, rz2 := region.At(r.cx2, r.cz2)
			for rz := rz1; rz < rz2; rz++ {
				for rx := rx1; rx < rx2; rx++ {
					getOrCreateWorker(r.world, r.dimension, rx, rz, r)
				}
			}
		}
	}
	close(closeTicker)
	autocloseTicker.Stop()
	for _, v := range w {
		close(v.c)
	}
	log.Println("Region router stopped for storage ", s.Root)
}

// from Path getSaveDirectory(RegistryKey<World> worldRef, Path worldDirectory)
func (s *FilesystemChunkStorage) getRegionPath(loc regionLocator) string {
	return path.Join(s.getRegionFolder(loc), fmt.Sprintf("r.%d.%d.mca", loc.rx, loc.rz))
}

func (s *FilesystemChunkStorage) getRegionFolder(loc regionLocator) string {
	if loc.dimension == "overworld" {
		return path.Join(s.Root, loc.world, "region")
	} else if loc.dimension == "the_end" {
		return path.Join(s.Root, loc.world, "DIM1", "region")
	} else if loc.dimension == "the_nether" {
		return path.Join(s.Root, loc.world, "DIM-1", "region")
	} else {
		return path.Join(s.Root, loc.world, "dimensions", "webchunk", loc.dimension, "region")
	}
}

// region worker holds file and performs operations on it
// if it fails to open or other error occurs it will signal router
// to close a region and respond to all pending requests with error
// until no more requests will arrive (router will close channel)
func (s *FilesystemChunkStorage) regionWorker(loc regionLocator, ch <-chan regionRequest, create bool) {
	reg, err := region.Open(s.getRegionPath(loc))
	refresher := time.NewTicker(500 * time.Millisecond)
	sendClose := func(err error) {
		s.requests <- regionRequest{
			op:        "regionClose",
			world:     loc.world,
			dimension: loc.dimension,
			cx1:       loc.rx,
			cz1:       loc.rz,
		}
	closeLoop2:
		for {
			select {
			case r, ok := <-ch:
				if !ok {
					break closeLoop2
				}
				r.result <- err
			case <-refresher.C:
			}
		}
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && create {
			reg, err = region.Create(s.getRegionPath(loc))
			if err != nil {
				sendClose(err)
				return
			}
		} else {
			sendClose(err)
			return
		}
	}
workerLoop:
	for {
		select {
		case r, ok := <-ch:
			if !ok {
				break workerLoop
			}
			switch r.op {
			case "set":
				x, z := region.In(r.cx1, r.cz1)
				err = reg.WriteSector(x, z, r.data)
				if err != nil {
					sendClose(err)
					return
				} else {
					r.result <- nil
				}
			case "get":
				x, z := region.In(r.cx1, r.cz1)
				d, err := reg.ReadSector(x, z)
				if err != nil {
					if errors.Is(err, region.ErrNoData) || errors.Is(err, region.ErrNoSector) || errors.Is(err, region.ErrSectorNegativeLength) {
						r.result <- nil
					} else if errors.Is(err, region.ErrTooLarge) {
						r.result <- nil //TODO: read c.x.z.mcc data
					} else {
						sendClose(err)
					}
				} else {
					r.result <- d
				}
			case "count":
				c := 0
				for x := 0; x < 32; x++ {
					for z := 0; z < 32; z++ {
						if reg.ExistSector(x, z) {
							c++
						}
					}
				}
				r.result <- c
			case "countRegion":
				for rx := 0; rx < 32; rx++ {
					for rz := 0; rz < 32; rz++ {
						x := rx * loc.rx
						z := rz * loc.rz
						if x >= r.cx1 && x <= r.cx2 && z >= r.cz1 && z <= r.cz2 {
							if reg.ExistSector(rx, rz) {
								r.result <- chunkStorage.ChunkData{
									X:    x,
									Z:    z,
									Data: int(1),
								}
							} else {
								r.result <- chunkStorage.ChunkData{
									X:    x,
									Z:    z,
									Data: int(1),
								}
							}
						}
					}
				}
			}
		case <-refresher.C:
		}
	}
	reg.Close()
}

func (s *FilesystemChunkStorage) AddChunk(wname, dname string, cx, cz int, col save.Chunk) error {
	d, err := col.Data(2)
	if err != nil {
		return err
	}
	return s.AddChunkRaw(wname, dname, cx, cz, d)
}

func (s *FilesystemChunkStorage) AddChunkRaw(wname, dname string, cx, cz int, dat []byte) error {
	r := make(chan interface{}, 2)
	s.requests <- regionRequest{
		op:        "set",
		world:     wname,
		dimension: dname,
		cx1:       cx,
		cx2:       0,
		cz1:       cz,
		cz2:       0,
		result:    r,
		data:      dat,
	}
	for ret := range r {
		switch v := ret.(type) {
		case error:
			return v
		case nil:
			return nil
		}
	}
	return errors.New("no response from region worker")
}

func (s *FilesystemChunkStorage) GetChunk(wname, dname string, cx, cz int) (*save.Chunk, error) {
	d, err := s.GetChunkRaw(wname, dname, cx, cz)
	if err != nil {
		return nil, err
	}
	if len(d) == 0 {
		return nil, nil
	}
	var c save.Chunk
	err = c.Load(d)
	return &c, err
}

func (s *FilesystemChunkStorage) GetChunkRaw(wname, dname string, cx, cz int) ([]byte, error) {
	r := make(chan interface{}, 2)
	s.requests <- regionRequest{
		op:        "get",
		world:     wname,
		dimension: dname,
		cx1:       cx,
		cz1:       cz,
		result:    r,
	}
GetChunkRawRecvLoop:
	for ret := range r {
		switch v := ret.(type) {
		case error:
			// log.Println("GetChunkRaw", cx, cz, "got error", v)
			return []byte{}, v
		case []byte:
			// log.Println("GetChunkRaw", cx, cz, "got chunk data with len", len(v))
			return v, nil
		case nil:
			// log.Println("GetChunkRaw", cx, cz, "chunk does not exist")
			return []byte{}, nil
		default:
			log.Printf("GetChunkRaw wrong result %T", ret)
			break GetChunkRawRecvLoop
		}
	}
	return []byte{}, errors.New("no response from region worker")
}

func normalizeCoords(x0, z0, x1, z1 int) (int, int, int, int) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if z0 > z1 {
		z0, z1 = z1, z0
	}
	return x0, z0, x1, z1
}

func (s *FilesystemChunkStorage) GetChunksRegion(wname, dname string, cx0, cz0, cx1, cz1 int) ([]chunkStorage.ChunkData, error) {
	cx0, cz0, cx1, cz1 = normalizeCoords(cx0, cz0, cx1, cz1)
	log.Println("GetChunksRegion", cx0, cz0, cx1, cz1)
	r := make(chan *chunkStorage.ChunkData, (cx1-cx0)*(cz1-cz0))
	t := 0
	for x := cx0; x < cx1; x++ {
		for z := cz0; z < cz1; z++ {
			s.wg.Add(1)
			t++
			sx, sz := x, z
			go func() {
				d, err := s.GetChunk(wname, dname, sx, sz)
				if err != nil {
					r <- &chunkStorage.ChunkData{
						X:    sx,
						Z:    sz,
						Data: err,
					}
				} else {
					r <- &chunkStorage.ChunkData{
						X:    sx,
						Z:    sz,
						Data: d,
					}
				}
				s.wg.Done()
			}()
		}
	}
	ret := []chunkStorage.ChunkData{}
	var errs error
collectLoop:
	for d := range r {
		t--
		switch d.Data.(type) {
		case nil:
			// log.Println("GetChunksRegion collected EMPTY", d.X, d.Z, "left", t)
		case error:
			// log.Println("GetChunksRegion collected error", d.X, d.Z, "left", t, d.Data)
		default:
			ret = append(ret, *d)
			// log.Println("GetChunksRegion collected", fmt.Sprintf("%T", d.Data), d.X, d.Z, "left", t)
		}
		if t == 0 {
			break collectLoop
		}
	}
	log.Println("GetChunksRegion return with", len(ret), errs)
	return ret, errs
}

func (s *FilesystemChunkStorage) GetChunksRegionRaw(wname, dname string, cx0, cz0, cx1, cz1 int) ([]chunkStorage.ChunkData, error) {
	cx0, cz0, cx1, cz1 = normalizeCoords(cx0, cz0, cx1, cz1)
	r := make(chan *chunkStorage.ChunkData, 16)
	e := make(chan error, 2)
	t := 0
	for x := cx0; x < cx1; x++ {
		for z := cz0; z < cz1; z++ {
			s.wg.Add(1)
			t++
			sx, sz := x, z
			go func() {
				d, err := s.GetChunkRaw(wname, dname, sx, sz)
				if err != nil {
					e <- err
				} else {
					r <- &chunkStorage.ChunkData{
						X:    sx,
						Z:    sz,
						Data: d,
					}
				}
				s.wg.Done()
			}()
		}
	}
	ret := []chunkStorage.ChunkData{}
	var errs error
collectLoop:
	for {
		select {
		case d := <-r:
			ret = append(ret, *d)
			t--
			if t == 0 {
				break collectLoop
			}
		case err := <-e:
			multierror.Append(errs, err)
			t--
			if t == 0 {
				break collectLoop
			}
		}
	}
	return ret, errs
}

func (s *FilesystemChunkStorage) GetChunksCountRegion(wname, dname string, cx0, cz0, cx1, cz1 int) ([]chunkStorage.ChunkData, error) {
	cx0, cz0, cx1, cz1 = normalizeCoords(cx0, cz0, cx1, cz1)
	resCount := (cx1 - cx0) * (cz1 - cz0)
	res := make(chan interface{}, (resCount)/2)
	s.requests <- regionRequest{
		op:        "countRegion",
		world:     wname,
		dimension: dname,
		cx1:       cx0,
		cx2:       cx1,
		cz1:       cz0,
		cz2:       cz1,
		data:      []byte{},
		result:    res,
	}
	resGot := 0
	var err error
	ret := []chunkStorage.ChunkData{}
	for resGot < resCount {
		r := (<-res).(chunkStorage.ChunkData)
		switch d := r.Data.(type) {
		case error:
			multierror.Append(err, d)
		case int:
			ret = append(ret, r)
		}
	}
	return ret, err
}

func (s *FilesystemChunkStorage) GetDimensionChunksCount(wname, dname string) (uint64, error) {
	dirloc := s.getRegionFolder(regionLocator{
		world:     wname,
		dimension: dname,
	})
	d, err := os.ReadDir(dirloc)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		} else {
			return 0, err
		}
	}
	var wg sync.WaitGroup
	var r atomic.Int64
	r.Store(0)
	for _, i := range d {
		if !i.IsDir() && strings.HasSuffix(i.Name(), ".mca") {
			wg.Add(1)
			go func(fname string) {
				a, err := CountRegionChunks(fname)
				if err != nil {
					log.Println("Failed to count region chunks", fname, err)
				}
				r.Add(int64(a))
				wg.Done()
			}(path.Join(dirloc, i.Name()))
		}
	}
	wg.Wait()
	return uint64(r.Load()), nil
}

// counts occupied header space of the region file
func CountRegionChunks(fname string) (int, error) {
	f, err := os.Open(fname)
	if os.IsNotExist(err) {
		return 0, nil
	}
	d := make([]int32, 1024)
	err = binary.Read(f, binary.BigEndian, &d)
	if err != nil {
		return 0, err
	}
	s := 0
	for i := 0; i < 1024; i++ {
		if d[i] != 0 {
			s++
		}
	}
	return s, f.Close()
}

func (s *FilesystemChunkStorage) GetDimensionChunksSize(wname, dname string) (r uint64, err error) {
	dirloc := s.getRegionFolder(regionLocator{
		world:     wname,
		dimension: dname,
	})
	d, err := os.ReadDir(dirloc)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		} else {
			return 0, err
		}
	}
	for _, i := range d {
		if !i.IsDir() && strings.HasSuffix(i.Name(), ".mca") {
			n, err := i.Info()
			if err != nil {
				log.Println("Error loading file info", path.Join(dirloc, i.Name()), err)
			}
			r += uint64(n.Size())
		}
	}
	return r, nil
}
