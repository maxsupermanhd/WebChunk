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
	"bytes"
	"context"
	_ "embed"
	"errors"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	_ "sync"

	"github.com/Tnze/go-mc/save"
	"github.com/gorilla/mux"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
	"github.com/nfnt/resize"
)

type chunkDataProviderFunc = func(wname, dname string, cx0, cz0, cx1, cz1 int) ([]chunkStorage.ChunkData, error)
type chunkPainterFunc = func(interface{}) *image.RGBA
type ttypeProviderFunc = func(chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc)

type ttype struct {
	Name        string
	DisplayName string
	IsOverlay   bool
	IsDefault   bool
}

var ttypes = map[ttype]ttypeProviderFunc{
	{"terrain", "Terrain", false, false}: func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksRegion, func(i interface{}) *image.RGBA {
			c := i.(save.Chunk)
			return drawChunk(&c)
		}
	},
	{"shadedterrain", "Shaded terrain", false, true}: func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return getChunksRegionWithContextFN(s), func(i interface{}) *image.RGBA {
			return drawShadedTerrain(i.(ContextedChunkData))
		}
	},
	{"counttiles", "Chunk count", false, false}: func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksCountRegion, func(i interface{}) *image.RGBA {
			return drawNumberOfChunks(int(i.(int)))
		}
	},
	{"counttilesheat", "Chunk count heatmap", true, false}: func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksCountRegion, func(i interface{}) *image.RGBA {
			return drawHeatOfChunks(int(i.(int)))
		}
	},
	{"heightmap", "Heightmap", false, false}: func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksRegion, func(i interface{}) *image.RGBA {
			c := i.(save.Chunk)
			return drawChunkHeightmap(&c)
		}
	},
	{"xray", "Xray", false, false}: func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksRegion, func(i interface{}) *image.RGBA {
			c := i.(save.Chunk)
			return drawChunkXray(&c)
		}
	},
	{"biomes", "Biomes", false, false}: func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksRegion, func(i interface{}) *image.RGBA {
			c := i.(save.Chunk)
			return drawChunkBiomes(&c)
		}
	},
	{"portalsheat", "Portals heatmap", true, false}: func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksRegion, func(i interface{}) *image.RGBA {
			c := i.(save.Chunk)
			return drawChunkPortalBlocksHeatmap(&c)
		}
	},
	{"chestheat", "Chest heatmap", true, false}: func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksRegion, func(i interface{}) *image.RGBA {
			c := i.(save.Chunk)
			return drawChunkChestBlocksHeatmap(&c)
		}
	},
	{"lavaage", "Lava age", false, false}: func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksRegion, func(i interface{}) *image.RGBA {
			c := i.(save.Chunk)
			return drawChunkLavaAge(&c, 255)
		}
	},
	{"lavaageoverlay", "Lava age (overlay)", true, false}: func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksRegion, func(i interface{}) *image.RGBA {
			c := i.(save.Chunk)
			return drawChunkLavaAge(&c, 128)
		}
	},
	{"shading", "Shading", true, false}: func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return getChunksRegionWithContextFN(s), func(i interface{}) *image.RGBA {
			return drawChunkShading(i.(ContextedChunkData))
		}
	},
}

func tileRouterHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	datatype := params["ttype"]
	wname, dname, fname, cx, cz, cs, err := tilingParams(w, r)
	if err != nil {
		return
	}
	if !r.URL.Query().Has("cached") || r.URL.Query().Get("cached") == "true" {
		img := imageCacheGetBlocking(wname, dname, datatype, cs, cx, cz)
		if img != nil {
			b := bytes.NewBuffer([]byte{})
			err := png.Encode(b, img)
			if err != nil {
				log.Printf("Failed to enclode image: %v", err)
			}
			bytes := b.Bytes()
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Content-Length", strconv.Itoa(len(bytes)))
			if _, err := w.Write(bytes); err != nil {
				log.Printf("Unable to write image: %s", err.Error())
			}
			return
		}
	}
	_, s, err := chunkStorage.GetWorldStorage(storages, wname)
	if err != nil {
		return
	}
	var ff ttypeProviderFunc
	ffound := false
	for tt := range ttypes {
		if tt.Name == datatype {
			ff = ttypes[tt]
			ffound = true
		}
	}
	if !ffound {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	g, p := ff(s)
	img := scaleImageryHandler(w, r, g, p)
	if img == nil {
		return
	}
	if r.Header.Get("Cache-Control") != "no-store" {
		imageCacheSave(img, wname, dname, datatype, cs, cx, cz)
	}
	w.WriteHeader(http.StatusOK)
	writeImage(w, fname, img)
	imageCacheSave(img, wname, dname, datatype, cs, cx, cz)
}

func scaleImageryHandler(w http.ResponseWriter, r *http.Request, getter chunkDataProviderFunc, painter chunkPainterFunc) *image.RGBA {
	wname, dname, _, cx, cz, cs, err := tilingParams(w, r)
	log.Println("Requested tile", wname, dname, cx, cz, cs)
	if err != nil {
		return nil
	}
	scale := 1
	if cs > 0 {
		scale = int(2 << (cs - 1))
	}
	imagesize := scale * 16
	if imagesize > 512 {
		imagesize = 512
	}
	img := image.NewRGBA(image.Rect(0, 0, int(imagesize), int(imagesize)))
	imagescale := int(imagesize / scale)
	offsetx := cx * scale
	offsety := cz * scale
	cc, err := getter(wname, dname, cx*scale, cz*scale, cx*scale+scale, cz*scale+scale)
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Error getting chunk data: "+err.Error())
		log.Println("Error getting chunk data: ", err)
		return nil
	}
	if len(cc) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	for _, c := range cc {
		if errors.Is(r.Context().Err(), context.Canceled) {
			return img
		}
		placex := int(c.X - offsetx)
		placey := int(c.Z - offsety)
		var chunk *image.RGBA
		chunk = func(d interface{}) *image.RGBA {
			defer func() {
				if err := recover(); err != nil {
					log.Println(cx, cz, err)
					debug.PrintStack()
				}
				chunk = nil
			}()
			var ret *image.RGBA
			ret = nil
			ret = painter(d)
			return ret
		}(c.Data)
		if chunk == nil {
			continue
		}
		tile := resize.Resize(uint(imagescale), uint(imagescale), chunk, resize.NearestNeighbor)
		draw.Draw(img, image.Rect(placex*int(imagescale), placey*int(imagescale), placex*int(imagescale)+imagescale, placey*int(imagescale)+imagescale),
			tile, image.Pt(0, 0), draw.Over)
	}
	return img
}

func tilingParams(w http.ResponseWriter, r *http.Request) (wname, dname, fname string, cx, cz, cs int, err error) {
	params := mux.Vars(r)
	dname = params["dim"]
	wname = params["world"]
	fname = params["format"]
	if fname != "jpeg" && fname != "png" {
		plainmsg(w, r, plainmsgColorRed, "Bad encoding")
		return
	}
	cxs := params["cx"]
	cxb, err := strconv.ParseInt(cxs, 10, 32)
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Bad cx id: "+err.Error())
		return
	}
	cx = int(cxb)
	czs := params["cz"]
	czb, err := strconv.ParseInt(czs, 10, 32)
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Bad cz id: "+err.Error())
		return
	}
	cz = int(czb)
	css := params["cs"]
	csb, err := strconv.ParseInt(css, 10, 32)
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Bad s id: "+err.Error())
		return
	}
	cs = int(csb)
	return
}

func writeImage(w http.ResponseWriter, format string, img *image.RGBA) {
	switch format {
	case "jpeg":
		writeImageJpeg(w, img)
	case "png":
		writeImagePng(w, img)
	}
}

func writeImageJpeg(w http.ResponseWriter, img *image.RGBA) {
	buffer := new(bytes.Buffer)
	if err := jpeg.Encode(buffer, img, nil); err != nil {
		log.Printf("Unable to encode image: %s", err.Error())
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(buffer.Bytes())))
	if _, err := w.Write(buffer.Bytes()); err != nil {
		log.Printf("Unable to write image: %s", err.Error())
	}
}

func writeImagePng(w http.ResponseWriter, img *image.RGBA) {
	buffer := new(bytes.Buffer)
	if err := png.Encode(buffer, img); err != nil {
		log.Printf("Unable to encode image: %s", err.Error())
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(buffer.Bytes())))
	if _, err := w.Write(buffer.Bytes()); err != nil {
		log.Printf("Unable to write image: %s", err.Error())
	}
}
