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

	"github.com/Tnze/go-mc/save"
	"github.com/jackc/pgx/v4"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
)

func (s *PostgresChunkStorage) ListWorldDimensions(wname string) ([]chunkStorage.SDim, error) {
	dims := []chunkStorage.SDim{}
	rows, derr := s.DBPool.Query(context.Background(), "SELECT name, created_at, data FROM dimensions WHERE world = $1", wname)
	if derr != nil && derr != pgx.ErrNoRows {
		return dims, derr
	}
	defer rows.Close()
	for rows.Next() {
		dim := chunkStorage.SDim{}
		err := rows.Scan(&dim.Name, &dim.CreatedAt, &dim.Data)
		if err != nil {
			return dims, nil
		}
		dims = append(dims, dim)
	}
	return dims, nil
}

func (s *PostgresChunkStorage) ListDimensions() ([]chunkStorage.SDim, error) {
	dims := []chunkStorage.SDim{}
	rows, derr := s.DBPool.Query(context.Background(), "SELECT name, created_at, data FROM dimensions")
	if derr != nil && derr != pgx.ErrNoRows {
		return dims, derr
	}
	defer rows.Close()
	for rows.Next() {
		dim := chunkStorage.SDim{}
		err := rows.Scan(&dim.Name, &dim.CreatedAt, &dim.Data)
		if err != nil {
			return dims, nil
		}
		dims = append(dims, dim)
	}
	return dims, nil

}

func (s *PostgresChunkStorage) AddDimension(wname string, dim chunkStorage.SDim) error {
	_, derr := s.DBPool.Exec(context.Background(), "INSERT INTO dimensions (name, world, data) VALUES ($1, $2, $3)", dim.Name, wname, dim.Data)
	return derr
}

func (s *PostgresChunkStorage) GetDimension(wname, dname string) (*chunkStorage.SDim, error) {
	dim := chunkStorage.SDim{}
	derr := s.DBPool.QueryRow(context.Background(), "SELECT name, world, created_at, data FROM dimensions WHERE name = $1, world = $2", dname, wname).Scan(&dim.Name, &dim.World, &dim.CreatedAt, &dim.World)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, derr
	}
	return &dim, derr
}

func (s *PostgresChunkStorage) SetDimensionData(wname, dname string, data save.DimensionType) error {
	_, derr := s.DBPool.Exec(context.Background(), `UPDATE dimensions SET data = $1 WHERE name = $2, world = $3`, data, dname, wname)
	return derr
}

func (s *PostgresChunkStorage) GetDimensionChunksCount(wname, dname string) (count uint64, derr error) {
	var dimID int
	derr = s.DBPool.QueryRow(context.Background(),
		`SELECT id FROM dimensions WHERE world = $1 and name = $2`, wname, dname).Scan(&dimID)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			derr = fmt.Errorf("world/dimension not found")
		}
		return 0, derr
	}
	derr = s.DBPool.QueryRow(context.Background(),
		`SELECT COUNT(id) FROM chunks WHERE dim = $1`, dimID).Scan(&count)
	return count, derr
}

func (s *PostgresChunkStorage) GetDimensionChunksSize(wname, dname string) (size uint64, derr error) {
	var dimID int
	derr = s.DBPool.QueryRow(context.Background(),
		`SELECT id FROM dimensions WHERE world = $1 and name = $2`, wname, dname).Scan(&dimID)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			derr = fmt.Errorf("world/dimension not found")
		}
		return 0, derr
	}
	derr = s.DBPool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(pg_column_size(data)), 0) FROM chunks WHERE dim = $1`, dimID).Scan(&size)
	return size, derr
}
