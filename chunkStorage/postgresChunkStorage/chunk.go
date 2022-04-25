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
	"log"

	"github.com/Tnze/go-mc/nbt"
	"github.com/Tnze/go-mc/save"
	"github.com/jackc/pgx/v4"
	"github.com/maxsupermanhd/mcwebchunk/chunkStorage"
)

func (s *PostgresChunkStorage) GetChunk(dname, wname string, cx, cz int) (*save.Chunk, error) {
	var c save.Chunk
	var d []byte
	derr := s.dbpool.QueryRow(context.Background(), `
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
	perr := c.Load(d)
	return &c, perr
}

func (s *PostgresChunkStorage) GetChunksRegion(dname, wname string, cx0, cz0, cx1, cz1 int) ([]chunkStorage.ChunkData, error) {
	// log.Printf("Requesting rectange x%d z%d  ==  x%d z%d", cx0, cz0, cx1, cz1)
	c := []chunkStorage.ChunkData{}
	var dimID int
	err := s.dbpool.QueryRow(context.Background(), `SELECT id FROM dimensions WHERE world = $1 and name = $2`, wname, dname).Scan(&dimID)
	if err != nil {
		if err == pgx.ErrNoRows {
			err = nil
		}
		return c, err
	}
	rows, err := s.dbpool.Query(context.Background(), `
		with grp as
		 (
			select x, z, data, created_at, dim, id,
				rank() over (partition by x, z order by x, z, created_at desc) r
			from chunks where dim = $5
		)
		select data, id
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
		rows.Scan(&d, &cid)
		var cc save.Chunk
		perr = cc.Load(d)
		if perr != nil {
			log.Printf("Chunk %d: %s", cid, perr.Error())
			continue
		}
		c = append(c, chunkStorage.ChunkData{X: cc.XPos, Z: cc.ZPos, Data: cc})
	}
	return c, perr
}

func (s *PostgresChunkStorage) GetChunksCountRegion(wname, dname string, cx0, cz0, cx1, cz1 int) ([]chunkStorage.ChunkData, error) {
	cc := []chunkStorage.ChunkData{}
	rows, derr := s.dbpool.Query(context.Background(), `
	select
	x, z, coalesce(count(*), 0) as c
	from chunks
	where dim = (select dimensions.id 
				 from dimensions 
				 join worlds on worlds.id = dimensions.server 
				 where worlds.name = $5 and dimensions.name = $6) AND
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
		var x, z, c int32
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
	raw, err := nbt.Marshal(col)
	if err != nil {
		return err
	}
	_, err = s.dbpool.Exec(context.Background(), `
			insert into chunks (x, z, data, dim, server)
			values ($1, $2, $3,
				(select dimensions.id 
				 from dimensions 
				 join worlds on worlds.id = dimensions.server 
				 where worlds.name = $4 and dimensions.name = $5),
				 (select id from worlds where name = $4))`,
		col.XPos, col.ZPos, raw, wname, dname)
	return err

}

func (s *PostgresChunkStorage) GetChunksCount() (chunksCount uint64, derr error) {
	derr = s.dbpool.QueryRow(context.Background(),
		`SELECT COUNT(id) from chunks;`).Scan(&chunksCount)
	return chunksCount, derr
}
func (s *PostgresChunkStorage) GetChunksSize() (chunksSize uint64, derr error) {
	derr = s.dbpool.QueryRow(context.Background(),
		`SELECT pg_total_relation_size('chunks');`).Scan(&chunksSize)
	return chunksSize, derr
}
