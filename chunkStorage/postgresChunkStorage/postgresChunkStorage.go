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

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
)

type PostgresChunkStorage struct {
	dbpool *pgxpool.Pool
}

func NewPostgresChunkStorage(ctx context.Context, connection string) (*PostgresChunkStorage, error) {
	p, err := pgxpool.Connect(context.Background(), connection)
	if err != nil {
		return nil, err
	}
	return &PostgresChunkStorage{dbpool: p}, nil
}

func (s *PostgresChunkStorage) Close() error {
	s.dbpool.Close()
	return nil
}

func (s *PostgresChunkStorage) GetAbilities() chunkStorage.StorageAbilities {
	return chunkStorage.StorageAbilities{
		CanCreateWorldsDimensions: true,
		CanAddChunks:              true,
		CanPreserveOldChunks:      true,
	}
}
