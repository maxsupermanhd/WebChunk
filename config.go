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
	"os"

	"github.com/maxsupermanhd/lac"
)

var cfg = lac.NewConf()

//lint:ignore U1000 for future use
func saveConfig() error {
	path := os.Getenv("WEBCHUNK_CONFIG")
	if path == "" {
		path = "config.json"
	}
	return cfg.ToFileIndentJSON(path, 0644)
}

func loadConfig() error {
	path := os.Getenv("WEBCHUNK_CONFIG")
	if path == "" {
		path = "config.json"
	}
	return cfg.SetFromFileJSON(path)
}

func cfgHandler(w http.ResponseWriter, r *http.Request) {
	b, err := cfg.ToBytesIndentJSON()
	if err != nil {
		templateRespond("plainmsg", w, r, map[string]any{"msg": err.Error()})
		return
	}
	templateRespond("cfg", w, r, map[string]any{"cfg": string(b)})
}

func apiSaveConfig(_ http.ResponseWriter, _ *http.Request) (int, string) {
	err := saveConfig()
	if err != nil {
		return 500, err.Error()
	}
	return 200, ""
}
