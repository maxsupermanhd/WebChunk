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
	"strconv"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
	"github.com/maxsupermanhd/mcwebchunk/chunkStorage"
)

var (
	dimNameRegexp  = regexp.MustCompile(`[\-a-zA-Z0-9.]+`)
	dimAliasRegexp = dimNameRegexp
)

func dimensionHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	sname := params["server"]
	dimname := params["dim"]
	server, derr := storage.GetServerByName(sname)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			plainmsg(w, r, plainmsgColorRed, "Server not found")
		} else {
			plainmsg(w, r, plainmsgColorRed, "Database query error: "+derr.Error())
		}
		return
	}
	dim, derr := storage.GetDimensionByNames(sname, dimname)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			plainmsg(w, r, plainmsgColorRed, "Dimension not found")
		} else {
			plainmsg(w, r, plainmsgColorRed, "Database query error: "+derr.Error())
		}
		return
	}
	basicLayoutLookupRespond("dim", w, r, map[string]interface{}{"Dim": dim, "Server": server})
}

func apiAddDimension(w http.ResponseWriter, r *http.Request) (int, string) {
	if r.ParseMultipartForm(0) != nil {
		return 400, "Unable to parse form parameters"
	}
	name := r.FormValue("name")
	if !dimNameRegexp.Match([]byte(name)) {
		return 400, "Invalid dimension name"
	}
	alias := r.FormValue("alias")
	if !dimAliasRegexp.Match([]byte(alias)) {
		return 400, "Invalid dimension alias"
	}
	serverid, err := strconv.Atoi(r.FormValue("server"))
	if err != nil {
		return 400, "Invalid dimension server id"
	}
	_, err = storage.GetServerByID(serverid)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 404, "Server not found"
		}
	}
	dim, err := storage.AddDimension(serverid, name, alias)
	if err != nil {
		return 500, "Failed to add server: " + err.Error()
	}
	setContentTypeJson(w)
	return marshalOrFail(200, dim)
}

func apiListDimensions(w http.ResponseWriter, r *http.Request) (int, string) {
	if r.ParseForm() != nil {
		return 400, "Unable to parse form parameters"
	}
	server := r.Form.Get("server")
	var dims []chunkStorage.DimStruct
	var err error
	if server == "" {
		dims, err = storage.ListDimensions()
		if err != nil {
			return 500, "Database call failed: " + err.Error()
		}
	} else {
		serverid, err := strconv.Atoi(server)
		if err != nil {
			return 400, "Invalid dimension server id"
		}
		dims, err = storage.ListDimensionsByServerID(serverid)
		if err != nil {
			return 500, "Database call failed: " + err.Error()
		}
	}
	setContentTypeJson(w)
	return marshalOrFail(200, dims)
}
