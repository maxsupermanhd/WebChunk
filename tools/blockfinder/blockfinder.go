package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/maxsupermanhd/WebChunk/chunkStorage/postgresChunkStorage"
)

var (
	dbstr      = flag.String("db", "", "Database connection string")
	dname      = flag.String("dname", "the_nether", "Dim name")
	wname      = flag.String("wname", "phoenixanarchy.com", "World name")
	outfname   = flag.String("out", "out.txt", "Filename for writing results to")
	threadsnum = flag.Int("threads", 3, "Thread count")
	cs         *postgresChunkStorage.PostgresChunkStorage
	did        int
)

func must(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

type xzcoord struct {
	x int64
	z int64
}

func worker(wid int, jobs <-chan xzcoord, results chan<- string, wg *sync.WaitGroup) {
	log.Printf("Worker %d started", wid)
	conn, err := cs.DBPool.Acquire(context.Background())
	must(err)
	defer wg.Done()
	chunkcount := 0
	for j := range jobs {
		c, err := postgresChunkStorage.ConnGetChunkByDID(conn, did, int(j.x), int(j.z))
		chunkcount++
		must(err)
		for si, s := range c.Sections {
			for _, b := range s.BlockStates.Palette {
				if strings.Contains(b.Name, "portal") {
					log.Printf("CHUNK x%d z%d section %d palette match %s", j.x, j.z, si, b.Name)
					results <- fmt.Sprintf("CHUNK x%d z%d section %d palette match %s", j.x, j.z, si, b.Name)
				}
			}
		}
	}
	log.Printf("Worker %d exits, processed %d chunks", wid, chunkcount)
}

func filewriter(results <-chan string) {
	log.Printf("Filewriter thread started")
	file, err := os.OpenFile(*outfname, os.O_APPEND|os.O_WRONLY, 0644)
	must(err)
	defer file.Close()
	linecount := 0
	for r := range results {
		linecount++
		if !strings.HasSuffix(r, "\n") {
			r = r + "\n"
		}
		file.WriteString(r)
	}
	log.Printf("File writer exits, wrote %d lines", linecount)
}

func main() {
	flag.Parse()
	log.Println("Hello world")
	if *dbstr == "" {
		log.Fatalln("Database connection string not set")
	}
	var err error
	cs, err = postgresChunkStorage.NewPostgresChunkStorage(context.Background(), *dbstr)
	must(err)
	must(cs.DBPool.QueryRow(context.Background(), "SELECT id FROM dimensions WHERE name = $1 and world = $2", *dname, *wname).Scan(&did))
	rows, err := cs.DBPool.Query(context.Background(), "SELECT DISTINCT x, z FROM chunks WHERE dim = $1", did)
	must(err)
	coordlist := make([]xzcoord, 0)
	for rows.Next() {
		var x, z int64
		must(rows.Scan(&x, &z))
		coordlist = append(coordlist, xzcoord{x, z})
	}

	jobs := make(chan xzcoord, 64)
	results := make(chan string)
	wg := new(sync.WaitGroup)
	go filewriter(results)
	for w := 0; w <= *threadsnum; w++ {
		wg.Add(1)
		go worker(w, jobs, results, wg)
	}
	prevchunks := 0
	starttime := time.Now()
	prevtime := time.Now()
	for i, coords := range coordlist {
		jobs <- coords
		if time.Since(prevtime) > 1*time.Second {
			deltachunks := i - prevchunks
			deltatime := time.Since(prevtime)
			log.Printf("Processed %10d chunks, %10d to go (%06.2f%%) (%6d chunks in %s) (%6.0f chunks/s) (%s ETA)",
				i, len(coordlist), float32(i)/float32(len(coordlist))*100,
				deltachunks, deltatime.Round(time.Second), float64(deltachunks)/deltatime.Seconds(),
				time.Duration(time.Duration(len(coordlist)/(deltachunks/int(deltatime.Seconds())))*time.Second).Round(time.Second))

			prevtime = time.Now()
			prevchunks = i
		}
	}
	close(jobs)
	wg.Wait()
	close(results)

	log.Printf("Processed %d chunks in %s", len(coordlist), time.Since(starttime).Round(time.Second))
}
