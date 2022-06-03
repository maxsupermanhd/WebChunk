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
	"os"
	"sync"

	"github.com/maxsupermanhd/WebChunk/chunkStorage"
	"github.com/maxsupermanhd/WebChunk/proxy"
)

var loadedConfig WebChunkConfig
var loadedConfigMutex sync.Mutex

type ProxyRoute struct {
	Address   string `json:"address"`
	World     string `json:"world"`
	Dimension string `json:"dimension"`
	Storage   string `json:"storage"`
}

type WebChunkConfig struct {
	LogsLocation string                 `json:"logs_location"`
	Storages     []chunkStorage.Storage `json:"storages"`
	Web          struct {
		LayoutsLocation string `json:"layouts_location"`
		LayoutsGlob     string `json:"layouts_glob"`
		Listen          string `json:"listen"`
		ColorsLocation  string `json:"color_pallete"`
	} `json:"web"`
	API struct {
		CreateWorlds        bool   `json:"create_worlds"`
		CreateDimensions    bool   `json:"create_dimensions"`
		FallbackStorageName string `json:"fallback_storage_name"`
	} `json:"api"`
	Proxy  proxy.ProxyConfig     `json:"proxy"`
	Routes map[string]ProxyRoute `json:"proxy_routing"`
	// Reconstructor viewer.ReconstructorConfig `json:"reconstructor"`
}

func ProxyRoutesHandler(username string) string {
	route, ok := loadedConfig.Routes[username]
	if !ok {
		return ""
	}
	return route.Address
}

func saveConfig() error {
	path := os.Getenv("WEBCHUNK_CONFIG")
	if path == "" {
		path = "config.json"
	}
	b, err := json.MarshalIndent(loadedConfig, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0664)
}

func loadConfig() error {
	path := os.Getenv("WEBCHUNK_CONFIG")
	if path == "" {
		path = "config.json"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	loadedConfigMutex.Lock()
	defer loadedConfigMutex.Unlock()
	return json.Unmarshal(b, &loadedConfig)
}
