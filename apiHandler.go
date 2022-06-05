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
	"encoding/json"
	"log"
	"net/http"
)

func apiHandle(f func(http.ResponseWriter, *http.Request) (int, string)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		code, content := f(w, r)
		if code == 500 && loadedConfig.API.LogErrors {
			log.Println("500 error code: " + content)
		}
		w.Header().Set("Server", "WebChunk webserver "+CommitHash)
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(code)
		w.Write([]byte(content))
	}
}

func marshalOrFail(code int, content interface{}) (int, string) {
	resp, err := json.Marshal(content)
	if err != nil && loadedConfig.API.LogErrors {
		log.Println("JSON serialization failed: " + err.Error())
		return 500, "JSON serialization failed: " + err.Error()
	}
	return code, string(resp) + "\n"
}

func setContentTypeJson(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}
