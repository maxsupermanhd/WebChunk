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
	"encoding/gob"
	"fmt"
	"image/color"
	"net/http"
	"os"
	"strconv"

	"github.com/Tnze/go-mc/level/block"
)

func hexColor(c color.Color) string {
	rgba := color.RGBAModel.Convert(c).(color.RGBA)
	return fmt.Sprintf("%.2x%.2x%.2x%.2x", rgba.R, rgba.G, rgba.B, rgba.A)
}

func colorsHandlerGET(w http.ResponseWriter, r *http.Request) {
	offsets := r.URL.Query().Get("o")
	counts := r.URL.Query().Get("c")
	var offset, count int
	var err error
	if offsets == "" {
		offset = 0
	} else {
		offset, err = strconv.Atoi(offsets)
		if err != nil {
			plainmsg(w, r, plainmsgColorRed, "Failed to parse offset: "+err.Error())
			return
		}
	}
	if counts == "" {
		count = 1000
	} else {
		count, err = strconv.Atoi(counts)
		if err != nil {
			plainmsg(w, r, plainmsgColorRed, "Failed to parse count: "+err.Error())
			return
		}
	}
	type BlockColor struct {
		BlockID          string
		BlockDescription string
		Color            string
	}
	c := map[int]BlockColor{}
	for i, b := range block.StateList {
		if i < offset {
			continue
		}
		if i > offset+count {
			continue
		}
		s := BlockColor{
			b.ID(),
			fmt.Sprintf("%##v", b),
			hexColor(colors[uint32(i)])}
		c[i] = s
	}
	basicLayoutLookupRespond("colors", w, r, map[string]interface{}{"Colors": c, "Offset": offset, "Count": count})
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
	colors[uint32(colorid)] = newColor
	plainmsg(w, r, plainmsgColorGreen, fmt.Sprint("Color ", colorid, " was changed to ", newColor))
	colorsHandlerGET(w, r)
}

var colors []color.RGBA64

func colorsSaveHandler(w http.ResponseWriter, r *http.Request) {
	f, err := os.Create(loadedConfig.Web.ColorsLocation)
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

func loadColors() error {
	b, err := os.ReadFile(loadedConfig.Web.ColorsLocation)
	if err != nil {
		return err
	}
	return gob.NewDecoder(bytes.NewReader(b)).Decode(&colors)
}
