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

package main

import (
	"context"
	"errors"
	"image"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
)

var (
	imageCacheMaxCache        = 64
	imageCachePropagateLevels = 16
	imageCacheProcess         = make(chan cacheTask, 32)
)

type cachedImage struct {
	img          *image.RGBA
	syncedToDisk bool
}

type imageLoc struct {
	world, dim, render string
	s, x, z            int
}

type cacheTask struct {
	loc imageLoc
	img *image.RGBA
	ret chan<- *image.RGBA
}

// ideally ring around all players should be loaded in case they are going somewhere suddenly but oh well

func imageCacheProcessor(ctx context.Context) {
	imageCache := map[imageLoc]cachedImage{}
	cleanupTicker := time.NewTicker(15 * time.Second)
	for {
		select {
		case <-ctx.Done():
			log.Println("Image cache sutting down...")
			for k, v := range imageCache {
				if !v.syncedToDisk {
					err := cacheSave(v.img, k.world, k.dim, k.render, k.s, k.x, k.z)
					if err != nil {
						log.Printf("Failed to save cache of %s:%s:%s at %ds %dx %dz because %v", k.world, k.dim, k.render, k.s, k.x, k.z, err)
					}
				}
			}
			return
		case p := <-imageCacheProcess:
			if p.img == nil { // read
				if p.ret == nil {
					log.Printf("Requested image but no return channel?! %v", spew.Sdump(p))
					break
				}
				i, ok := imageCache[p.loc]
				if ok {
					p.ret <- i.img
					break
				}
				var err error
				i.img, err = cacheLoad(p.loc.world, p.loc.dim, p.loc.render, p.loc.s, p.loc.x, p.loc.z)
				if err != nil {
					if !errors.Is(err, os.ErrNotExist) {
						log.Printf("Weird stuff you got with image cache %v: %v", p.loc, err)
					}
					close(p.ret)
					break
				}
				p.ret <- i.img
				i.syncedToDisk = true
				imageCache[p.loc] = i
			} else { // write
				imageCache[p.loc] = cachedImage{
					img:          p.img,
					syncedToDisk: false,
				}
				if p.loc.s == 0 {
					for ts := 1; ts <= imageCachePropagateLevels; ts++ {
						tsize := 16 * (2 << (ts - 1))
						pabsx := p.loc.x * 16
						pabsz := p.loc.z * 16
						tloc := imageLoc{
							world:  p.loc.world,
							dim:    p.loc.dim,
							render: p.loc.render,
							s:      ts,
							x:      pabsx / tsize,
							z:      pabsz / tsize,
						}
						img, ok := imageCache[tloc]
						if !ok {
							img = cachedImage{
								img: image.NewRGBA(image.Rectangle{
									Min: image.Point{0, 0},
									Max: image.Point{tsize, tsize},
								}),
								syncedToDisk: false,
							}
						}
						toofsetx := int(pabsx % tsize)
						toofsetz := int(pabsz % tsize)
						draw.Draw(img.img, image.Rect(toofsetx, toofsetz, toofsetx+16, toofsetz+16), p.img, image.Pt(0, 0), draw.Over)
						imageCache[p.loc] = img
					}
				}
			}
		case <-cleanupTicker.C:
			if len(imageCache) < imageCacheMaxCache*2 {
				break
			}
			keys := make([]imageLoc, 0, len(imageCache))
			for k, v := range imageCache {
				keys = append(keys, k)
				if !v.syncedToDisk {
					err := cacheSave(v.img, k.world, k.dim, k.render, k.s, k.x, k.z)
					if err != nil {
						log.Printf("Failed to save cache of %s:%s:%s at %ds %dx %dz because %v", k.world, k.dim, k.render, k.s, k.x, k.z, err)
					}
				}
			}
			sort.Slice(keys, func(i, j int) bool {
				return keys[i].s < keys[j].s
			})
			for i := 0; i < imageCacheMaxCache; i++ {
				delete(imageCache, keys[i])
			}
		}
	}
}

func imageCacheGetBlocking(world, dim, render string, s, x, z int) *image.RGBA {
	recv := make(chan *image.RGBA)
	imageCacheProcess <- cacheTask{
		loc: imageLoc{
			world:  world,
			dim:    dim,
			render: render,
			s:      s,
			x:      x,
			z:      z,
		},
		img: nil,
		ret: recv,
	}
	for i := range recv {
		return i
	}
	return nil
}

func imageCacheSave(img *image.RGBA, world, dim, render string, s, x, z int) {
	imageCacheProcess <- cacheTask{
		loc: imageLoc{
			world:  world,
			dim:    dim,
			render: render,
			s:      s,
			x:      x,
			z:      z,
		},
		img: img,
		ret: nil,
	}
}

func getImageCachePrefix() string {
	prefix := os.Getenv("CACHE_PREFIX")
	if prefix == "" {
		prefix = "imageCache"
	}
	return prefix
}

func cacheGetFilename(world, dim, render string, s, x, z int) string {
	return path.Join(".", getImageCachePrefix(), world, dim, render, strconv.FormatInt(int64(s), 10), strconv.FormatInt(int64(x), 10)+"x"+strconv.FormatInt(int64(z), 10)+".png")
}

func cacheSave(img *image.RGBA, world, dim, render string, s, x, z int) error {
	storePath := cacheGetFilename(world, dim, render, s, x, z)
	err := os.MkdirAll(path.Dir(storePath), 0764)
	if err != nil {
		return err
	}
	file, err := os.Create(storePath)
	if err != nil {
		return err
	}
	err = png.Encode(file, img)
	if err != nil {
		return err
	}
	return file.Close()
}

func cacheLoad(world, dim, render string, s, x, z int) (*image.RGBA, error) {
	fp := cacheGetFilename(world, dim, render, s, x, z)
	f, err := os.Open(fp)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ii, _, err := image.Decode(f)
	if err != nil {
		log.Printf("Failed to decode image cache %v: %v", fp, err)
		err = os.Remove(fp)
		if err != nil {
			log.Printf("Failed to remove broken cache file %v: %v", fp, err)
		}
		return nil, err
	}
	return imageToRGBA(ii), nil
}

func imageToRGBA(src image.Image) *image.RGBA {
	if dst, ok := src.(*image.RGBA); ok {
		return dst
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

func DirCountSize(path string) (count, size int64, err error) {
	err = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			count++
			size += info.Size()
		}
		return err
	})
	if os.IsNotExist(err) {
		return 0, 0, nil
	}
	return count, size, err
}

func getImageCacheCountSize(world, dim string) (int64, int64, error) {
	return DirCountSize(path.Join(".", getImageCachePrefix(), world, dim))
}
