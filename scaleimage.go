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
	_ "embed"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"log"
	"math"
	"net/http"
	"strconv"
	_ "sync"

	"github.com/Tnze/go-mc/save"
	"github.com/gorilla/mux"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
	"github.com/nfnt/resize"
)

type chunkDataProviderFunc = func(dname, sname string, cx0, cz0, cx1, cz1 int) ([]chunkStorage.ChunkData, error)
type chunkPainterFunc = func(interface{}) *image.RGBA
type ttypeProviderFunc = func(chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc)

var ttypes = map[string]ttypeProviderFunc{
	"terrain": func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksRegion, func(i interface{}) *image.RGBA {
			s := i.(save.Chunk)
			return drawChunk(&s)
		}
	},
	"counttiles": func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksCountRegion, func(i interface{}) *image.RGBA {
			return drawNumberOfChunks(int(i.(int32)))
		}
	},
	"counttilesheat": func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksCountRegion, func(i interface{}) *image.RGBA {
			return drawHeatOfChunks(int(i.(int32)))
		}
	},
	"heightmap": func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksRegion, func(i interface{}) *image.RGBA {
			s := i.(save.Chunk)
			return drawChunkHeightmap(&s)
		}
	},
	"xray": func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksRegion, func(i interface{}) *image.RGBA {
			s := i.(save.Chunk)
			return drawChunkXray(&s)
		}
	},
	"biomes": func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksRegion, func(i interface{}) *image.RGBA {
			s := i.(save.Chunk)
			return drawChunkBiomes(&s)
		}
	},
	"portalsheat": func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksRegion, func(i interface{}) *image.RGBA {
			s := i.(save.Chunk)
			return drawChunkPortalBlocksHeatmap(&s)
		}
	},
	"chestheat": func(s chunkStorage.ChunkStorage) (chunkDataProviderFunc, chunkPainterFunc) {
		return s.GetChunksRegion, func(i interface{}) *image.RGBA {
			s := i.(save.Chunk)
			return drawChunkChestBlocksHeatmap(&s)
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
	if r.Header.Get("Cache-Control") != "no-cache" {
		if bytes, err := loadImageCache(wname, dname, datatype, cs, cx, cz); err == nil {
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
	ff, ok := ttypes[datatype]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	g, p := ff(s)
	img := scaleImageryHandler(w, r, g, p)
	if img == nil {
		return
	}
	if r.Header.Get("Cache-Control") != "no-store" {
		err = saveImageCache(img, wname, dname, datatype, cs, cx, cz)
		if err != nil {
			log.Println("Failed to cache image:", err.Error())
		}
	}
	w.WriteHeader(http.StatusOK)
	writeImage(w, fname, img)
}

func scaleImageryHandler(w http.ResponseWriter, r *http.Request, getter chunkDataProviderFunc, painter chunkPainterFunc) *image.RGBA {
	wname, dname, _, cx, cz, cs, err := tilingParams(w, r)
	if err != nil {
		return nil
	}
	scale := int(math.Pow(2, float64(cs)))
	imagesize := 512
	img := image.NewRGBA(image.Rect(0, 0, imagesize, imagesize))
	imagescale := int(imagesize / scale)
	offsetx := cx * scale
	offsety := cz * scale
	cc, err := getter(wname, dname, cx*scale, cz*scale, cx*scale+scale, cz*scale+scale)
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Error getting chunk data: "+err.Error())
		return nil
	}
	if len(cc) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}
	for _, c := range cc {
		placex := int(c.X) - offsetx
		placey := int(c.Z) - offsety
		tile := resize.Resize(uint(imagescale), uint(imagescale), painter(c.Data), resize.NearestNeighbor)
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
	cx, err = strconv.Atoi(cxs)
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Bad cx id: "+err.Error())
		return
	}
	czs := params["cz"]
	cz, err = strconv.Atoi(czs)
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Bad cz id: "+err.Error())
		return
	}
	css := params["cs"]
	cs, err = strconv.Atoi(css)
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Bad s id: "+err.Error())
		return
	}
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
