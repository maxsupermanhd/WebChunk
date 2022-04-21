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
	"github.com/Tnze/go-mc/save"
)

type WorldStruct struct {
	Name string `json:"name"` // unique
	IP   string `json:"ip"`
}

type DimStruct struct {
	Name  string `json:"name"` // unique per world
	Alias string `json:"alias"`
	World string `json:"world"`
}

type ChunkData struct {
	X, Z int32
	Data interface{}
}

type ChunkStorage interface {
	ListWorlds() ([]WorldStruct, error)
	GetWorld(wname string) (*WorldStruct, error)
	AddWorld(name, ip string) (*WorldStruct, error)
	GetChunksCount() (uint64, error)
	GetChunksSize() (uint64, error)

	ListWorldDimensions(wname string) ([]DimStruct, error)
	ListDimensions() ([]DimStruct, error)
	GetDimension(wname, dname string) (*DimStruct, error)
	AddDimension(wname, name, alias string) (*DimStruct, error)
	GetDimensionChunksCount(wname, dname string) (uint64, error)
	GetDimensionChunksSize(wname, dname string) (uint64, error)

	AddChunk(wname, dname string, cx, cz int, col save.Chunk) error
	GetChunk(wname, dname string, cx, cz int) (*save.Chunk, error)
	GetChunksRegion(wname, dname string, cx0, cz0, cx1, cz1 int) ([]ChunkData, error)
	GetChunksCountRegion(wname, dname string, cx0, cz0, cx1, cz1 int) ([]ChunkData, error)

	Close() error
}
