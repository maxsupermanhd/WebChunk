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

	"github.com/georgysavva/scany/pgxscan"
	"github.com/jackc/pgx/v4"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
)

func (s *PostgresChunkStorage) ListWorlds() ([]chunkStorage.WorldStruct, error) {
	worlds := []chunkStorage.WorldStruct{}
	derr := pgxscan.Select(context.Background(), s.DBPool, &worlds,
		`SELECT name, ip FROM worlds`)
	if derr == pgx.ErrNoRows {
		return nil, nil
	}
	return worlds, derr
}

// func (s *PostgresChunkStorage) GetWorldByID(sid int) (*chunkStorage.WorldStruct, error) {
// 	world := chunkStorage.WorldStruct{}
// 	derr := pgxscan.Select(context.Background(), s.dbpool, &world,
// 		`SELECT id, name, ip FROM worlds WHERE id = $1`, sid)
// 	if derr == pgx.ErrNoRows {
// 		return nil, nil
// 	}
// 	return &world, derr
// }

func (s *PostgresChunkStorage) GetWorld(wname string) (*chunkStorage.WorldStruct, error) {
	world := chunkStorage.WorldStruct{}
	derr := s.DBPool.QueryRow(context.Background(),
		`SELECT name, ip FROM worlds WHERE name = $1 LIMIT 1`, wname).Scan(&world.Name, &world.IP)
	if derr == pgx.ErrNoRows {
		return nil, nil
	} else if derr == nil {
		return &world, nil
	} else {
		return nil, derr
	}
}

func (s *PostgresChunkStorage) AddWorld(name, ip string) (*chunkStorage.WorldStruct, error) {
	world := chunkStorage.WorldStruct{}
	world.IP = ip
	world.Name = name
	_, derr := s.DBPool.Exec(context.Background(), `INSERT INTO worlds (name, ip) VALUES ($1, $2)`, name, ip)
	return &world, derr
}
