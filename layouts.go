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
	"log"
	"net/http"
)

const (
	plainmsgColorRed = iota
	plainmsgColorGreen
)

func plainmsg(w http.ResponseWriter, r *http.Request, color int, msg string) {
	basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{
		"msgred":   color == plainmsgColorRed,
		"msggreen": color == plainmsgColorGreen,
		"msg":      msg})
}

func basicLayoutLookupRespond(page string, w http.ResponseWriter, r *http.Request, m map[string]interface{}) {
	in := layouts.Lookup(page)
	if in != nil {
		m["NavWhere"] = page
		// sessionAppendUser(r, &m)
		w.Header().Set("Server", "WebChunk webserver "+CommitHash)
		w.Header().Set("Cache-Control", "no-cache")
		err := in.Execute(w, m)
		if err != nil {
			log.Println(err)
		}
	} else {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
}
