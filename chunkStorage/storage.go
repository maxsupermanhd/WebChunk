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

package chunkStorage

import (
	"fmt"
	"log"

	"github.com/Tnze/go-mc/save"
)

type WorldStruct struct {
	Name string `json:"name"` // unique
	IP   string `json:"ip"`
}

type DimStruct struct {
	Name       string   `json:"name"` // unique per world
	Alias      string   `json:"alias"`
	World      string   `json:"world"`
	Spawnpoint [3]int64 `json:"spawn"`
	LowestY    int      `json:"miny"`
	BuildLimit int      `json:"maxy"`
}

type ChunkData struct {
	X, Z int32
	Data interface{}
}

type StorageAbilities struct {
	CanCreateWorldsDimensions bool
	CanAddChunks              bool
	CanPreserveOldChunks      bool
}

// Everything returns empty slice/nil if specified
// object is not found, error only in case of listing/requesting
// or other abnormal things.
type ChunkStorage interface {
	GetAbilities() StorageAbilities

	ListWorlds() ([]WorldStruct, error)
	GetWorld(wname string) (*WorldStruct, error)
	AddWorld(name, ip string) (*WorldStruct, error)
	GetChunksCount() (uint64, error)
	GetChunksSize() (uint64, error)

	ListWorldDimensions(wname string) ([]DimStruct, error)
	ListDimensions() ([]DimStruct, error)
	GetDimension(wname, dname string) (*DimStruct, error)
	AddDimension(DimStruct) (*DimStruct, error)
	GetDimensionChunksCount(wname, dname string) (uint64, error)
	GetDimensionChunksSize(wname, dname string) (uint64, error)

	AddChunk(wname, dname string, cx, cz int, col save.Chunk) error
	GetChunk(wname, dname string, cx, cz int) (*save.Chunk, error)
	GetChunksRegion(wname, dname string, cx0, cz0, cx1, cz1 int) ([]ChunkData, error)
	GetChunksCountRegion(wname, dname string, cx0, cz0, cx1, cz1 int) ([]ChunkData, error)

	Close() error
}

type Storage struct {
	Name    string       `json:"name"`
	Type    string       `json:"type"`
	Address string       `json:"addr"`
	Driver  ChunkStorage `json:"-"`
}

func ListWorlds(storages []Storage) []WorldStruct {
	worlds := []WorldStruct{}
	for _, s := range storages {
		if s.Driver != nil {
			w, err := s.Driver.ListWorlds()
			if err != nil {
				log.Printf("Failed to list worlds on storage %s: %s", s.Name, err.Error())
			}
			worlds = append(worlds, w...)
		}
	}
	return worlds
}

func ListDimensions(storages []Storage, wname string) ([]DimStruct, error) {
	dims := []DimStruct{}
	if wname == "" {
		for _, s := range storages {
			if s.Driver != nil {
				d, err := s.Driver.ListDimensions()
				if err != nil {
					log.Printf("Failed to list dims on storage %s: %s", s.Name, err.Error())
				}
				dims = append(dims, d...)
			}
		}
	} else {
		_, s, err := GetWorldStorage(storages, wname)
		if err != nil {
			return dims, err
		}
		if s == nil {
			return dims, fmt.Errorf("world storage not found")
		}
		dims, err = s.ListWorldDimensions(wname)
		if err != nil {
			return dims, err
		}
	}
	return dims, nil
}

func GetWorldStorage(storages []Storage, wname string) (*WorldStruct, ChunkStorage, error) {
	for _, s := range storages {
		if s.Driver != nil {
			w, err := s.Driver.GetWorld(wname)
			if err != nil {
				return nil, nil, err
			}
			if w != nil {
				return w, s.Driver, nil
			}
		}
	}
	return nil, nil, nil
}
