package main

import (
	"fmt"
	"image/color"
	"net/http"

	"github.com/Tnze/go-mc/data/block"
)

func hexColor(c color.Color) string {
	rgba := color.RGBAModel.Convert(c).(color.RGBA)
	return fmt.Sprintf("%.2x%.2x%.2x%.2x", rgba.R, rgba.G, rgba.B, rgba.A)
}

func colorsHandlerGET(w http.ResponseWriter, r *http.Request) {
	type BlockColor struct {
		Block block.Block
		Color string
	}
	c := make([]BlockColor, len(block.ByID))
	for i, b := range block.ByID {
		c[i].Block = *b
		c[i].Color = hexColor(colors[i])
	}
	basicLayoutLookupRespond("colors", w, r, map[string]interface{}{"Colors": c})
}
