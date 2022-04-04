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

func (s *PostgresChunkStorage) ListDimensionsByServerName(server string) ([]chunkStorage.DimStruct, error) {
	var dims []chunkStorage.DimStruct
	derr := pgxscan.Select(context.Background(), s.dbpool, &dims,
		`SELECT dimensions.id, dimensions.name, dimensions.alias, server FROM dimensions JOIN SERVERS ON dimensions.server = servers.id WHERE servers.name = $1`, server)
	return dims, derr
}

func (s *PostgresChunkStorage) ListDimensionsByServerID(sid int) ([]chunkStorage.DimStruct, error) {
	var dims []chunkStorage.DimStruct
	derr := pgxscan.Select(context.Background(), s.dbpool, &dims,
		`SELECT id, name, alias, server FROM dimensions WHERE server = $1`, sid)
	return dims, derr
}

func (s *PostgresChunkStorage) ListDimensions() ([]chunkStorage.DimStruct, error) {
	var dims []chunkStorage.DimStruct
	derr := pgxscan.Select(context.Background(), s.dbpool, &dims,
		`SELECT id, name, alias, server FROM dimensions`)
	return dims, derr
}

//lint:ignore U1000 for future use
func (s *PostgresChunkStorage) GetDimensionByID(did int) (chunkStorage.DimStruct, error) {
	var dim chunkStorage.DimStruct
	derr := pgxscan.Select(context.Background(), s.dbpool, &dim,
		`SELECT id, name, alias, server FROM dimensions WHERE id = $1`, did)
	return dim, derr
}

func (s *PostgresChunkStorage) GetDimensionByNames(server, dimension string) (chunkStorage.DimStruct, error) {
	var dim chunkStorage.DimStruct
	derr := pgxscan.Get(context.Background(), s.dbpool, &dim, `
		SELECT dimensions.id, dimensions.name, dimensions.alias, dimensions.server FROM dimensions
			JOIN SERVERS ON dimensions.server = servers.id
			WHERE dimensions.name = $1 AND servers.name = $2
			LIMIT 1`, dimension, server)
	return dim, derr
}

func (s *PostgresChunkStorage) AddDimension(server int, name, alias string) (chunkStorage.DimStruct, error) {
	var dim chunkStorage.DimStruct
	derr := s.dbpool.QueryRow(context.Background(),
		`INSERT INTO dimensions (server, name, alias) VALUES ($1, $2, $3) RETURNING id`, server, name, alias).Scan(&dim.ID)
	dim.Alias = alias
	dim.Name = name
	dim.Server = server
	return dim, derr
}

func (s *PostgresChunkStorage) GetDimensionChunkCountSize(dimensionid int) (count int64, size string, derr error) {
	derr = s.dbpool.QueryRow(context.Background(),
		`SELECT COUNT(id), COALESCE(pg_size_pretty(SUM(pg_column_size(data))), '0 kB') FROM chunks WHERE dim = $1`, dimensionid).Scan(&count, &size)
	return count, size, derr

}
