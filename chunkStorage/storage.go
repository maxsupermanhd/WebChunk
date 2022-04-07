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

import "github.com/Tnze/go-mc/save"

type ServerStruct struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip"`
}

type DimStruct struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Alias  string `json:"alias"`
	Server int    `json:"server"`
}

type ChunkData struct {
	X, Z int32
	Data interface{}
}

type ChunkStorage interface {
	ListServers() ([]ServerStruct, error)
	GetServerByID(sid int) (ServerStruct, error)
	GetServerByName(servername string) (ServerStruct, error)
	AddServer(name, ip string) (ServerStruct, error)

	ListDimensionsByServerName(server string) ([]DimStruct, error)
	ListDimensionsByServerID(sid int) ([]DimStruct, error)
	ListDimensions() ([]DimStruct, error)
	GetDimensionByID(did int) (DimStruct, error)
	GetDimensionByNames(server, dimension string) (DimStruct, error)
	AddDimension(server int, name, alias string) (DimStruct, error)
	GetDimensionChunkCountSize(dimensionid int) (count int64, size string, derr error)

	AddChunk(dname, sname string, cx, cz int32, col save.Chunk) error
	GetChunksCount() (uint64, error)
	GetChunksSize() (uint64, error)
	GetChunkData(dname, sname string, cx, cz int) (save.Chunk, error)
	GetChunksRegion(dname, sname string, cx0, cz0, cx1, cz1 int) ([]ChunkData, error)
	GetChunksCountRegion(dname, sname string, cx0, cz0, cx1, cz1 int) ([]ChunkData, error)

	Close() error
}
