package renderers

import (
	"image"
	"image/color"
	"strings"

	"github.com/maxsupermanhd/WebChunk/data/biomes"
	"github.com/maxsupermanhd/WebChunk/render"
	"github.com/maxsupermanhd/go-vmc/v764/level"
	"github.com/maxsupermanhd/go-vmc/v764/save"
)

func prepareSectionBiomes(s *save.Section) *level.PaletteContainer[level.BiomesState] {
	rawp := make([]level.BiomesState, len(s.Biomes.Palette))
	for pi, vv := range s.Biomes.Palette {
		v := strings.TrimPrefix(string(vv), "minecraft:")
		i, ok := biomes.BiomeID[v]
		if !ok {
			i = 127
		}
		rawp[pi] = level.BiomesState(i)
	}
	return level.NewBiomesPaletteContainerWithData(4*4*4, s.Biomes.Data, rawp)
}

func NewBiomesChunkRenderer(colors []color.RGBA) render.ChunkRenderer {
	return render.ChunkRenderer{
		Name: "biome",
		Render: func(data render.ChunkData) (img *image.RGBA) {
			chunk := data.Get()
			img = image.NewRGBA(image.Rect(0, 0, 4, 4))
			c := prepareSectionBiomes(&chunk.Sections[len(chunk.Sections)])
			for i := 0; i < 4*4; i++ {
				biomeid := int(c.Get(i))
				if biomeid >= 0 && biomeid < len(colors) {
					img.Set(i%4, i/4, colors[biomeid])
				} else {
					img.Set(i%4, i/4, color.Black)
				}
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
