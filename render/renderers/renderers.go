package renderers

import (
	"bytes"
	"encoding/gob"
	"image/color"
	"os"

	"github.com/maxsupermanhd/WebChunk/data/biomes"
	"github.com/maxsupermanhd/WebChunk/render"
	"github.com/maxsupermanhd/lac"
)

func ConstructRenderers(cfg *lac.ConfSubtree) []render.ChunkRenderer {
	return []render.ChunkRenderer{
		NewBiomesChunkRenderer(loadBiomeColors(cfg)),
		NewTerrainChunkRenderer(loadBlockColors(cfg)),
	}
}

func loadBiomeColors(cfg *lac.ConfSubtree) []color.RGBA {
	// TODO: move to file
	return biomes.BiomeColors
}

func loadBlockColors(cfg *lac.ConfSubtree) []color.RGBA64 {
	var colors []color.RGBA64
	b, err := os.ReadFile(cfg.GetDSString("colors.gob", "blockColorsPath"))
	if err != nil {
		return nil
	}
	err = gob.NewDecoder(bytes.NewReader(b)).Decode(&colors)
	if err != nil {
		return nil
	}
	return colors
}
