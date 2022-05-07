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

	"github.com/maxsupermanhd/WebChunk/chunkStorage"
	"github.com/maxsupermanhd/WebChunk/proxy"
)

func closeStorages(s []chunkStorage.Storage) {
	for _, s2 := range s {
		if s2.Driver != nil {
			s2.Driver.Close()
		}
	}
}

func chunkConsumer(c chan *proxy.ProxiedChunk) {
	for r := range c {
		route, ok := loadedConfig.Routes[r.Username]
		if !ok {
			log.Printf("Got UNKNOWN chunk [%v] from [%v] by [%v]", r.Pos, r.Server, r.Username)
		}
		log.Printf("Got chunk [%v] from [%v] by [%v]", r.Pos, r.Server, r.Username)
		if route.World == "" {
			route.World = r.Server
		}
		if route.Dimension == "" {
			route.Dimension = r.Dimension
		}
		w, s, err := chunkStorage.GetWorldStorage(storages, route.World)
		if err != nil {
			log.Println("Failed to lookup world storage: ", err)
			break
		}
		if w == nil || s == nil {
			continue
		}
	}
}
