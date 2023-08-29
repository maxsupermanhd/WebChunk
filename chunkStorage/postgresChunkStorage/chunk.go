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
	"errors"
	"log"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
	"github.com/maxsupermanhd/go-vmc/v762/save"
)

func (s *PostgresChunkStorage) GetChunk(wname, dname string, cx, cz int) (*save.Chunk, error) {
	d, err := s.GetChunkRaw(wname, dname, cx, cz)
	if err != nil {
		return nil, err
	}
	var c save.Chunk
	if len(d) > 1 {
		err = c.Load(d)
	} else {
		err = errors.New("data is zero length")
	}
	return &c, err
}

func (s *PostgresChunkStorage) GetChunkRaw(wname, dname string, cx, cz int) ([]byte, error) {
	var d []byte
	derr := s.DBPool.QueryRow(context.Background(), `
		select data
		from chunks
		where x = $1 AND z = $2 AND
			dim = (select dimensions.id 
			 from dimensions
			 where dimensions.world = $3 and dimensions.name = $4)
		order by created_at desc
		limit 1;`, cx, cz, wname, dname).Scan(&d)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			derr = nil
		} else {
			log.Print(derr.Error())
		}
		return nil, derr
	}
	return d, derr
}

func ConnGetChunkByDID(conn *pgxpool.Conn, did int, cx, cz int) (*save.Chunk, error) {
	d, err := ConnGetChunkRawByDID(conn, did, cx, cz)
	if err != nil {
		return nil, err
	}
	var c save.Chunk
	if len(d) > 1 {
		err = c.Load(d)
	} else {
		err = errors.New("data is zero length")
	}
	return &c, err
}

func ConnGetChunkRawByDID(conn *pgxpool.Conn, did int, cx, cz int) ([]byte, error) {
	var d []byte
	derr := conn.QueryRow(context.Background(), `
		select data
		from chunks
		where x = $1 AND z = $2 AND dim = $3
		order by created_at desc
		limit 1;`, cx, cz, did).Scan(&d)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			derr = nil
		} else {
			log.Print(derr.Error())
		}
		return nil, derr
	}
	return d, derr
}

func (s *PostgresChunkStorage) GetChunkByDID(did int, cx, cz int) (*save.Chunk, error) {
	d, err := s.GetChunkRawByDID(did, cx, cz)
	if err != nil {
		return nil, err
	}
	var c save.Chunk
	if len(d) > 1 {
		err = c.Load(d)
	} else {
		err = errors.New("data is zero length")
	}
	return &c, err
}

func (s *PostgresChunkStorage) GetChunkRawByDID(did int, cx, cz int) ([]byte, error) {
	var d []byte
	derr := s.DBPool.QueryRow(context.Background(), `
		select data
		from chunks
		where x = $1 AND z = $2 AND dim = $3
		order by created_at desc
		limit 1;`, cx, cz, did).Scan(&d)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			derr = nil
		} else {
			log.Print(derr.Error())
		}
		return nil, derr
	}
	return d, derr
}

func (s *PostgresChunkStorage) GetChunksRegion(wname, dname string, cx0, cz0, cx1, cz1 int) ([]chunkStorage.ChunkData, error) {
	ar, err := s.GetChunksRegionRaw(wname, dname, cx0, cz0, cx1, cz1)
	if err != nil {
		return ar, err
	}
	ret := []chunkStorage.ChunkData{}
	for i := range ar {
		dat, ok := ar[i].Data.([]byte)
		if !ok {
			log.Printf("GetChunksRegionRaw returned something that is not a []byte, chunk x%d z%d", ar[i].X, ar[i].Z)
			continue
		}
		c, err := chunkStorage.ConvFlexibleNBTtoSave(dat)
		if err != nil {
			log.Printf("Failed to parse chunk data (%s), chunk x%d z%d", err.Error(), ar[i].X, ar[i].Z)
			continue
		}
		ret = append(ret, chunkStorage.ChunkData{
			X:    ar[i].X,
			Z:    ar[i].Z,
			Data: *c,
		})
	}
	return ret, err
}

func (s *PostgresChunkStorage) GetChunksRegionRaw(wname, dname string, cx0, cz0, cx1, cz1 int) ([]chunkStorage.ChunkData, error) {
	c := []chunkStorage.ChunkData{}
	var dimID int
	err := s.DBPool.QueryRow(context.Background(), `SELECT id FROM dimensions WHERE world = $1 and name = $2`, wname, dname).Scan(&dimID)
	if err != nil {
		if err == pgx.ErrNoRows {
			err = nil
		}
		return c, err
	}
	rows, err := s.DBPool.Query(context.Background(), `
		with grp as
		 (
			select x, z, data, created_at, dim, id,
				rank() over (partition by x, z order by x, z, created_at desc) r
			from chunks where dim = $5
		)
		select x, z, data, id
		from grp
		where x >= $1 AND z >= $2 AND x < $3 AND z < $4 AND r = 1 AND dim = $5
		`, cx0, cz0, cx1, cz1, dimID)
	if err != nil {
		if err == pgx.ErrNoRows {
			err = nil
		} else {
			log.Print(err.Error())
		}
		return c, err
	}
	var perr error
	for rows.Next() {
		var d []byte
		var cid int
		var x int
		var z int
		err = rows.Scan(&x, &z, &d, &cid)
		if err != nil {
			return c, err
		}
		c = append(c, chunkStorage.ChunkData{X: x, Z: z, Data: d})
	}
	return c, perr
}

func (s *PostgresChunkStorage) GetChunksCountRegion(wname, dname string, cx0, cz0, cx1, cz1 int) ([]chunkStorage.ChunkData, error) {
	cc := []chunkStorage.ChunkData{}
	rows, derr := s.DBPool.Query(context.Background(), `
	select
	x, z, coalesce(count(*), 0) as c
	from chunks
	where dim = (select dimensions.id from dimensions
				 where dimensions.world = $5 and dimensions.name = $6) AND
		  x >= $1 AND z >= $2 AND x < $3 AND z < $4
	group by x, z
	order by c desc
		`, cx0, cz0, cx1, cz1, wname, dname)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			derr = nil
		} else {
			log.Print(derr.Error())
		}
		return cc, derr
	}
	for rows.Next() {
		var x, z, c int
		derr := rows.Scan(&x, &z, &c)
		if derr != nil {
			log.Print(derr.Error())
			continue
		}
		cc = append(cc, chunkStorage.ChunkData{X: x, Z: z, Data: c})
	}
	return cc, derr
}

func (s *PostgresChunkStorage) AddChunk(wname, dname string, cx, cz int, col save.Chunk) error {
	b, err := col.Data(1)
	if err != nil {
		log.Printf("Error marshling: %s", err.Error())
		return err
	}
	return s.AddChunkRaw(wname, dname, cx, cz, b)
}

func (s *PostgresChunkStorage) AddChunkRaw(wname, dname string, cx, cz int, dat []byte) error {
	_, err := s.DBPool.Exec(context.Background(), `
			insert into chunks (x, z, data, dim)
			values ($1, $2, $3,
				(select dimensions.id from dimensions
				 where dimensions.world = $4 and dimensions.name = $5))`,
		cx, cz, dat, wname, dname)
	return err
}

func (s *PostgresChunkStorage) GetChunkModDate(wname, dname string, cx, cz int) (*time.Time, error) {
	var t time.Time
	err := s.DBPool.QueryRow(context.Background(), `
		SELECT created_at FROM chunks
		WHERE x = $1 AND z = $2 AND dim = (select dimensions.id from dimensions where dimensions.world = $3 and dimensions.name = $4)
		ORDER BY created_at DESC
		LIMIT 1`, cx, cz, wname, dname).Scan(&t)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}
