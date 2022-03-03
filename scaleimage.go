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
	"github.com/nfnt/resize"
)

type chunkData struct {
	x, z int32
	data interface{}
}

type chunkDataProviderFunc = func(dname, sname string, cx0, cz0, cx1, cz1 int) ([]chunkData, error)
type chunkPainterFunc = func(interface{}) *image.RGBA

func tileRouterHandler(w http.ResponseWriter, r *http.Request) {
	var g chunkDataProviderFunc
	var p chunkPainterFunc
	params := mux.Vars(r)
	datatype := params["ttype"]
	switch datatype {
	case "terrain":
		g = getChunksRegion
		p = func(i interface{}) *image.RGBA {
			s := i.(save.Chunk)
			return drawChunk(&s)
		}
	case "counttiles":
		g = getChunksCountRegion
		p = func(i interface{}) *image.RGBA {
			return drawNumberOfChunks(int(i.(int32)))
		}
	case "counttilesheat":
		g = getChunksCountRegion
		p = func(i interface{}) *image.RGBA {
			return drawHeatOfChunks(int(i.(int32)))
		}
	case "heightmap":
		g = getChunksRegion
		p = func(i interface{}) *image.RGBA {
			s := i.(save.Chunk)
			return drawChunkHeightmap(&s)
		}
	case "xray":
		g = getChunksRegion
		p = func(i interface{}) *image.RGBA {
			s := i.(save.Chunk)
			// return drawChunkXray(&s)
			return drawChunk(&s)
		}
	case "portalsheat":
		g = getChunksRegion
		p = func(i interface{}) *image.RGBA {
			s := i.(save.Chunk)
			return drawChunkPortalBlocksHeightmap(&s)
		}
	case "chestheat":
		g = getChunksRegion
		p = func(i interface{}) *image.RGBA {
			s := i.(save.Chunk)
			// return drawChunkChestBlocksHeightmap(&s)
			return drawChunk(&s)
		}
	}
	scaleImageryHandler(w, r, g, p)
}

func scaleImageryHandler(w http.ResponseWriter, r *http.Request, getter func(dname, sname string, cx0, cz0, cx1, cz1 int) ([]chunkData, error), painter func(interface{}) *image.RGBA) {
	sname, dname, fname, cx, cz, cs, err := tilingParams(w, r)
	if err != nil {
		return
	}
	scale := int(math.Pow(2, float64(cs)))
	imagesize := 512
	img := image.NewRGBA(image.Rect(0, 0, imagesize, imagesize))
	imagescale := int(imagesize / scale)
	offsetx := cx * scale
	offsety := cz * scale
	cc, err := getter(dname, sname, cx*scale, cz*scale, cx*scale+scale, cz*scale+scale)
	if err != nil {
		plainmsg(w, r, 2, "Error getting chunk data: "+err.Error())
		return
	}
	for _, c := range cc {
		placex := int(c.x) - offsetx
		placey := int(c.z) - offsety
		tile := resize.Resize(uint(imagescale), uint(imagescale), painter(c.data), resize.NearestNeighbor)
		draw.Draw(img, image.Rect(placex*int(imagescale), placey*int(imagescale), placex*int(imagescale)+imagescale, placey*int(imagescale)+imagescale),
			tile, image.Pt(0, 0), draw.Over)
	}
	w.WriteHeader(http.StatusOK)
	writeImage(w, fname, img)
}

func tilingParams(w http.ResponseWriter, r *http.Request) (sname, dname, fname string, cx, cz, cs int, err error) {
	params := mux.Vars(r)
	dname = params["dim"]
	sname = params["server"]
	fname = params["format"]
	if fname != "jpeg" && fname != "png" {
		plainmsg(w, r, 2, "Bad encoding")
		return
	}
	cxs := params["cx"]
	cx, err = strconv.Atoi(cxs)
	if err != nil {
		plainmsg(w, r, 2, "Bad cx id: "+err.Error())
		return
	}
	czs := params["cz"]
	cz, err = strconv.Atoi(czs)
	if err != nil {
		plainmsg(w, r, 2, "Bad cz id: "+err.Error())
		return
	}
	css := params["cs"]
	cs, err = strconv.Atoi(css)
	if err != nil {
		plainmsg(w, r, 2, "Bad s id: "+err.Error())
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
