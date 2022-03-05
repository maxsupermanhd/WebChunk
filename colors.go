package main

import (
	"encoding/gob"
	"fmt"
	"image/color"
	"net/http"
	"os"
	"strconv"

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

func ParseHexColor(s string) (c color.RGBA64, err error) {
	t := color.RGBA{}
	_, err = fmt.Sscanf(s, "#%02x%02x%02x%02x", &t.R, &t.G, &t.B, &t.A)
	c.R = uint16(t.R) * 256
	c.G = uint16(t.G) * 256
	c.B = uint16(t.B) * 256
	c.A = uint16(t.A) * 256
	return
}

func colorsHandlerPOST(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Error parsing form data: "+err.Error())
		return
	}
	colorids := r.PostFormValue("colorid")
	if colorids == "" {
		plainmsg(w, r, plainmsgColorRed, "No colorid in request")
		return
	}
	colorid, err := strconv.Atoi(colorids)
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Failed to parse colorid: "+err.Error())
		return
	}
	if colorid < 0 || colorid > len(colors) {
		plainmsg(w, r, plainmsgColorRed, "Bad colorid in request")
		return
	}
	colorvalue := r.PostFormValue("colorvalue")
	if colorvalue == "" {
		plainmsg(w, r, plainmsgColorRed, "No colorvalue in request")
		return
	}
	newColor, err := ParseHexColor(colorvalue)
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Failed to parse colorvalue")
		return
	}
	colors[colorid] = newColor
	plainmsg(w, r, plainmsgColorGreen, fmt.Sprint("Color ", colorid, " was changed to ", newColor))
	colorsHandlerGET(w, r)
}

func colorsSaveHandler(w http.ResponseWriter, r *http.Request) {
	f, err := os.Create("colors.gob")
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Error saving color palette to disk: "+err.Error())
		return
	}
	defer f.Close()
	if err := gob.NewEncoder(f).Encode(colors); err != nil {
		plainmsg(w, r, plainmsgColorRed, "Error saving color palette to disk: "+err.Error())
		return
	}
	plainmsg(w, r, plainmsgColorGreen, "Color palette saved to disk.")
}
