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
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
)

var (
	worldNameRegexp = regexp.MustCompile(`[\-a-zA-Z0-9.]+`)
	worldIPRegexp   = regexp.MustCompile(`[\-a-zA-Z0-9.]+`)
)

func worldHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	wname := params["world"]
	world, s, err := chunkStorage.GetWorldStorage(storages, wname)
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Error looking up world storage: "+err.Error())
		return
	}
	if s == nil || world == nil {
		plainmsg(w, r, plainmsgColorRed, "World not found")
		return
	}
	dims, err := s.ListWorldDimensions(wname)
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Error getting dimensions from world: "+err.Error())
		return
	}
	basicLayoutLookupRespond("world", w, r, map[string]interface{}{"Dims": dims, "World": world})
}

func apiAddWorld(w http.ResponseWriter, r *http.Request) (int, string) {
	if r.ParseMultipartForm(0) != nil {
		return 400, "Unable to parse form parameters"
	}
	name := r.FormValue("name")
	if !worldNameRegexp.Match([]byte(name)) {
		return 400, "Invalid world name"
	}
	ip := r.FormValue("ip")
	if !worldIPRegexp.Match([]byte(ip)) {
		return 400, "Invalid world ip"
	}
	sname := r.FormValue("storage")
	var driver chunkStorage.ChunkStorage
	driver = nil
	for _, s := range storages {
		if sname == s.Name {
			driver = s.Driver
		}
	}
	if driver == nil {
		return 500, "Storage not found or not initialized"
	}
	world, err := driver.AddWorld(name, ip)
	if err != nil {
		return 500, "Failed to add world: " + err.Error()
	}
	setContentTypeJson(w)
	return marshalOrFail(200, world)
}

func apiListWorlds(w http.ResponseWriter, r *http.Request) (int, string) {
	worlds := chunkStorage.ListWorlds(storages)
	setContentTypeJson(w)
	return marshalOrFail(200, worlds)
}
