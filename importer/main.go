package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Tnze/go-mc/save"
	"github.com/Tnze/go-mc/save/region"
	"github.com/jackc/pgx/v4"
	"github.com/joho/godotenv"
)

func main() {
	basedir := "/home/max/p/worlddownloader/simplyspawn_logo/region/"
	de, err := os.ReadDir(basedir)
	if err != nil {
		log.Fatal(err)
	}
	err = godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}
	conn, err := pgx.Connect(context.Background(), os.Getenv("DB"))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close(context.Background())
	for _, d := range de {
		p := filepath.Join(basedir, d.Name())
		var rx, rz int
		if _, err := fmt.Sscanf(d.Name(), "r.%d.%d.mca", &rx, &rz); err != nil {
			log.Printf("Error parsing file name of %s: %v, ignoring", d.Name(), err)
			continue
		}
		r, err := region.Open(p)
		if err != nil {
			log.Printf("Error when opening %s: %v, ignoring", d.Name(), err)
			continue
		}
		log.Printf("Sending region %d %d", rx, rz)
		for x := 0; x < 32; x++ {
			for z := 0; z < 32; z++ {
				if !r.ExistSector(x, z) {
					continue
				}
				data, err := r.ReadSector(x, z)
				if err != nil {
					log.Printf("Read sector (%d.%d) error: %v", x, z, err)
				}
				var col save.Column
				col.Load(data)
				// log.Printf("Chunk %d %d", col.Level.PosX, col.Level.PosZ)
				tag, err := conn.Exec(context.Background(), `
					insert into chunks (dim, x, z, data) values (1, $1, $2, $3)`, rx*32+x, rz*32+z, data)
				if err != nil {
					log.Print(err.Error())
				}
				if tag.RowsAffected() != 1 {
					log.Print("Rows affected ", tag.RowsAffected())
				}
			}
		}
		if err := r.Close(); err != nil {
			log.Printf("Close r.%d.%d.mca error: %v", rx, rz, err)
		}
	}
}
