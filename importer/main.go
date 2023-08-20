package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	// "github.com/maxsupermanhd/go-vmc/v762/save"
	"github.com/jackc/pgx/v4"
	"github.com/joho/godotenv"
	"github.com/maxsupermanhd/go-vmc/v762/save"
	"github.com/maxsupermanhd/go-vmc/v762/save/region"
)

var (
	basedir = "/home/max/p/worlddownloader/simplyspawn_logo/region/"
)

func main() {
	log.Print("Reading dir")
	de, err := os.ReadDir(basedir)
	if err != nil {
		log.Fatal(err)
	}
	log.Print("Loading env")
	err = godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}
	sendThreadedHttpChunks(de)
}

func sendThreadedHttpRegions(de []fs.DirEntry) {
	const threadscount = 8

	var wg sync.WaitGroup
	wg.Add(threadscount)
	c := make(chan []byte, threadscount*2)
	log.Print("Launching ", threadscount, " threads")

	for i := 0; i < threadscount; i++ {
		go func(chan []byte) {
			for data := range c {
				res, err := http.Post("https://spring-forge-eg-answered.trycloudflare.com/api/submit/region/1", "binary/octet-stream", bytes.NewReader(data))
				if err != nil {
					log.Fatal(err)
				}
				body, err := io.ReadAll(res.Body)
				res.Body.Close()
				if res.StatusCode > 299 {
					log.Fatalf("Response failed with status code: %d\n%s\n", res.StatusCode, body)
				}
				if err != nil {
					log.Fatal(err)
				}
				fmt.Printf("%s", body)
				wg.Done()
			}
		}(c)
	}
	for _, d := range de {
		p := filepath.Join(basedir, d.Name())
		var rx, rz int
		if _, err := fmt.Sscanf(d.Name(), "r.%d.%d.mca", &rz, &rx); err != nil {
			log.Printf("Error parsing file name of %s: %v, ignoring", d.Name(), err)
			continue
		}
		r, err := region.Open(p)
		if err != nil {
			log.Printf("Error when opening %s: %v, ignoring", d.Name(), err)
			continue
		}
		if err := r.Close(); err != nil {
			log.Printf("Close r.%d.%d.mca error: %v", rx, rz, err)
		}
		data, err := ioutil.ReadFile(p)
		if err != nil {
			log.Print("Error reading file: ", err.Error())
		}
		log.Printf("Sending region %d %d", rx, rz)
		wg.Add(1)
		c <- []byte(data)
		// for x := 0; x < 32; x++ {
		// 	for z := 0; z < 32; z++ {
		// 		if !r.ExistSector(x, z) {
		// 			continue
		// 		}
		// 		data, err := r.ReadSector(x, z)
		// 		if err != nil {
		// 			log.Printf("Read sector (%d.%d) error: %v", x, z, err)
		// 		}

		// var col save.Chunk
		// col.Load(data)
		// // log.Printf("Chunk %d %d", col.Level.PosX, col.Level.PosZ)
		// tag, err := conn.Exec(context.Background(), `
		// 	insert into chunks (dim, x, z, data) values (1, $1, $2, $3)`, -rx*32+x, rz*32+z, data)
		// if err != nil {
		// 	log.Print(err.Error())
		// }
		// if tag.RowsAffected() != 1 {
		// 	log.Print("Rows affected ", tag.RowsAffected())
		// }

		// res, err := http.Post("http://localhost:3001/api/submit/chunk/1", "binary/octet-stream", bytes.NewReader(data))
		// if err != nil {
		// 	log.Fatal(err)
		// }
		// body, err := io.ReadAll(res.Body)
		// res.Body.Close()
		// if res.StatusCode > 299 {
		// 	log.Fatalf("Response failed with status code: %d\n%s\n", res.StatusCode, body)
		// }
		// if err != nil {
		// 	log.Fatal(err)
		// }
		// fmt.Printf("%s", body)
		// 	}
		// }
	}
	go func() {
		time.Sleep(1 * time.Second)
		log.Print("Tasks left ", len(c))
	}()
	wg.Wait()
	log.Print("Done")
}

func sendThreadedHttpChunks(de []fs.DirEntry) {
	const threadscount = 8

	var wg sync.WaitGroup
	wg.Add(threadscount)
	c := make(chan []byte, threadscount*32*32)
	log.Print("Launching ", threadscount, " threads")

	for i := 0; i < threadscount; i++ {
		go func(chan []byte) {
			for data := range c {
				res, err := http.Post("http://spring-forge-eg-answered.trycloudflare.com/api/submit/chunk/1", "binary/octet-stream", bytes.NewReader(data))
				if err != nil {
					log.Fatal(err)
				}
				body, err := io.ReadAll(res.Body)
				res.Body.Close()
				if res.StatusCode > 299 {
					log.Fatalf("Response failed with status code: %d\n%s\n", res.StatusCode, body)
				}
				if err != nil {
					log.Fatal(err)
				}
				fmt.Printf("%s", body)
				wg.Done()
			}
		}(c)
	}
	for _, d := range de {
		p := filepath.Join(basedir, d.Name())
		var rx, rz int
		if _, err := fmt.Sscanf(d.Name(), "r.%d.%d.mca", &rz, &rx); err != nil {
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
				log.Print("Adding task ", x, z)
				wg.Add(1)
				c <- data

				// var col save.Chunk
				// col.Load(data)
				// // log.Printf("Chunk %d %d", col.Level.PosX, col.Level.PosZ)
				// tag, err := conn.Exec(context.Background(), `
				// 	insert into chunks (dim, x, z, data) values (1, $1, $2, $3)`, -rx*32+x, rz*32+z, data)
				// if err != nil {
				// 	log.Print(err.Error())
				// }
				// if tag.RowsAffected() != 1 {
				// 	log.Print("Rows affected ", tag.RowsAffected())
				// }

				// res, err := http.Post("http://localhost:3001/api/submit/chunk/1", "binary/octet-stream", bytes.NewReader(data))
				// if err != nil {
				// 	log.Fatal(err)
				// }
				// body, err := io.ReadAll(res.Body)
				// res.Body.Close()
				// if res.StatusCode > 299 {
				// 	log.Fatalf("Response failed with status code: %d\n%s\n", res.StatusCode, body)
				// }
				// if err != nil {
				// 	log.Fatal(err)
				// }
				// fmt.Printf("%s", body)
			}
		}
		if err := r.Close(); err != nil {
			log.Printf("Close r.%d.%d.mca error: %v", rx, rz, err)
		}
	}
	go func() {
		time.Sleep(1 * time.Second)
		log.Print("Tasks left ", len(c))
	}()
	wg.Wait()
	log.Print("Done")
}

func sendDBchunks(de []fs.DirEntry) {
	log.Print("Connecting to database")
	conn, err := pgx.Connect(context.Background(), os.Getenv("DB"))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close(context.Background())
	for _, d := range de {
		p := filepath.Join(basedir, d.Name())
		var rx, rz int
		if _, err := fmt.Sscanf(d.Name(), "r.%d.%d.mca", &rz, &rx); err != nil {
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

				var col save.Chunk
				col.Load(data)
				log.Printf("Chunk %d %d", col.XPos, col.ZPos)
				tag, err := conn.Exec(context.Background(), `
					insert into chunks (dim, x, z, data) values (1, $1, $2, $3)`, col.XPos, col.ZPos, data)
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
	log.Print("Done")
	return
}
