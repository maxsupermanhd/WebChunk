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
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/maxsupermanhd/mcwebchunk/chunkStorage"
	"github.com/maxsupermanhd/mcwebchunk/chunkStorage/postgresChunkStorage"
)

type storagesJSON struct {
	Storages []chunkStorage.Storage `json:"storages"`
}

func saveStorages(path string, s []chunkStorage.Storage) error {
	d, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, d, 0666)
}

func closeStorages(s []chunkStorage.Storage) {
	for _, s2 := range s {
		s2.Driver.Close()
	}
}

func loadStorages(path string) ([]chunkStorage.Storage, error) {
	s := storagesJSON{}
	f, err := os.ReadFile(path)
	if err != nil {
		return s.Storages, err
	}
	err = json.Unmarshal(f, &s)
	if err != nil {
		return s.Storages, err
	}
	for i := range s.Storages {
		switch {
		case s.Storages[i].Type == "postgres":
			s.Storages[i].Driver, err = postgresChunkStorage.NewPostgresChunkStorage(context.Background(), s.Storages[i].Address)
			if err != nil {
				log.Printf("Failed to initialize postgres storage %s: %s\n", s.Storages[i].Name, err.Error())
				s.Storages[i].Driver = nil
			}
		default:
			log.Printf("Storage type [%s] not implemented!\n", s.Storages[i].Type)
		}
	}
	return s.Storages, nil
}
