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

package postgresChunkStorage

import (
	"context"
	"fmt"

	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
)

func (s *PostgresChunkStorage) ListWorldDimensions(world string) ([]chunkStorage.DimStruct, error) {
	dims := []chunkStorage.DimStruct{}
	derr := pgxscan.Select(context.Background(), s.dbpool, &dims,
		`SELECT name, alias, world FROM dimensions WHERE world = $1`, world)
	if derr == pgx.ErrNoRows {
		derr = nil
	}
	return dims, derr
}

// func (s *PostgresChunkStorage) ListDimensionsByWorldID(wid int) ([]chunkStorage.DimStruct, error) {
// 	dims := []chunkStorage.DimStruct{}
// 	derr := pgxscan.Select(context.Background(), s.dbpool, &dims,
// 		`SELECT id, name, alias, world FROM dimensions WHERE world = $1`, wid)
// 	if derr == pgx.ErrNoRows {
// 		derr = nil
// 	}
// 	return dims, derr
// }

func (s *PostgresChunkStorage) ListDimensions() ([]chunkStorage.DimStruct, error) {
	dims := []chunkStorage.DimStruct{}
	derr := pgxscan.Select(context.Background(), s.dbpool, &dims,
		`SELECT name, alias, world FROM dimensions`)
	if derr == pgx.ErrNoRows {
		derr = nil
	}
	return dims, derr
}

// func (s *PostgresChunkStorage) GetDimensionByID(did int) (*chunkStorage.DimStruct, error) {
// 	dim := chunkStorage.DimStruct{}
// 	derr := pgxscan.Select(context.Background(), s.dbpool, &dim,
// 		`SELECT id, name, alias, world FROM dimensions WHERE id = $1`, did)
// 	if derr == pgx.ErrNoRows {
// 		return nil, nil
// 	}
// 	return &dim, derr
// }

func (s *PostgresChunkStorage) GetDimension(world, dimension string) (*chunkStorage.DimStruct, error) {
	dim := chunkStorage.DimStruct{}
	derr := s.dbpool.QueryRow(context.Background(),
		`SELECT name, alias, world, spawnpoint, miny, maxy FROM dimensions WHERE name = $1 AND world = $2`, dimension, world).
		Scan(&dim.Name, &dim.Alias, &dim.World, &dim.Spawnpoint, &dim.LowestY, &dim.BuildLimit)
	if derr == pgx.ErrNoRows {
		return nil, nil
	}
	return &dim, derr
}

func (s *PostgresChunkStorage) AddDimension(dim chunkStorage.DimStruct) (*chunkStorage.DimStruct, error) {
	_, derr := s.dbpool.Exec(context.Background(),
		`INSERT INTO dimensions (world, name, alias, spawnpoint, miny, maxy) VALUES ($1, $2, $3, $4, $5, $6)`, dim.World, dim.Name, dim.Alias, dim.Spawnpoint, dim.LowestY, dim.BuildLimit)
	return &dim, derr
}

func (s *PostgresChunkStorage) GetDimensionChunksCount(wname, dname string) (count uint64, derr error) {
	var dimID int
	derr = s.dbpool.QueryRow(context.Background(),
		`SELECT id FROM dimensions WHERE world = $1 and name = $2`, wname, dname).Scan(&dimID)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			derr = fmt.Errorf("world/dimension not found")
		}
		return 0, derr
	}
	derr = s.dbpool.QueryRow(context.Background(),
		`SELECT COUNT(id) FROM chunks WHERE dim = $1`, dimID).Scan(&count)
	return count, derr
}

func (s *PostgresChunkStorage) GetDimensionChunksSize(wname, dname string) (size uint64, derr error) {
	var dimID int
	derr = s.dbpool.QueryRow(context.Background(),
		`SELECT id FROM dimensions WHERE world = $1 and name = $2`, wname, dname).Scan(&dimID)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			derr = fmt.Errorf("world/dimension not found")
		}
		return 0, derr
	}
	derr = s.dbpool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(pg_column_size(data)), 0) FROM chunks WHERE dim = $1`, dimID).Scan(&size)
	return size, derr
}
