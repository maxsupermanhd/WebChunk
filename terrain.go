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
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	_ "sync"
	"time"
	"unsafe"

	"github.com/Tnze/go-mc/data/block"
	"github.com/Tnze/go-mc/level"
	"github.com/Tnze/go-mc/save"
	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

func getChunkData(dname, sname string, cx, cz int) (save.Chunk, error) {
	var c save.Chunk
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

func getChunksRegion(dname, sname string, cx0, cz0, cx1, cz1 int) ([]chunkData, error) {
	// log.Printf("Requesting rectange x%d z%d  ==  x%d z%d", cx0, cz0, cx1, cz1)
	c := []chunkData{}
	dim, err := getDimensionByNames(sname, dname)
	if err != nil {
		return c, err
	}
	rows, derr := dbpool.Query(context.Background(), `
		with grp as
		 (
			select x, z, data, created_at, dim, id,
				rank() over (partition by x, z order by x, z, created_at desc) r
			from chunks where dim = $5
		)
		select data, id
		from grp
		where x >= $1 AND z >= $2 AND x < $3 AND z < $4 AND r = 1 AND dim = $5
		`, cx0, cz0, cx1, cz1, dim.ID)
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
		var cc save.Chunk
		perr = cc.Load(d)
		if perr != nil {
			log.Printf("Chunk %d: %s", cid, perr.Error())
			continue
		}
		c = append(c, chunkData{x: cc.XPos, z: cc.ZPos, data: cc})
	}
	return c, perr
}

type metricsCollect struct {
	t time.Duration
	m string
}

type metricsMeasure struct {
	sum   time.Duration
	count int64
}

var (
	metricsSend = make(chan metricsCollect, 1024)
	metrics     = map[string]metricsMeasure{}
)

func metricsDispatcher() {
	for m := range metricsSend {
		d, ok := metrics[m.m]
		if ok {
			d.count++
			d.sum += m.t
			metrics[m.m] = d
		} else {
			metrics[m.m] = metricsMeasure{sum: m.t, count: 1}
		}
		if ok && d.count%200 == 0 {
			log.Println("Chunk", m.m, "rendering metrics", time.Duration(d.sum.Nanoseconds()/d.count).String(), "per chunk (total", d.count, ")")
		}
	}
}

func appendMetrics(t time.Duration, m string) {
	metricsSend <- metricsCollect{t: t, m: m}
}

func drawChunkHeightmap(chunk *save.Chunk) (img *image.RGBA) {
	t := time.Now()
	img = image.NewRGBA(image.Rect(0, 0, 16, 16))
	defaultColor := color.RGBA{0, 0, 0, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{defaultColor}, image.Point{}, draw.Src)
	sort.Slice(chunk.Sections, func(i, j int) bool {
		return chunk.Sections[i].Y > chunk.Sections[j].Y
	})
	for _, s := range chunk.Sections {
		if len(s.BlockStates.Data) == 0 {
			continue
		}
		data := *(*[]uint64)((unsafe.Pointer)(&s.BlockStates.Data))
		palette := s.BlockStates.Palette
		rawPalette := make([]int, len(palette))
		for i, v := range palette {
			rawPalette[i] = int(stateIDs[strings.TrimPrefix(v.Name, "minecraft:")])
		}
		c := level.NewStatesPaletteContainerWithData(16*16*16, data, rawPalette)
		for y := 15; y >= 0; y-- {
			layerImg := image.NewRGBA(image.Rect(0, 0, 16, 16))
			for i := 16*16 - 1; i >= 0; i-- {
				if img.At(i%16, i/16) != defaultColor {
					continue
				}
				state, ok := block.StateID[uint32(c.Get(y*16*16+i))]
				if !ok {
					continue
				}
				block, ok := block.ByID[state]
				if !ok {
					continue
				}
				if block.Name != "air" {
					absy := uint8(int(s.Y)*16 + y)
					layerImg.Set(i%16, i/16, color.RGBA{absy, absy, 255, 255})
				}
			}
			draw.Draw(
				img, image.Rect(0, 0, 16, 16),
				layerImg, image.Pt(0, 0),
				draw.Over,
			)
		}
	}
	appendMetrics(time.Since(t), "heightmap")
	return img
}

func drawChunk(chunk *save.Chunk) (img *image.RGBA) {
	t := time.Now()
	img = image.NewRGBA(image.Rect(0, 0, 16, 16))
	defaultColor := color.RGBA{0, 0, 0, 0}
	draw.Draw(img, img.Bounds(), &image.Uniform{defaultColor}, image.Point{}, draw.Src)
	sort.Slice(chunk.Sections, func(i, j int) bool {
		return chunk.Sections[i].Y > chunk.Sections[j].Y
	})
	type OutputBlock struct {
		sR, sG, sB, sA uint64
		c              uint64
	}
	outputs := make([]OutputBlock, 16*16)
	failedState := 0
	failedID := 0
	for _, s := range chunk.Sections {
		if len(s.BlockStates.Data) == 0 {
			continue
		}
		data := *(*[]uint64)((unsafe.Pointer)(&s.BlockStates.Data))
		palette := s.BlockStates.Palette
		rawPalette := make([]int, len(palette))
		for i, v := range palette {
			rawPalette[i] = int(stateIDs[strings.TrimPrefix(v.Name, "minecraft:")])
		}
		c := level.NewStatesPaletteContainerWithData(16*16*16, data, rawPalette)
		for y := 15; y >= 0; y-- {
			layerImg := image.NewRGBA(image.Rect(0, 0, 16, 16))
			for i := 16*16 - 1; i >= 0; i-- {
				if img.At(i%16, i/16) != defaultColor {
					continue
				}
				state, ok := block.StateID[uint32(c.Get(y*16*16+i))]
				if !ok {
					failedState++
					continue
				}
				block, ok := block.ByID[state]
				if !ok {
					failedID++
					continue
				}
				if block.Name == "air" {
					continue
				}
				// printColor := func(c color.RGBA64) string {
				// 	return fmt.Sprintf("{% 6d % 6d % 6d % 6d}", c.R, c.G, c.B, c.A)
				// }
				if block.Transparent {
					outputs[i].sR += uint64(colors[state].R)
					outputs[i].sG += uint64(colors[state].G)
					outputs[i].sB += uint64(colors[state].B)
					outputs[i].sA += uint64(colors[state].A)
					outputs[i].c++
				} else {
					if outputs[i].c == 0 {
						// log.Println("Painting", colors[state], "no alpha")
						layerImg.Set(i%16, i/16, colors[state])
					} else {
						backColor := colors[state]
						frontColor := color.RGBA64{
							R: uint16(outputs[i].sR / outputs[i].c),
							G: uint16(outputs[i].sG / outputs[i].c),
							B: uint16(outputs[i].sB / outputs[i].c),
							A: uint16(outputs[i].sA / outputs[i].c),
						}
						multiply := 1 - float64(frontColor.A)/float64(65535)
						backColor.R = uint16(float64(backColor.R) * multiply)
						backColor.G = uint16(float64(backColor.G) * multiply)
						backColor.B = uint16(float64(backColor.B) * multiply)
						finalR := uint32(backColor.R) + uint32(frontColor.R)
						finalG := uint32(backColor.G) + uint32(frontColor.G)
						finalB := uint32(backColor.B) + uint32(frontColor.B)
						if finalR > 65535 {
							finalR = 65535
						}
						if finalG > 65535 {
							finalG = 65535
						}
						if finalB > 65535 {
							finalB = 65535
						}
						// I know that capping those values is a bad idea and there is a proper solution
						// But I am too lazy and/or stupid to implement it, I tried for over 2 hours already
						final := color.RGBA64{uint16(finalR), uint16(finalG), uint16(finalB), 65535}
						// log.Println("Final blend", fmt.Sprintf("% 3d %02d:%02d", outputs[i].c, i%16, i/16), printColor(colors[state]), printColor(backColor), printColor(frontColor), printColor(final))
						layerImg.Set(i%16, i/16, final)
					}
				}
				// absy := uint8(int(s.Y)*16 + y)
			}
			draw.Draw(
				img, image.Rect(0, 0, 16, 16),
				layerImg, image.Pt(0, 0),
				draw.Over,
			)
		}
	}
	if failedState != 0 {
		log.Println("Failed to lookup", failedState, "block states")
	}
	if failedID != 0 {
		log.Println("Failed to lookup", failedID, "block IDS")
	}
	appendMetrics(time.Since(t), "colors")
	return img
}

func drawChunkPortalBlocksHeightmap(chunk *save.Chunk) (img *image.RGBA) {
	t := time.Now()
	portalBlockID, ok := idByName["nether_portal"]
	if !ok {
		log.Println("Failed to find portal block id")
	}
	portalsDetected := 0
	for _, s := range chunk.Sections {
		if len(s.BlockStates.Data) == 0 {
			continue
		}
		data := *(*[]uint64)((unsafe.Pointer)(&s.BlockStates.Data))
		palette := s.BlockStates.Palette
		rawPalette := make([]int, len(palette))
		for i, v := range palette {
			rawPalette[i] = int(stateIDs[strings.TrimPrefix(v.Name, "minecraft:")])
		}
		c := level.NewStatesPaletteContainerWithData(16*16*16, data, rawPalette)
		for y := 15; y >= 0; y-- {
			for i := 16*16 - 1; i >= 0; i-- {
				bid, ok := block.StateID[uint32(c.Get(y*16*16+i))]
				if ok && portalBlockID == uint32(bid) {
					portalsDetected++
				}
			}
		}
	}
	img = image.NewRGBA(image.Rect(0, 0, 16, 16))
	alpha := 0
	if portalsDetected/6 > 255 {
		alpha = 255
	} else {
		alpha = portalsDetected * 8
	}
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{255, 0, 0, uint8(alpha)}}, image.Point{}, draw.Src)
	appendMetrics(time.Since(t), "portal_heat")
	return
}

var colors []color.RGBA64

//go:embed colors.gob
var colorsBin []byte // gob([]color.RGBA64)

var idByName = make(map[string]uint32, len(block.ByID))

var stateIDs map[string]uint32

func initChunkDraw() {
	if err := gob.NewDecoder(bytes.NewReader(colorsBin)).Decode(&colors); err != nil {
		panic(err)
	}
	for _, v := range block.ByID {
		// log.Println(v.Transparent, v.Name, colors[i])
		idByName[v.Name] = uint32(v.ID)
	}
	stateIDs = map[string]uint32{}
	for i, v := range block.StateID {
		name := block.ByID[v].Name
		if _, ok := stateIDs[name]; !ok {
			stateIDs[name] = i
		}
	}
	go metricsDispatcher()
}

// func drawColumnChestBlocksHeightmap(column *save.Column) (img *image.RGBA) {
// 	count := 0
// 	for si := len(column.Level.Sections) - 1; si >= 0; si-- {
// 		s := column.Level.Sections[si]
// 		bpb := len(s.BlockStates) * 64 / (16 * 16 * 16)
// 		if len(s.BlockStates) == 0 {
// 			continue
// 		}
// 		data := *(*[]uint64)(unsafe.Pointer(&s.BlockStates))
// 		bs := save.NewBitStorage(bpb, 4096, data)
// 		for y := 16 - 1; y >= 0; y-- {
// 			for i := 16*16 - 1; i >= 0; i-- {
// 				bid := getBID(bpb, bs, &s, y, i)
// 				if bid == block.Chest.ID || bid == block.EnderChest.ID {
// 					count++
// 				}
// 			}
// 		}
// 	}
// 	img = image.NewRGBA(image.Rect(0, 0, 16, 16))
// 	alpha := 0
// 	if count/8 > 255 {
// 		alpha = 255
// 	} else {
// 		alpha = count * 8
// 	}
// 	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{255, 0, 0, uint8(alpha)}}, image.Point{}, draw.Src)
// 	return
// }

// func filterBlock(i block.ID) (r bool) {
// 	m := map[block.ID]bool{
// 		block.CoalOre.ID:          true,
// 		block.RedstoneOre.ID:      true,
// 		block.IronOre.ID:          true,
// 		block.EmeraldOre.ID:       true,
// 		block.DiamondOre.ID:       true,
// 		block.MossyCobblestone.ID: true,
// 		block.Air.ID:              true,
// 	}
// 	_, r = m[i]
// 	return
// }

// func drawColumnXray(column *save.Column) (img *image.RGBA) {
// 	img = image.NewRGBA(image.Rect(0, 0, 16, 16))
// 	defaultColor := color.RGBA{0, 0, 0, 0}
// 	draw.Draw(img, img.Bounds(), &image.Uniform{defaultColor}, image.Point{}, draw.Src)
// 	for si := 0; si < len(column.Level.Sections); si++ {
// 		s := column.Level.Sections[si]
// 		bpb := len(s.BlockStates) * 64 / (16 * 16 * 16)
// 		if len(s.BlockStates) == 0 {
// 			continue
// 		}
// 		data := *(*[]uint64)(unsafe.Pointer(&s.BlockStates))
// 		bs := save.NewBitStorage(bpb, 4096, data)
// 		for y := 0; y < 16; y++ {
// 			if int(s.Y)*16+y < 8 {
// 				continue
// 			}
// 			layerImg := image.NewRGBA(image.Rect(0, 0, 16, 16))
// 			for i := 16*16 - 1; i >= 0; i-- {
// 				bid := getBID(bpb, bs, &s, y, i)
// 				if !filterBlock(bid) {
// 					layerImg.Set(i%16, i/16, color.RGBA{100, 100, 100, 1})
// 				} else {
// 					r, g, b, _ := colors[bid].RGBA()
// 					layerImg.Set(i%16, i/16, color.RGBA{uint8(r), uint8(g), uint8(b), 1})
// 				}
// 			}
// 			draw.Draw(
// 				img, image.Rect(0, 0, 16, 16),
// 				layerImg, image.Pt(0, 0),
// 				draw.Over,
// 			)
// 		}
// 	}
// 	return
// }

func terrainInfoHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	sname := params["server"]
	dname := params["dim"]
	server, derr := getServerByName(sname)
	if derr != nil {
		plainmsg(w, r, plainmsgColorRed, "Database query error: "+derr.Error())
		return
	}
	dim, derr := getDimensionByNames(sname, dname)
	if derr != nil {
		plainmsg(w, r, plainmsgColorRed, "Database query error: "+derr.Error())
		return
	}
	cxs := params["cx"]
	cx, err := strconv.Atoi(cxs)
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Chunk X coordinate is shit: "+err.Error())
		return
	}
	czs := params["cz"]
	cz, err := strconv.Atoi(czs)
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Chunk Z coordinate is shit: "+err.Error())
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
	draw.Draw(layerImg, layerImg.Bounds(), &image.Uniform{color.RGBA{255, 0, 0, uint8(c * 30)}}, image.Point{}, draw.Src)
	return layerImg
}

func getChunksCountRegion(dname, sname string, cx0, cz0, cx1, cz1 int) ([]chunkData, error) {
	cc := []chunkData{}
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
		var x, z, c int32
		derr := rows.Scan(&x, &z, &c)
		if derr != nil {
			log.Print(derr.Error())
			continue
		}
		cc = append(cc, chunkData{x: x, z: z, data: c})
	}
	return cc, derr
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
	_, err = getChunkData(dname, sname, cz, cx)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.WriteHeader(http.StatusOK)
	// writeImage(w, fname, drawColumn(&c))
}
