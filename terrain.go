package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/gob"
	"html/template"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log"
	"math"
	"net/http"
	"strconv"
	_ "sync"
	"unsafe"

	"github.com/Tnze/go-mc/data/block"
	"github.com/Tnze/go-mc/save"
	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

func getChunkData(did, cx, cz int) (save.Column, error) {
	var c save.Column
	var d []byte
	derr := dbpool.QueryRow(context.Background(), `
		select data
		from chunks
		where dim = $1 AND x = $2 AND z = $3
		limit 1;`, did, cx, cz).Scan(&d)
	if derr != nil {
		if derr != pgx.ErrNoRows {
			log.Print(derr.Error())
		}
		return c, derr
	}
	perr := c.Load(d)
	return c, perr
}

func getChunksRegion(did, cx0, cz0, cx1, cz1 int) ([]save.Column, error) {
	c := []save.Column{}
	rows, derr := dbpool.Query(context.Background(), `
		select data
		from chunks
		where dim = $1 AND x < $2 AND x >= $3 AND z < $4 AND z >= $5
		limit 1;`, did, cx0, cz0, cx1, cz1)
	if derr != nil {
		if derr != pgx.ErrNoRows {
			log.Print(derr.Error())
		}
		return c, derr
	}
	var perr error
	for rows.Next() {
		var d []byte
		rows.Scan(&d)
		var cc save.Column
		perr = cc.Load(d)
		if perr != nil {
			continue
		}
		c = append(c, cc)
	}
	return c, perr
}

func terrainScaleJpegHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	dids := params["did"]
	did, err := strconv.Atoi(dids)
	if err != nil {
		plainmsg(w, r, 2, "Bad dim id: "+err.Error())
		return
	}
	cxs := params["cx"]
	cx, err := strconv.Atoi(cxs)
	if err != nil {
		plainmsg(w, r, 2, "Bad cx id: "+err.Error())
		return
	}
	czs := params["cz"]
	cz, err := strconv.Atoi(czs)
	if err != nil {
		plainmsg(w, r, 2, "Bad cz id: "+err.Error())
		return
	}
	css := params["cs"]
	cs, err := strconv.Atoi(css)
	if err != nil {
		plainmsg(w, r, 2, "Bad s id: "+err.Error())
		return
	}
	cc, err := getChunksRegion(did, cx, cz, cx+int(math.Pow(2, float64(cs))), cz+int(math.Pow(2, float64(cs))))
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	_ = cc
}

func drawRegion(d, s, x, z int) (img *image.RGBA, err error) {
	// cc, err := getChunksRegion(d, x, z, x+int(math.Pow(2, float64(s))), z+int(math.Pow(2, float64(s))))
	// if err != nil {
	// 	return nil, err
	// }
	// img = image.NewRGBA(image.Rect(0, 0, 16*int(math.Pow(2, float64(s))), 16*int(math.Pow(2, float64(s)))))
	// for i, c := range cc {
		// draw.Draw(
		// 	img, image.Rect(0, 0, 16, 16),
		// 	layerImg, image.Pt(0, 0),
		// 	draw.Over,
		// )
		// drawColumn
	// }
	return nil, nil
}

func drawColumn(column *save.Column) (img *image.RGBA) {
	img = image.NewRGBA(image.Rect(0, 0, 16, 16))
	c := column.Level.Sections
	for _, s := range c {
		drawSection(&s, img)
		// wg.Done()
	}
	// c := make(chan *save.Chunk)
	// var wg sync.WaitGroup
	// for i := 0; i < 2; i++ {
	// 	go func() {
	// 	}()
	// }
	// defer close(c)
	// wg.Add(len(s))
	// for i := range s {
	// 	c <- &s[i]
	// }
	// wg.Wait()
	return
}

var colors []color.RGBA64

//go:embed colors.gob
var colorsBin []byte // gob([]color.RGBA64)

var idByName = make(map[string]uint32, len(block.ByID))

func initChunkDraw() {
	for _, v := range block.ByID {
		idByName["minecraft:"+v.Name] = uint32(v.ID)
	}
	if err := gob.NewDecoder(bytes.NewReader(colorsBin)).Decode(&colors); err != nil {
		panic(err)
	}
}

func drawSection(s *save.Chunk, img *image.RGBA) {
	bpb := len(s.BlockStates) * 64 / (16 * 16 * 16)
	if len(s.BlockStates) == 0 {
		return
	}
	data := *(*[]uint64)(unsafe.Pointer(&s.BlockStates))
	bs := save.NewBitStorage(bpb, 4096, data)
	for y := 0; y < 16; y++ {
		layerImg := image.NewRGBA(image.Rect(0, 0, 16, 16))
		for i := 16*16 - 1; i >= 0; i-- {
			var bid block.ID
			switch {
			case bpb > 9:
				bid = block.StateID[uint32(bs.Get(y*16*16+i))]
			case bpb > 4:
				fallthrough
			case bpb <= 4:
				b := s.Palette[bs.Get(y*16*16+i)]
				if id, ok := idByName[b.Name]; ok {
					bid = block.StateID[id]
				}
			}
			c := colors[block.ByID[bid].ID]
			layerImg.Set(i%16, i/16, c)
		}
		draw.Draw(
			img, image.Rect(0, 0, 16, 16),
			layerImg, image.Pt(0, 0),
			draw.Over,
		)
	}
	return
}

func writeImage(w http.ResponseWriter, img *image.RGBA) {
	buffer := new(bytes.Buffer)
	if err := jpeg.Encode(buffer, img, nil); err != nil {
		log.Println("unable to encode image.")
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(buffer.Bytes())))
	if _, err := w.Write(buffer.Bytes()); err != nil {
		log.Println("unable to write image.")
	}
}

func terrainJpegHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	dids := params["did"]
	did, err := strconv.Atoi(dids)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	cxs := params["cx"]
	cx, err := strconv.Atoi(cxs)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	czs := params["cz"]
	cz, err := strconv.Atoi(czs)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	c, err := getChunkData(did, cz, cx)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeImage(w, drawColumn(&c))
	w.WriteHeader(http.StatusOK)
}

func terrainInfoHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	sids := params["sid"]
	sid, err := strconv.Atoi(sids)
	if err != nil {
		plainmsg(w, r, 2, "Bad server id: "+err.Error())
		return
	}
	dids := params["did"]
	did, err := strconv.Atoi(dids)
	if err != nil {
		plainmsg(w, r, 2, "Bad dim id: "+err.Error())
		return
	}
	server, derr := getServerByID(sid)
	if derr != nil {
		plainmsg(w, r, 2, "Database query error: "+derr.Error())
		return
	}
	dim, derr := getDimensionByID(did)
	if derr != nil {
		plainmsg(w, r, 2, "Database query error: "+derr.Error())
		return
	}
	cxs := params["cx"]
	cx, err := strconv.Atoi(cxs)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	czs := params["cz"]
	cz, err := strconv.Atoi(czs)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	c, err := getChunkData(did, cz, cx)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	basicLayoutLookupRespond("chunkinfo", w, r, map[string]interface{}{"Server": server, "Dim": dim, "Chunk": c, "PrettyChunk": template.HTML(spew.Sdump(c))})
}
