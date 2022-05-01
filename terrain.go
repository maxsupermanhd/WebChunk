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
	"encoding/gob"
	"html/template"
	"image"
	"image/color"
	"image/draw"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	_ "sync"
	"time"
	"unsafe"

	"github.com/Tnze/go-mc/level"
	"github.com/Tnze/go-mc/level/block"
	"github.com/Tnze/go-mc/save"
	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
)

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

func isAirState(s int) bool {
	switch block.StateList[s].(type) {
	case block.Air, block.CaveAir, block.VoidAir:
		return true
	default:
		return false
	}
}

func isAirBlock(s block.Block) bool {
	return s.ID() == "air" || s.ID() == "cave_air" || s.ID() == "void_air"
}

func prepareSectionBlockstates(s *save.Section) *level.PaletteContainer {
	data := *(*[]uint64)((unsafe.Pointer)(&s.BlockStates.Data))
	statePalette := s.BlockStates.Palette
	stateRawPalette := make([]int, len(statePalette))
	for i, v := range statePalette {
		b, ok := block.FromID[v.Name]
		if !ok {
			b, ok = block.FromID["minecraft:"+v.Name]
			if !ok {
				if os.Getenv("REPORT_CHUNK_PROBLEMS") == "all" {
					log.Printf("Can not find block from id [%v]", v.Name)
				}
				return nil
			}
		}
		if v.Properties.Data != nil {
			err := v.Properties.Unmarshal(&b)
			if err != nil {
				if os.Getenv("REPORT_CHUNK_PROBLEMS") == "all" {
					log.Printf("Error unmarshling properties of block [%v] from [%v]: %v", v.Name, v.Properties.String(), err.Error())
				}
				return nil
			}
		}
		s := block.ToStateID[b]
		stateRawPalette[i] = s
	}
	return level.NewStatesPaletteContainerWithData(16*16*16, data, stateRawPalette)
}

func drawChunkHeightmap(chunk *save.Chunk) (img *image.RGBA) {
	t := time.Now()
	img = image.NewRGBA(image.Rect(0, 0, 16, 16))
	defaultColor := color.RGBA{0, 0, 0, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{defaultColor}, image.Point{}, draw.Src)
	sort.Slice(chunk.Sections, func(i, j int) bool {
		return int8(chunk.Sections[i].Y) > int8(chunk.Sections[j].Y)
	})
	for _, s := range chunk.Sections {
		if len(s.BlockStates.Data) == 0 {
			continue
		}
		states := prepareSectionBlockstates(&s)
		if states == nil {
			if os.Getenv("REPORT_CHUNK_PROBLEMS") == "yes" || os.Getenv("REPORT_CHUNK_PROBLEMS") == "all" {
				log.Printf("Chunk %d:%d section %d has broken pallete", chunk.XPos, chunk.YPos, s.Y)
			}
			continue
		}
		for y := 15; y >= 0; y-- {
			layerImg := image.NewRGBA(image.Rect(0, 0, 16, 16))
			for i := 16*16 - 1; i >= 0; i-- {
				if img.At(i%16, i/16) != defaultColor {
					continue
				}
				state := states.Get(y*16*16 + i)
				// block := block.StateList[state]
				if !isAirState(state) {
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
		return int8(chunk.Sections[i].Y) > int8(chunk.Sections[j].Y)
	})
	type OutputBlock struct {
		sR, sG, sB, sA uint64
		c              uint64
		b              []block.Block
	}
	outputs := make([]OutputBlock, 16*16)
	failedState := 0
	failedID := 0
	colored := make([]bool, 32*32)
	for i := range colored {
		colored[i] = false
	}
	for _, s := range chunk.Sections {
		if len(s.BlockStates.Data) == 0 {
			continue
		}
		states := prepareSectionBlockstates(&s)
		if states == nil {
			if os.Getenv("REPORT_CHUNK_PROBLEMS") == "yes" || os.Getenv("REPORT_CHUNK_PROBLEMS") == "all" {
				log.Printf("Chunk %d:%d section %d has broken pallete", chunk.XPos, chunk.YPos, s.Y)
			}
			continue
		}
		for y := 15; y >= 0; y-- {
			for i := 16*16 - 1; i >= 0; i-- {
				if colored[i] {
					continue
				}
				state := states.Get(y*16*16 + i)
				blockState := block.StateList[state]
				if isAirState(state) {
					continue
				}
				toColor := color.RGBA64{R: 0, G: 0, B: 0, A: 0}
				isTransparent := false
				switch blockState.(type) {
				// Grass tint for plains
				// TODO: actually grab correct tint from biome
				case block.GrassBlock:
					toColor = color.RGBA64{R: 0x91 * 257, G: 0xBD * 257, B: 0x59 * 257, A: 0xFF * 257}
				case block.Grass:
					toColor = color.RGBA64{R: 0x91 * 257, G: 0xBD * 257, B: 0x59 * 257, A: 0x7F * 257}
					isTransparent = true
				case block.TallGrass:
					toColor = color.RGBA64{R: 0x91 * 257, G: 0xBD * 257, B: 0x59 * 257, A: 0x7F * 257}
					isTransparent = true
				case block.Fern:
					toColor = color.RGBA64{R: 0x91 * 257, G: 0xBD * 257, B: 0x59 * 257, A: 0x7F * 257}
					isTransparent = true
				case block.LargeFern:
					toColor = color.RGBA64{R: 0x91 * 257, G: 0xBD * 257, B: 0x59 * 257, A: 0x7F * 257}
					isTransparent = true
				case block.PottedFern:
					toColor = color.RGBA64{R: 0x91 * 257, G: 0xBD * 257, B: 0x59 * 257, A: 0x7F * 257}
					isTransparent = true
				case block.SugarCane:
					toColor = color.RGBA64{R: 0x91 * 257, G: 0xBD * 257, B: 0x59 * 257, A: 0x7F * 257}
					isTransparent = true

				// Foliage tint for plains
				// TODO: actually grab correct tint from biome
				case block.OakLeaves:
					toColor = color.RGBA64{R: 0x77 * 257, G: 0xAB * 257, B: 0x2F * 257, A: 0x7F * 257}
					isTransparent = true
				case block.JungleLeaves:
					toColor = color.RGBA64{R: 0x77 * 257, G: 0xAB * 257, B: 0x2F * 257, A: 0x7F * 257}
					isTransparent = true
				case block.AcaciaLeaves:
					toColor = color.RGBA64{R: 0x77 * 257, G: 0xAB * 257, B: 0x2F * 257, A: 0x7F * 257}
					isTransparent = true
				case block.DarkOakLeaves:
					toColor = color.RGBA64{R: 0x77 * 257, G: 0xAB * 257, B: 0x2F * 257, A: 0x7F * 257}
					isTransparent = true
				case block.Vine:
					toColor = color.RGBA64{R: 0x77 * 257, G: 0xAB * 257, B: 0x2F * 257, A: 0x7F * 257}
					isTransparent = true

				// Water tint for "most biomes" lmao

				case block.Water:
					toColor = color.RGBA64{R: 0x3F * 257, G: 0x76 * 257, B: 0xE4 * 257, A: 0x7F * 257}
					// isTransparent = true
				case block.WaterCauldron:
					toColor = color.RGBA64{R: 0x3F * 257, G: 0x76 * 257, B: 0xE4 * 257, A: 0xFF * 257}
				default:
					toColor = colors[state]
				}

				if isTransparent {
					outputs[i].sR += uint64(toColor.R)
					outputs[i].sG += uint64(toColor.G)
					outputs[i].sB += uint64(toColor.B)
					outputs[i].sA += uint64(toColor.A)
					outputs[i].c++
					outputs[i].b = append(outputs[i].b, blockState)
				} else {
					if outputs[i].c != 0 {
						backColor := toColor
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
						toColor = color.RGBA64{uint16(finalR), uint16(finalG), uint16(finalB), 65535}
						// log.Println("Final blend", fmt.Sprintf("% 3d %02d:%02d", outputs[i].c, i%16, i/16), printColor(colors[state]), printColor(backColor), printColor(frontColor), printColor(final))
					}
					// log.Printf("Painting %02d:%02d %v %#v %#v", i%16, i/16, toColor, blockState.ID(), outputs[i].b)
					img.Set(i%16, i/16, toColor)
					colored[i] = true
				}

				// absy := uint(int(s.Y)*16 + y)
			}
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

func drawChunkXray(chunk *save.Chunk) (img *image.RGBA) {
	t := time.Now()
	img = image.NewRGBA(image.Rect(0, 0, 16, 16))
	defaultColor := color.RGBA{0, 0, 0, 0}
	draw.Draw(img, img.Bounds(), &image.Uniform{defaultColor}, image.Point{}, draw.Src)
	sort.Slice(chunk.Sections, func(i, j int) bool {
		return int8(chunk.Sections[i].Y) > int8(chunk.Sections[j].Y)
	})
	type OutputBlock struct {
		sR, sG, sB, sA uint64
		c              uint64
		b              []block.Block
	}
	outputs := make([]OutputBlock, 16*16)
	failedState := 0
	failedID := 0
	colored := make([]bool, 32*32)
	for i := range colored {
		colored[i] = false
	}
	for _, s := range chunk.Sections {
		if len(s.BlockStates.Data) == 0 {
			continue
		}
		states := prepareSectionBlockstates(&s)
		if states == nil {
			if os.Getenv("REPORT_CHUNK_PROBLEMS") == "yes" || os.Getenv("REPORT_CHUNK_PROBLEMS") == "all" {
				log.Printf("Chunk %d:%d section %d has broken pallete", chunk.XPos, chunk.YPos, s.Y)
			}
			continue
		}
		for y := 15; y >= 0; y-- {
			for i := 16*16 - 1; i >= 0; i-- {
				state := states.Get(y*16*16 + i)
				toColor := colors[state]
				outputs[i].sR += uint64(toColor.R)
				outputs[i].sG += uint64(toColor.G)
				outputs[i].sB += uint64(toColor.B)
				outputs[i].sA += uint64(toColor.A)
				outputs[i].c++
				outputs[i].b = append(outputs[i].b, block.StateList[state])
			}
		}
	}
	for i := range outputs {
		if outputs[i].c != 0 {
			toColor := color.RGBA64{uint16(outputs[i].sR / outputs[i].c), uint16(outputs[i].sG / outputs[i].c), uint16(outputs[i].sB / outputs[i].c), 65535}
			img.Set(i%16, i/16, toColor)
			// log.Println("Final blend", fmt.Sprintf("% 3d %02d:%02d", outputs[i].c, i%16, i/16), printColor(colors[state]), printColor(backColor), printColor(frontColor), printColor(final))
		}
	}
	if failedState != 0 {
		log.Println("Failed to lookup", failedState, "block states")
	}
	if failedID != 0 {
		log.Println("Failed to lookup", failedID, "block IDS")
	}
	appendMetrics(time.Since(t), "xray")
	return img
}

func drawChunkPortalBlocksHeatmap(chunk *save.Chunk) (img *image.RGBA) {
	t := time.Now()
	portalsDetected := 0
	for _, s := range chunk.Sections {
		if len(s.BlockStates.Data) == 0 {
			continue
		}
		states := prepareSectionBlockstates(&s)
		if states == nil {
			if os.Getenv("REPORT_CHUNK_PROBLEMS") == "yes" || os.Getenv("REPORT_CHUNK_PROBLEMS") == "all" {
				log.Printf("Chunk %d:%d section %d has broken pallete", chunk.XPos, chunk.YPos, s.Y)
			}
			continue
		}
		for y := 15; y >= 0; y-- {
			for i := 16*16 - 1; i >= 0; i-- {
				b := block.StateList[states.Get(y*16*16+i)]
				if b.ID() == "nether_portal" {
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

func drawChunkChestBlocksHeatmap(chunk *save.Chunk) (img *image.RGBA) {
	t := time.Now()
	portalsDetected := 0
	for _, s := range chunk.Sections {
		if len(s.BlockStates.Data) == 0 {
			continue
		}
		states := prepareSectionBlockstates(&s)
		if states == nil {
			if os.Getenv("REPORT_CHUNK_PROBLEMS") == "yes" || os.Getenv("REPORT_CHUNK_PROBLEMS") == "all" {
				log.Printf("Chunk %d:%d section %d has broken pallete", chunk.XPos, chunk.YPos, s.Y)
			}
			continue
		}
		for y := 15; y >= 0; y-- {
			for i := 16*16 - 1; i >= 0; i-- {
				b := block.StateList[states.Get(y*16*16+i)]
				if b.ID() == "chest" {
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

func initChunkDraw() {
	if err := gob.NewDecoder(bytes.NewReader(colorsBin)).Decode(&colors); err != nil {
		panic(err)
	}
	go metricsDispatcher()
}

func terrainInfoHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	wname := params["world"]
	dname := params["dim"]
	world, s, err := chunkStorage.GetWorldStorage(storages, wname)
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Error getting world: "+err.Error())
		return
	}
	if world == nil || s == nil {
		plainmsg(w, r, plainmsgColorRed, "World not found")
		return
	}
	dim, err := s.GetDimension(wname, dname)
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Database query error: "+err.Error())
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
	c, err := s.GetChunk(wname, dname, cx, cz)
	if err != nil {
		plainmsg(w, r, 2, "Chunk query error: "+err.Error())
		return
	}
	basicLayoutLookupRespond("chunkinfo", w, r, map[string]interface{}{"World": world, "Dim": dim, "Chunk": c, "PrettyChunk": template.HTML(spew.Sdump(c))})
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
