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
	"net/http"
	"regexp"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

var (
	serverNameRegexp = regexp.MustCompile(`[\-a-zA-Z0-9.]+`)
	serverIPRegexp   = regexp.MustCompile(`[\-a-zA-Z0-9.]+`)
)

func serverHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	sname := params["server"]
	server, derr := storage.GetServerByName(sname)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			plainmsg(w, r, plainmsgColorRed, "Server not found")
		} else {
			plainmsg(w, r, plainmsgColorRed, "Database query error (server): "+derr.Error())
			return
		}
	}
	dims, derr := storage.ListDimensionsByServerName(sname)
	if derr != nil {
		plainmsg(w, r, plainmsgColorRed, "Database query error (dimensions): "+derr.Error())
		return
	}
	basicLayoutLookupRespond("server", w, r, map[string]interface{}{"Dims": dims, "Server": server})
}

func apiAddServer(w http.ResponseWriter, r *http.Request) (int, string) {
	if r.ParseMultipartForm(0) != nil {
		return 400, "Unable to parse form parameters"
	}
	name := r.FormValue("name")
	if !serverNameRegexp.Match([]byte(name)) {
		return 400, "Invalid server name"
	}
	ip := r.FormValue("ip")
	if !serverIPRegexp.Match([]byte(ip)) {
		return 400, "Invalid server ip"
	}
	server, err := storage.AddServer(name, ip)
	if err != nil {
		return 500, "Failed to add server: " + err.Error()
	}
	setContentTypeJson(w)
	return marshalOrFail(200, server)
}

func apiListServers(w http.ResponseWriter, r *http.Request) (int, string) {
	servers, err := storage.ListServers()
	if err != nil {
		return 500, "Database call failed: " + err.Error()
	}
	setContentTypeJson(w)
	return marshalOrFail(200, servers)
}
