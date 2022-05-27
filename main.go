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
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/maxsupermanhd/WebChunk/chunkStorage"
	"github.com/maxsupermanhd/WebChunk/proxy"
	"github.com/maxsupermanhd/WebChunk/viewer"

	humanize "github.com/dustin/go-humanize"
	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
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

var storages []chunkStorage.Storage
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
	"add": func(a, b int) int {
		return a + b
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
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	buildinfo, ok := debug.ReadBuildInfo()
	if ok {
		GoVersion = buildinfo.GoVersion
	}
	_ = GoVersion
	err := loadConfig()
	if err != nil {
		log.Fatal("Error loading config file: " + err.Error())
	}
	storages = loadedConfig.Storages
	log.SetOutput(io.MultiWriter(&lumberjack.Logger{
		Filename: loadedConfig.LogsLocation,
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
	layoutsGlobPtr := &loadedConfig.Web.LayoutsGlob
	layouts, err = template.New("main").Funcs(layoutFuncs).ParseGlob(*layoutsGlobPtr)
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
					nlayouts, err := template.New("main").Funcs(layoutFuncs).ParseGlob(*layoutsGlobPtr)
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
	err = watcher.Add(loadedConfig.Web.LayoutsLocation)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Adding routes")
	router := mux.NewRouter()
	router.PathPrefix("/static").Handler(http.StripPrefix("/static/", http.FileServer(hiddenFileSystem{http.Dir("./static")}))).Methods("GET")
	router.HandleFunc("/favicon.ico", faviconHandler).Methods("GET")
	router.HandleFunc("/robots.txt", robotsHandler).Methods("GET")

	router.HandleFunc("/", indexHandler).Methods("GET")
	router.HandleFunc("/worlds", worldsHandler).Methods("GET")
	router.HandleFunc("/worlds/{world}", worldHandler).Methods("GET")
	router.HandleFunc("/worlds/{world}/{dim}", dimensionHandler).Methods("GET")
	router.HandleFunc("/worlds/{world}/{dim}/chunk/info/{cx:-?[0-9]+}/{cz:-?[0-9]+}", terrainInfoHandler).Methods("GET")
	router.HandleFunc("/worlds/{world}/{dim}/tiles/{ttype}/{cs:[0-9]+}/{cx:-?[0-9]+}/{cz:-?[0-9]+}/{format}", tileRouterHandler).Methods("GET")
	router.HandleFunc("/colors", colorsHandlerGET).Methods("GET")
	router.HandleFunc("/colors", colorsHandlerPOST).Methods("POST")
	router.HandleFunc("/colors/save", colorsSaveHandler).Methods("GET")

	router.HandleFunc("/api/submit/chunk/{world}/{dim}", apiAddChunkHandler)
	router.HandleFunc("/api/submit/region/{world}/{dim}", apiAddRegionHandler)

	router.HandleFunc("/api/storages", apiHandle(apiStoragesGET)).Methods("GET")
	router.HandleFunc("/api/storages/{storage}/reinit", apiHandle(apiStorageReinit)).Methods("GET")

	router.HandleFunc("/api/worlds", apiHandle(apiAddWorld)).Methods("POST")
	router.HandleFunc("/api/worlds", apiHandle(apiListWorlds)).Methods("GET")

	router.HandleFunc("/api/dims", apiHandle(apiAddDimension)).Methods("POST")
	router.HandleFunc("/api/dims", apiHandle(apiListDimensions)).Methods("GET")

	router1 := handlers.ProxyHeaders(router)
	router2 := handlers.CompressHandler(router1)
	router3 := handlers.CustomLoggingHandler(os.Stdout, router2, customLogger)
	router4 := handlers.RecoveryHandler()(router3)

	log.Println("Initializing storages...")
	for i := range storages {
		storages[i].Driver, err = initStorage(storages[i].Type, storages[i].Address)
		if err != nil {
			log.Println("Failed to initialize storage: " + err.Error())
			continue
		}
		ver, err := storages[i].Driver.GetStatus()
		if err != nil {
			log.Println("Error getting storage status: " + err.Error())
			continue
		}
		log.Println("Storage initialized: " + ver)
	}
	defer chunkStorage.CloseStorages(storages)

	chunkChannel := make(chan *proxy.ProxiedChunk, 12*12)
	go func() {
		if loadedConfig.Web.Listen == "" {
			log.Println("Not starting web server because listen address is empty")
			return
		}
		log.Println("Starting web server (http://" + loadedConfig.Web.Listen + "/)")
		log.Println(http.ListenAndServe(loadedConfig.Web.Listen, router4))
	}()
	go func() {
		if loadedConfig.Proxy.Listen == "" {
			log.Println("Not starting proxy because listen address is empty")
			return
		}
		log.Println("Starting proxy")
		proxy.RunProxy(ProxyRoutesHandler, &loadedConfig.Proxy, chunkChannel)
	}()
	go chunkConsumer(chunkChannel)
	go func() {
		if loadedConfig.Reconstructor.Listen == "" {
			log.Println("Not starting reconstructor because listen address is empty")
			return
		}
		log.Println("Starting reconstructor")
		viewer.StartReconstructor(storages, &loadedConfig.Reconstructor)
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	for range c {
		return
	}
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
		// prevCPUReport = fmt.Sprintf("%.1f%% [busy: %.2f, total: %.2f] (past %s)", CPUUsage, totalTicks-idleTicks, totalTicks, (time.Duration(time.Since(prevTime).Seconds()) * time.Second).String())
		prevCPUReport = fmt.Sprintf("%.1f%% (past %s)", CPUUsage, (time.Duration(time.Since(prevTime).Seconds()) * time.Second).String())
		prevTime = time.Now()
		prevCPUIdle = CPUIdle
		prevCPUTotal = CPUTotal
	}
	CPUReport := prevCPUReport
	prevLock.Unlock()

	var chunksCount, chunksSizeBytes uint64
	type DimData struct {
		Dim        chunkStorage.DimStruct
		ChunkSize  string
		ChunkCount uint64
		CacheSize  string
		CacheCount int64
	}
	type WorldData struct {
		World chunkStorage.WorldStruct
		Dims  []DimData
	}
	type StorageData struct {
		S      chunkStorage.Storage
		Worlds []WorldData
		Online bool
	}
	st := []StorageData{}
	for _, s := range storages {
		worlds := []WorldData{}
		if s.Driver == nil {
			st = append(st, StorageData{S: s, Worlds: worlds, Online: false})
			// log.Println("Skipping storage " + s.Name + " because driver is uninitialized")
			continue
		}
		achunksCount, _ := s.Driver.GetChunksCount()
		achunksSizeBytes, _ := s.Driver.GetChunksSize()
		chunksCount += achunksCount
		chunksSizeBytes += achunksSizeBytes
		worldss, err := s.Driver.ListWorlds()
		if err != nil {
			plainmsg(w, r, plainmsgColorRed, "Error listing worlds of storage "+s.Name+": "+err.Error())
			return
		}
		for _, wrld := range worldss {
			wd := WorldData{World: wrld, Dims: []DimData{}}
			dims, err := s.Driver.ListWorldDimensions(wrld.Name)
			if err != nil {
				plainmsg(w, r, plainmsgColorRed, "Error listing dimensions of world "+wrld.Name+" of storage "+s.Name+": "+err.Error())
				return
			}
			for _, dim := range dims {
				dimChunksCount, err := s.Driver.GetDimensionChunksCount(wrld.Name, dim.Name)
				if err != nil {
					plainmsg(w, r, plainmsgColorRed, "Error getting chunk count of dim "+dim.Name+" of world "+wrld.Name+" of storage "+s.Name+": "+err.Error())
					return
				}
				dimChunksSize, err := s.Driver.GetDimensionChunksSize(wrld.Name, dim.Name)
				if err != nil {
					plainmsg(w, r, plainmsgColorRed, "Error getting chunks size of dim "+dim.Name+" of world "+wrld.Name+" of storage "+s.Name+": "+err.Error())
					return
				}
				dimCacheCount, dimCacheSize, err := getImageCacheCountSize(wrld.Name, dim.Name)
				if err != nil {
					plainmsg(w, r, plainmsgColorRed, "Error getting cache size and counts of dim "+dim.Name+" of world "+wrld.Name+": "+err.Error())
					return
				}
				wd.Dims = append(wd.Dims, DimData{
					Dim:        dim,
					ChunkSize:  humanize.Bytes(dimChunksSize),
					ChunkCount: dimChunksCount,
					CacheSize:  humanize.Bytes(uint64(dimCacheSize)),
					CacheCount: dimCacheCount,
				})
			}
			worlds = append(worlds, wd)
		}
		st = append(st, StorageData{S: s, Worlds: worlds, Online: true})
	}
	chunksSize := humanize.Bytes(chunksSizeBytes)
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
		"Storages":    st,
	})
}

func worldsHandler(w http.ResponseWriter, r *http.Request) {
	worlds := chunkStorage.ListWorlds(storages)
	basicLayoutLookupRespond("worlds", w, r, map[string]interface{}{"Worlds": worlds})
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
