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

package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"reflect"
	"strconv"
	_ "strconv"
	"strings"
	"sync"
	"time"

	"github.com/maxsupermanhd/mcwebchunk/chunkStorage"
	"github.com/maxsupermanhd/mcwebchunk/chunkStorage/postgresChunkStorage"

	viewer "github.com/maxsupermanhd/mcwebchunk/viewer"

	humanize "github.com/dustin/go-humanize"
	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/natefinch/lumberjack"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
)

var (
	BuildTime  = "00000000.000000"
	CommitHash = "0000000"
	GoVersion  = "0.0"
	GitTag     = "0.0"
)

var storage chunkStorage.ChunkStorage
var layouts *template.Template
var layoutFuncs = template.FuncMap{
	"noescape": func(s string) template.HTML {
		return template.HTML(s)
	},
	"inc": func(i int) int {
		return i + 1
	},
	"avail": func(name string, data interface{}) bool {
		v := reflect.ValueOf(data)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		m, ok := data.(map[string]interface{})
		if ok {
			_, ok := m[name]
			return ok
		}
		if v.Kind() != reflect.Struct {
			return false
		}
		return v.FieldByName(name).IsValid()
	},
	"FormatBytes":   ByteCountIEC,
	"FormatPercent": FormatPercent,
}

func FormatPercent(p float64) string {
	return fmt.Sprintf("%.1f%%", p)
}

func ByteCountIEC(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func robotsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "User-agent: *\nDisallow: /\n\n\n")
}
func faviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/favicon.ico")
}

func customLogger(writer io.Writer, params handlers.LogFormatterParams) {
	r := params.Request
	ip := r.Header.Get("CF-Connecting-IP")
	geo := r.Header.Get("CF-IPCountry")
	ua := r.Header.Get("user-agent")
	log.Println("["+geo+" "+ip+"]", r.Method, params.StatusCode, r.RequestURI, "["+ua+"]")
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())
	_ = GoVersion
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}
	port := os.Getenv("WEB_PORT")
	if port == "" {
		port = "3000"
	}
	log.SetOutput(io.MultiWriter(&lumberjack.Logger{
		Filename: "webchunk.log",
		MaxSize:  10,
		Compress: true,
	}, os.Stdout))

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println()
	log.Println("WebChunk web server is starting up...")
	log.Printf("Built %s, Ver %s (%s)\n", BuildTime, GitTag, CommitHash)
	log.Println()

	prevTime = time.Now()

	initChunkDraw()

	log.Println("Loading layouts")
	layouts, err = template.New("main").Funcs(layoutFuncs).ParseGlob("layouts/*.gohtml")
	if err != nil {
		panic(err)
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("Updating templates")
					nlayouts, err := template.New("main").Funcs(layoutFuncs).ParseGlob("layouts/*.gohtml")
					if err != nil {
						log.Println("Error while parsing templates:", err.Error())
					} else {
						layouts = nlayouts.Funcs(layoutFuncs)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()
	err = watcher.Add("layouts/")
	if err != nil {
		log.Fatal(err)
	}

	// log.Println("Initializing OpenGL")
	// err = initOpenGL()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer glfw.Terminate()
	// log.Println("Compiling shaders")
	// err = prepareShaders()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// log.Println("Running demo app")
	// app()

	log.Println("Connecting to database")

	storage, err = postgresChunkStorage.NewStorage(context.Background(), os.Getenv("DB"))
	defer storage.Close()

	log.Println("Adding routes")
	router := mux.NewRouter()
	router.PathPrefix("/static").Handler(http.StripPrefix("/static/", http.FileServer(hiddenFileSystem{http.Dir("./static")}))).Methods("GET")
	router.HandleFunc("/favicon.ico", faviconHandler).Methods("GET")
	router.HandleFunc("/robots.txt", robotsHandler).Methods("GET")

	router.HandleFunc("/", indexHandler).Methods("GET")
	router.HandleFunc("/servers", serversHandler).Methods("GET")
	router.HandleFunc("/servers/{server}", serverHandler).Methods("GET")
	router.HandleFunc("/servers/{server}/{dim}", dimensionHandler).Methods("GET")
	router.HandleFunc("/servers/{server}/{dim}/chunk/info/{cx:-?[0-9]+}/{cz:-?[0-9]+}", terrainInfoHandler).Methods("GET")
	router.HandleFunc("/servers/{server}/{dim}/chunk/image/{cx:-?[0-9]+}/{cz:-?[0-9]+}/{format}", terrainImageHandler).Methods("GET")
	router.HandleFunc("/servers/{server}/{dim}/tiles/{ttype}/{cs:[0-9]+}/{cx:-?[0-9]+}/{cz:-?[0-9]+}/{format}", tileRouterHandler).Methods("GET")
	router.HandleFunc("/colors", colorsHandlerGET).Methods("GET")
	router.HandleFunc("/colors", colorsHandlerPOST).Methods("POST")
	router.HandleFunc("/colors/save", colorsSaveHandler).Methods("GET")

	router.HandleFunc("/api/submit/chunk/{server}/{dim}", apiAddChunkHandler)
	router.HandleFunc("/api/submit/region/{server}/{dim}", apiAddRegionHandler)

	router.HandleFunc("/api/servers", apiHandle(apiAddServer)).Methods("POST")
	router.HandleFunc("/api/servers", apiHandle(apiListServers)).Methods("GET")

	router.HandleFunc("/api/dims", apiHandle(apiAddDimension)).Methods("POST")
	router.HandleFunc("/api/dims", apiHandle(apiListDimensions)).Methods("GET")

	router1 := handlers.ProxyHeaders(router)
	// router2 := handlers.CompressHandler(router1)
	router3 := handlers.CustomLoggingHandler(os.Stdout, router1, customLogger)
	// router4 := handlers.RecoveryHandler()(router3)
	log.Println("Started! (http://127.0.0.1:" + port + "/)")
	go func() {
		log.Panic(http.ListenAndServe(":"+port, router3))
	}()
	viewer.StartReconstructor(storage)
}

var prevCPUIdle uint64
var prevCPUTotal uint64
var prevTime time.Time
var prevCPUReport string
var prevLock sync.Mutex

func indexHandler(w http.ResponseWriter, r *http.Request) {
	load, _ := load.Avg()
	virtmem, _ := mem.VirtualMemory()
	uptime, _ := host.Uptime()
	uptimetime, _ := time.ParseDuration(strconv.Itoa(int(uptime)) + "s")

	prevLock.Lock()
	var CPUUsage float64
	var idleTicks, totalTicks float64
	if time.Since(prevTime) > 1*time.Second {
		CPUIdle, CPUTotal := getCPUSample()
		idleTicks = float64(CPUIdle - prevCPUIdle)
		totalTicks = float64(CPUTotal - prevCPUTotal)
		CPUUsage = 100 * (totalTicks - idleTicks) / totalTicks
		prevCPUReport = fmt.Sprintf("%.1f%% [busy: %.2f, total: %.2f] (past %s)", CPUUsage, totalTicks-idleTicks, totalTicks, (time.Duration(time.Since(prevTime).Seconds()) * time.Second).String())
		prevTime = time.Now()
		prevCPUIdle = CPUIdle
		prevCPUTotal = CPUTotal
	}
	CPUReport := prevCPUReport
	prevLock.Unlock()

	chunksCount, _ := storage.GetChunksCount()
	chunksSizeBytes, _ := storage.GetChunksSize()
	chunksSize := humanize.Bytes(chunksSizeBytes)
	serverss, err := storage.ListServers()
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Error getting servers: "+err.Error())
		return
	}
	type DimData struct {
		Dim        chunkStorage.DimStruct
		ChunkSize  string
		ChunkCount int64
		CacheSize  string
		CacheCount int64
	}
	type ServerData struct {
		Server chunkStorage.ServerStruct
		Dims   []DimData
	}
	servers := []ServerData{}
	for _, s := range serverss {
		servers = append(servers, ServerData{Server: s, Dims: []DimData{}})
	}
	dimss, err := storage.ListDimensions()
	if err != nil {
		plainmsg(w, r, plainmsgColorRed, "Error getting dimensions: "+err.Error())
		return
	}
	for _, d := range dimss {
		chunkCount, chunkSize, err := storage.GetDimensionChunkCountSize(d.ID)
		if err != nil {
			plainmsg(w, r, plainmsgColorRed, "Error getting dimension details from database: "+err.Error())
		}
		for si, ss := range servers {
			if ss.Server.ID == d.Server {
				cacheCount, cacheSize, err := getImageCacheCountSize(ss.Server.Name, d.Name)
				if err != nil {
					plainmsg(w, r, plainmsgColorRed, "Error getting dimension details from cache: "+err.Error())
				}
				servers[si].Dims = append(servers[si].Dims, DimData{
					Dim:        d,
					ChunkSize:  chunkSize,
					ChunkCount: chunkCount,
					CacheSize:  humanize.Bytes(uint64(cacheSize)),
					CacheCount: cacheCount,
				})
			}
		}
	}

	basicLayoutLookupRespond("index", w, r, map[string]interface{}{
		"BuildTime":   BuildTime,
		"GitTag":      GitTag,
		"CommitHash":  CommitHash,
		"GoVersion":   GoVersion,
		"LoadAvg":     load,
		"VirtMem":     virtmem,
		"Uptime":      uptimetime,
		"ChunksCount": chunksCount,
		"ChunksSize":  chunksSize,
		"CPUReport":   CPUReport,
		"Servers":     servers,
	})
}

func serversHandler(w http.ResponseWriter, r *http.Request) {
	servers, derr := storage.ListServers()
	if derr != nil {
		plainmsg(w, r, plainmsgColorRed, "Database query error: "+derr.Error())
		return
	}
	basicLayoutLookupRespond("servers", w, r, map[string]interface{}{"Servers": servers})
}

func getCPUSample() (idle, total uint64) {
	contents, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return
	}
	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if fields[0] == "cpu" {
			numFields := len(fields)
			for i := 1; i < numFields; i++ {
				val, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					fmt.Println("Error: ", i, fields[i], err)
				}
				total += val // tally up all the numbers to get total ticks
				if i == 4 {  // idle is the 5th field in the cpu line
					idle = val
				}
			}
			return
		}
	}
	return
}
