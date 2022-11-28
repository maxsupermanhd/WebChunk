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

	"github.com/Tnze/go-mc/save"
	"github.com/jackc/pgx/v4"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
)

func (s *PostgresChunkStorage) ListWorlds() ([]chunkStorage.SWorld, error) {
	worlds := []chunkStorage.SWorld{}
	rows, err := s.DBPool.Query(context.Background(), `SELECT name, ip, created_at, data FROM worlds`)
	if err != nil {
		if err == pgx.ErrNoRows {
			return worlds, nil
		} else {
			return nil, err
		}
	}
	worlds = make([]chunkStorage.SWorld, rows.CommandTag().RowsAffected())
	n := 0
	for rows.Next() {
		err = rows.Scan(&worlds[n].Name, &worlds[n].Alias, &worlds[n].IP, &worlds[n].CreatedAt, &worlds[n].Data)
		if err != nil {
			return nil, err
		}
		n++
	}
	return worlds, nil
}

func (s *PostgresChunkStorage) ListWorldNames() ([]string, error) {
	var names []string
	derr := s.DBPool.QueryRow(context.Background(), "SELECT array_agg(name) FROM worlds").Scan(&names)
	return names, derr
}

func (s *PostgresChunkStorage) GetWorld(wname string) (*chunkStorage.SWorld, error) {
	world := chunkStorage.SWorld{}
	derr := s.DBPool.QueryRow(context.Background(),
		`SELECT name, ip, created_at, data FROM worlds WHERE name = $1 LIMIT 1`, wname).Scan(&world.Name, &world.Data)
	if derr == pgx.ErrNoRows {
		return nil, nil
	} else if derr == nil {
		return &world, nil
	} else {
		return nil, derr
	}
}

func (s *PostgresChunkStorage) AddWorld(world chunkStorage.SWorld) error {
	tag, derr := s.DBPool.Exec(context.Background(), `INSERT INTO worlds (name, alias, ip, data) VALUES ($1, $2, $3, $4)`, world.Name, world.Alias, world.IP, world.Data)
	if derr != nil || !tag.Insert() || tag.RowsAffected() != 1 {
		return derr
	}
	return nil
}

func (s *PostgresChunkStorage) SetWorldAlias(wname, alias string) error {
	_, derr := s.DBPool.Exec(context.Background(), `UPDATE worlds SET alias = $1 WHERE name = $2`, wname, alias)
	return derr
}

func (s *PostgresChunkStorage) SetWorldIP(wname, ip string) error {
	_, derr := s.DBPool.Exec(context.Background(), `UPDATE worlds SET ip = $1 WHERE name = $2`, wname, ip)
	return derr
}

func (s *PostgresChunkStorage) SetWorldData(wname string, data save.LevelData) error {
	_, derr := s.DBPool.Exec(context.Background(), `UPDATE worlds SET data = $1 WHERE name = $2`, wname, data)
	return derr

}
