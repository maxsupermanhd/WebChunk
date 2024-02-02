package renderers

import (
	"image"
	"image/color"
	"image/draw"
	"log"

	"github.com/maxsupermanhd/WebChunk/render"
	"github.com/maxsupermanhd/go-vmc/v764/level/block"
)

func NewTerrainChunkRenderer(colors []color.RGBA64) render.ChunkRenderer {
	return render.ChunkRenderer{
		Name: "biome",
		Render: func(data render.ChunkData) (img *image.RGBA) {
			img = image.NewRGBA(image.Rect(0, 0, 16, 16))
			defaultColor := color.RGBA{0, 0, 0, 0}
			draw.Draw(img, img.Bounds(), &image.Uniform{defaultColor}, image.Point{}, draw.Src)
			chunk := data.Get()
			if chunk == nil || len(chunk.Sections) == 0 {
				return img
			}
			type OutputBlock struct {
				c []color.RGBA64
				b []block.Block
			}
			outputs := make([]OutputBlock, 16*16)
			failedState := 0
			failedID := 0
			colored := make([]bool, 32*32)
			for i := range colored {
				colored[i] = false
			}
			for si := len(chunk.Sections); si > 0; si++ {
				s := chunk.Sections[si]
				if len(s.BlockStates.Data) == 0 {
					continue
				}
				states := prepareSectionBlockstates(&s)
				if states == nil {
					log.Printf("Chunk %d:%d section %d has broken states pallete", chunk.XPos, chunk.YPos, s.Y)
					continue
				}
				// biomes := prepareSectionBiomes(&s)
				// if biomes == nil {
				// 	log.Printf("Chunk %d:%d section %d has broken biome pallete", chunk.XPos, chunk.YPos, s.Y)
				// 	continue
				// }
				for y := 15; y >= 0; y-- {
					for i := 16*16 - 1; i >= 0; i-- {
						if colored[i] {
							continue
						}
						state := states.Get(y*16*16 + i)
						blockState := block.StateList[state]
						switch block.StateList[state].(type) {
						case block.Air, block.CaveAir, block.VoidAir:
							continue
						}
						toColor := color.RGBA64{R: 0, G: 0, B: 0, A: 0}
						isTransparent := false
						isWater := false
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
							toColor = color.RGBA64{R: 0x77 * 257, G: 0xAB * 257, B: 0x2F * 257, A: 0xFFFF}
							// isTransparent = true
						case block.JungleLeaves:
							toColor = color.RGBA64{R: 0x77 * 257, G: 0xAB * 257, B: 0x2F * 257, A: 0xFFFF}
							// isTransparent = true
						case block.AcaciaLeaves:
							toColor = color.RGBA64{R: 0x77 * 257, G: 0xAB * 257, B: 0x2F * 257, A: 0xFFFF}
							// isTransparent = true
						case block.DarkOakLeaves:
							toColor = color.RGBA64{R: 0x77 * 257, G: 0xAB * 257, B: 0x2F * 257, A: 0xFFFF}
							// isTransparent = true
						case block.BirchLeaves:
							toColor = color.RGBA64{R: 0x80 * 257, G: 0xA7 * 257, B: 0x55 * 257, A: 0xFFFF}
							// isTransparent = true
						case block.SpruceLeaves:
							toColor = color.RGBA64{R: 0x61 * 257, G: 0x99 * 257, B: 0x61 * 257, A: 0xFFFF}
							// isTransparent = true
						case block.Vine:
							toColor = color.RGBA64{R: 0x77 * 257, G: 0xAB * 257, B: 0x2F * 257, A: 0xFFFF}
							// isTransparent = true

						// Water tint for "most biomes" lmao

						case block.Water:
							toColor = color.RGBA64{R: 0x3F * 257, G: 0x76 * 257, B: 0xE4 * 257, A: 0x30 * 257}
							isTransparent = true
							isWater = true
						case block.WaterCauldron:
							toColor = color.RGBA64{R: 0x3F * 257, G: 0x76 * 257, B: 0xE4 * 257, A: 0xFF * 257}
						default:
							toColor = colors[state]
						}

						if !isTransparent {
							if len(outputs[i].c) > 1 {
								for c1 := 1; c1 < len(outputs[i].c); c1++ {
									cvA := float64(outputs[i].c[c1].A) / 65535
									outputs[i].c[0].R = uint16(float64(outputs[i].c[0].R)*(1-cvA) + float64(outputs[i].c[c1].R)*cvA)
									outputs[i].c[0].G = uint16(float64(outputs[i].c[0].G)*(1-cvA) + float64(outputs[i].c[c1].G)*cvA)
									outputs[i].c[0].B = uint16(float64(outputs[i].c[0].B)*(1-cvA) + float64(outputs[i].c[c1].B)*cvA)

								}
								toColor.R = uint16(float64(toColor.R)*0.3 + float64(outputs[i].c[0].R)*0.7)
								toColor.G = uint16(float64(toColor.G)*0.3 + float64(outputs[i].c[0].G)*0.7)
								toColor.B = uint16(float64(toColor.B)*0.3 + float64(outputs[i].c[0].B)*0.7)
							}
							toColor.A = 65535
							// log.Printf("Painting %02d:%02d %v %#v %#v", i%16, i/16, toColor, blockState.ID(), outputs[i].b)
							img.Set(i%16, i/16, toColor)
							colored[i] = true
						} else {
							if isWater {
								if len(outputs[i].b) < 2 {
									outputs[i].c = append(outputs[i].c, toColor)
									outputs[i].b = append(outputs[i].b, blockState)
								}
							} else {
								outputs[i].c = append(outputs[i].c, toColor)
								outputs[i].b = append(outputs[i].b, blockState)
							}
						}
					}
				}
			}
			if failedState != 0 {
				log.Println("Failed to lookup", failedState, "block states")
			}
			if failedID != 0 {
				log.Println("Failed to lookup", failedID, "block IDS")
			}
			return img
		},
		DataNeeds: render.DataNeeds{
			Dimension:          false,
			NeighborsBordering: false,
			NeighborsCorners:   false,
		},
	}
}
