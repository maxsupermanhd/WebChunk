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
	"image/png"
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

func getChunkData(dname, sname string, cx, cz int) (save.Column, error) {
	var c save.Column
	var d []byte
	derr := dbpool.QueryRow(context.Background(), `
		select data
		from chunks
		where x = $1 AND z = $2 AND
			dim = (select dimensions.id 
			 from dimensions 
			 join servers on servers.id = dimensions.server 
			 where servers.name = $3 and dimensions.name = $4)
		order by created_at desc
		limit 1;`, cx, cz, sname, dname).Scan(&d)
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

func terrainScaleImageHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	err, sname, dname, fname, cx, cz, cs := tilingParams(w, r, params)
	if err != nil {
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
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{0, 0, 0, 255}}, image.ZP, draw.Src)
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
	writeImage(w, fname, img)
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

func terrainImageHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	dname := params["dim"]
	sname := params["server"]
	fname := params["format"]
	if fname != "jpeg" && fname != "png" {
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
	c, err := getChunkData(dname, sname, cz, cx)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusOK)
	writeImage(w, fname, drawColumn(&c))
}

func terrainInfoHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	sname := params["server"]
	dname := params["dim"]
	server, derr := getServerByName(sname)
	if derr != nil {
		plainmsg(w, r, 2, "Database query error: "+derr.Error())
		return
	}
	dim, derr := getDimensionByNames(sname, dname)
	if derr != nil {
		plainmsg(w, r, 2, "Database query error: "+derr.Error())
		return
	}
	cxs := params["cx"]
	cx, err := strconv.Atoi(cxs)
	if err != nil {
		plainmsg(w, r, 2, "Chunk X coordinate is shit: "+err.Error())
		return
	}
	czs := params["cz"]
	cz, err := strconv.Atoi(czs)
	if err != nil {
		plainmsg(w, r, 2, "Chunk Z coordinate is shit: "+err.Error())
		return
	}
	c, err := getChunkData(dname, sname, cx, cz)
	if err != nil {
		plainmsg(w, r, 2, "Chunk query error: "+err.Error())
		return
	}
	basicLayoutLookupRespond("chunkinfo", w, r, map[string]interface{}{"Server": server, "Dim": dim, "Chunk": c, "PrettyChunk": template.HTML(spew.Sdump(c))})
}

func drawNumberOfChunks(c int) *image.RGBA {
	layerImg := image.NewRGBA(image.Rect(0, 0, 16, 16))
	digits := [][]string{
		{"001100", "010010", "010010", "010010", "010010", "001100"},
		{"001100", "010100", "000100", "000100", "000100", "000100"},
		{"001100", "010010", "000100", "001000", "010000", "011110"},
		{"001100", "010010", "000100", "000100", "010010", "001100"},
		{"000010", "000110", "001010", "010010", "011110", "000010"},
		{"011110", "010000", "011100", "000010", "000010", "011100"},
		{"001100", "010000", "011100", "010010", "010010", "001100"},
		{"011110", "000010", "000110", "001100", "011000", "010000"},
		{"001100", "010010", "001100", "001100", "010010", "001100"},
		{"001100", "010010", "001110", "000010", "000010", "001100"}}
	d1 := c % 10
	d2 := int(c / 10 % 10)
	for i := 0; i < 6; i++ {
		for j := 0; j < 6; j++ {
			if digits[d2][i][j] == '1' {
				layerImg.Set(j, i, color.Black)
			} else {
				layerImg.Set(j, i, color.White)
			}
			if digits[d1][i][j] == '1' {
				layerImg.Set(7+j, i, color.Black)
			} else {
				layerImg.Set(7+j, i, color.White)
			}
		}
	}
	return layerImg
}

func drawHeatOfChunks(c int) *image.RGBA {
	layerImg := image.NewRGBA(image.Rect(0, 0, 16, 16))
	draw.Draw(layerImg, layerImg.Bounds(), &image.Uniform{color.RGBA{255, 0, 0, uint8(c * 30)}}, image.ZP, draw.Src)
	return layerImg
}

type chunkCounts struct {
	x, z, c int
}

func getChunksCountRegion(dname, sname string, cx0, cz0, cx1, cz1 int) ([]chunkCounts, error) {
	// log.Printf("Requesting rectange x%d z%d  ==  x%d z%d", cx0, cz0, cx1, cz1)
	cc := []chunkCounts{}
	rows, derr := dbpool.Query(context.Background(), `
	select
	x, z, coalesce(count(*), 0) as c
	from chunks
	where dim = (select dimensions.id 
				 from dimensions 
				 join servers on servers.id = dimensions.server 
				 where servers.name = $5 and dimensions.name = $6) AND
		  x >= $1 AND z >= $2 AND x < $3 AND z < $4
	group by x, z
	order by c desc
		`, cx0, cz0, cx1, cz1, sname, dname)
	if derr != nil {
		if derr != pgx.ErrNoRows {
			log.Print(derr.Error())
		}
		return cc, derr
	}
	for rows.Next() {
		var x, z, c int
		derr := rows.Scan(&x, &z, &c)
		if derr != nil {
			log.Print(derr.Error())
			continue
		}
		cc = append(cc, chunkCounts{x: x, z: z, c: c})
	}
	return cc, derr
}

func tilingParams(w http.ResponseWriter, r *http.Request, params map[string]string) (err error, sname, dname, fname string, cx, cz, cs int) {
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

func terrainChunkCountScaleImageHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	err, sname, dname, fname, cx, cz, cs := tilingParams(w, r, params)
	if err != nil {
		return
	}
	scale := int(math.Pow(2, float64(cs)))
	imagesize := 512
	cc, err := getChunksCountRegion(dname, sname, cx*scale, cz*scale, cx*scale+scale, cz*scale+scale)
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
		placex := int(c.x) - offsetx
		placey := int(c.z) - offsety
		// log.Printf("Chunk [%d] %d %d offsetted %d %d scaled %v", i,
		// c.Level.PosX, c.Level.PosZ, placex, placey, image.Rect(placex*int(imagescale), placey*int(imagescale), imagescale, imagescale))
		tile := resize.Resize(uint(imagescale), uint(imagescale), drawNumberOfChunks(c.c), resize.NearestNeighbor)
		draw.Draw(img, image.Rect(placex*int(imagescale), placey*int(imagescale), placex*int(imagescale)+imagescale, placey*int(imagescale)+imagescale),
			tile, image.Pt(0, 0), draw.Over)
		// draw.Draw(img, image.Rect(0, 0, imagesize, imagesize),
		// 	tile, image.Pt(placex*int(imagescale), placey*int(imagescale)),
		// 	draw.Src)
	}
	w.WriteHeader(http.StatusOK)
	writeImage(w, fname, img)
}

func terrainChunkCountHeatScaleImageHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	err, sname, dname, fname, cx, cz, cs := tilingParams(w, r, params)
	if err != nil {
		return
	}
	scale := int(math.Pow(2, float64(cs)))
	imagesize := 512
	cc, err := getChunksCountRegion(dname, sname, cx*scale, cz*scale, cx*scale+scale, cz*scale+scale)
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
		placex := int(c.x) - offsetx
		placey := int(c.z) - offsety
		// log.Printf("Chunk [%d] %d %d offsetted %d %d scaled %v", i,
		// c.Level.PosX, c.Level.PosZ, placex, placey, image.Rect(placex*int(imagescale), placey*int(imagescale), imagescale, imagescale))
		tile := resize.Resize(uint(imagescale), uint(imagescale), drawHeatOfChunks(c.c), resize.NearestNeighbor)
		draw.Draw(img, image.Rect(placex*int(imagescale), placey*int(imagescale), placex*int(imagescale)+imagescale, placey*int(imagescale)+imagescale),
			tile, image.Pt(0, 0), draw.Over)
		// draw.Draw(img, image.Rect(0, 0, imagesize, imagesize),
		// 	tile, image.Pt(placex*int(imagescale), placey*int(imagescale)),
		// 	draw.Src)
	}
	w.WriteHeader(http.StatusOK)
	writeImage(w, fname, img)
}
