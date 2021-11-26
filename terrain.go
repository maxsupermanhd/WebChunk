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
	"github.com/nfnt/resize"
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

func getChunksRegion(dname, sname string, cx0, cz0, cx1, cz1 int) ([]save.Column, error) {
	// log.Printf("Requesting rectange x%d z%d  ==  x%d z%d", cx0, cz0, cx1, cz1)
	c := []save.Column{}
	rows, derr := dbpool.Query(context.Background(), `
		with grp as
		 (
			select x, z, data, created_at, dim, id,
				rank() over (partition by x, z order by x, z, created_at desc) r
			from chunks
		)
		select data, id
		from grp
		where x >= $1 AND z >= $2 AND x < $3 AND z < $4 AND r = 1 AND
			dim = (select dimensions.id 
			 from dimensions 
			 join servers on servers.id = dimensions.server 
			 where servers.name = $5 and dimensions.name = $6)
		`, cx0, cz0, cx1, cz1, sname, dname)
	if derr != nil {
		if derr != pgx.ErrNoRows {
			log.Print(derr.Error())
		}
		return c, derr
	}
	var perr error
	for rows.Next() {
		var d []byte
		var cid int
		rows.Scan(&d, &cid)
		var cc save.Column
		perr = cc.Load(d)
		if perr != nil {
			log.Printf("Chunk %d: %s", cid, perr.Error())
			continue
		}
		c = append(c, cc)
	}
	return c, perr
}

func terrainScaleJpegHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	dname := params["dim"]
	sname := params["server"]
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
	scale := int(math.Pow(2, float64(cs)))
	imagesize := 512
	cc, err := getChunksRegion(dname, sname, cx*scale, cz*scale, cx*scale+scale, cz*scale+scale)
	if err != nil {
		plainmsg(w, r, 2, "Error getting chunk data: "+err.Error())
		return
	}
	img := image.NewRGBA(image.Rect(0, 0, imagesize, imagesize))
	imagescale := int(imagesize / scale)
	// log.Print("Scale ", scale)
	// log.Print("Image scale ", imagescale, imagesize-imagescale)
	offsetx := cx * scale
	offsety := cz * scale
	// log.Print("Offsets ", offsetx, offsety)
	// log.Print("Chunks ", len(cc))
	for _, c := range cc {
		placex := int(c.Level.PosX) - offsetx
		placey := int(c.Level.PosZ) - offsety
		// log.Printf("Chunk [%d] %d %d offsetted %d %d scaled %v", i,
		// c.Level.PosX, c.Level.PosZ, placex, placey, image.Rect(placex*int(imagescale), placey*int(imagescale), imagescale, imagescale))
		tile := resize.Resize(uint(imagescale), uint(imagescale), drawColumn(&c), resize.NearestNeighbor)
		draw.Draw(img, image.Rect(placex*int(imagescale), placey*int(imagescale), placex*int(imagescale)+imagescale, placey*int(imagescale)+imagescale),
			tile, image.Pt(0, 0), draw.Over)
		// draw.Draw(img, image.Rect(0, 0, imagesize, imagesize),
		// 	tile, image.Pt(placex*int(imagescale), placey*int(imagescale)),
		// 	draw.Src)
	}
	w.WriteHeader(http.StatusOK)
	writeImage(w, img)
}

func drawColumn(column *save.Column) (img *image.RGBA) {
	img = image.NewRGBA(image.Rect(0, 0, 16, 16))
	c := column.Level.Sections
	for _, s := range c {
		drawSection(&s, img)
	}
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
	w.WriteHeader(http.StatusOK)
	writeImage(w, drawColumn(&c))
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
