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
	"github.com/maxsupermanhd/mcwebchunk/chunkStorage"
)

func (s *PostgresChunkStorage) ListServers() ([]chunkStorage.ServerStruct, error) {
	servers := []chunkStorage.ServerStruct{}
	derr := pgxscan.Select(context.Background(), s.dbpool, &servers,
		`SELECT id, name, ip FROM servers`)
	return servers, derr
}

func (s *PostgresChunkStorage) GetServerByID(sid int) (chunkStorage.ServerStruct, error) {
	server := chunkStorage.ServerStruct{}
	derr := pgxscan.Select(context.Background(), s.dbpool, &server,
		`SELECT id, name, ip FROM servers WHERE id = $1`, sid)
	return server, derr
}

func (s *PostgresChunkStorage) GetServerByName(servername string) (chunkStorage.ServerStruct, error) {
	server := []chunkStorage.ServerStruct{}
	derr := pgxscan.Select(context.Background(), s.dbpool, &server,
		`SELECT * FROM servers WHERE name = $1 LIMIT 1`, servername)
	if len(server) > 0 {
		return server[0], derr
	} else {
		return chunkStorage.ServerStruct{}, derr
	}
}

func (s *PostgresChunkStorage) AddServer(name, ip string) (chunkStorage.ServerStruct, error) {
	server := chunkStorage.ServerStruct{}
	server.IP = ip
	server.Name = name
	derr := s.dbpool.QueryRow(context.Background(),
		`INSERT INTO servers (name, ip) VALUES ($1, $2) RETURNING id`, name, ip).Scan(&server.ID)
	return server, derr
}
