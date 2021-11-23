package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"reflect"
	"strconv"
	_ "strconv"
	"time"

	"github.com/alexedwards/scs/pgxstore"
	scs "github.com/alexedwards/scs/v2"
	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"
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

var layouts *template.Template
var sessionManager *scs.SessionManager
var dbpool *pgxpool.Pool
var layoutFuncs = template.FuncMap{
	"noescape": func(s string) template.HTML {
		return template.HTML(s)
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
	fmt.Fprintln(w, "User-agent: *\nDisallow: /\n\n")
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
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println()
	log.Println("WebChunk server is starting up...")
	log.Printf("Built %s, Ver %s (%s)\n", BuildTime, GitTag, CommitHash)
	log.Println()
	rand.Seed(time.Now().UTC().UnixNano())
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	log.SetOutput(&lumberjack.Logger{
		Filename: "webchunk.log",
		MaxSize:  10,
		Compress: true,
	})

	log.Println()
	log.Println("WebChunk web server is starting up...")
	log.Printf("Built %s, Ver %s (%s)\n", BuildTime, GitTag, CommitHash)
	log.Println()

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

	log.Println("Connecting to database")
	dbpool, err = pgxpool.Connect(context.Background(), os.Getenv("DB"))
	if err != nil {
		log.Fatal(err)
	}
	defer dbpool.Close()

	log.Println("Starting session manager")
	sessionManager = scs.New()
	store := pgxstore.New(dbpool)
	sessionManager.Store = store
	sessionManager.Lifetime = 14 * 24 * time.Hour
	defer store.StopCleanup()

	log.Println("Adding routes")
	router := mux.NewRouter()
	// router.NotFoundHandler = myNotFoundHandler()
	router.PathPrefix("/static").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	router.HandleFunc("/favicon.ico", faviconHandler)
	router.HandleFunc("/robots.txt", robotsHandler)

	router.HandleFunc("/", indexHandler)
	router.HandleFunc("/servers/{sid:[0-9]+}", serverHandler)
	router.HandleFunc("/servers/{sid:[0-9]+}/dimensions/{did:-?[0-9]+}", dimensionHandler)
	router.HandleFunc("/servers/{sid:[0-9]+}/dimensions/{did:[0-9]+}/terrain/{cx:-?[0-9]+}/{cz:-?[0-9]+}/jpeg", terrainJpegHandler)
	router.HandleFunc("/servers/{sid:[0-9]+}/dimensions/{did:[0-9]+}/terrain/{cx:-?[0-9]+}/{cz:-?[0-9]+}/info", terrainInfoHandler)
	router.HandleFunc("/servers/{sid:[0-9]+}/dimensions/{did:[0-9]+}/tiles/{cs:[0-9]+}/{cx:-?[0-9]+}/{cz:-?[0-9]+}/jpeg", terrainScaleJpegHandler)

	router.HandleFunc("/api/submit/chunk/{did:-?[0-9]+}", apiAddChunkHandler)
	router.HandleFunc("/api/submit/region/{did:-?[0-9]+}", apiAddRegionHandler)

	router0 := sessionManager.LoadAndSave(router)
	router1 := handlers.ProxyHeaders(router0)
	//	router2 := handlers.CompressHandler(router1)
	router3 := handlers.CustomLoggingHandler(os.Stdout, router1, customLogger)
	// router4 := handlers.RecoveryHandler()(router3)
	log.Println("Started!")
	log.Panic(http.ListenAndServe(":"+port, router3))
}

type DimStruct struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Alias string `json:"alias"`
}

type ServerStruct struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip"`
}

func listServers() ([]ServerStruct, error) {
	var servers []ServerStruct
	derr := dbpool.QueryRow(context.Background(), `
	select
		json_agg(json_build_object('id', id)::jsonb ||
			json_build_object('name', name)::jsonb ||
			json_build_object('ip', ip)::jsonb)
	from servers;`).Scan(&servers)
	return servers, derr
}

func getServerByID(sid int) (ServerStruct, error) {
	var server ServerStruct
	server.ID = sid
	derr := dbpool.QueryRow(context.Background(), `
	select
		json_build_object('name', name)::jsonb || json_build_object('ip', ip)::jsonb
	from servers
	where id = $1;`, sid).Scan(&server)
	return server, derr
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	load, _ := load.Avg()
	virtmem, _ := mem.VirtualMemory()
	uptime, _ := host.Uptime()
	uptimetime, _ := time.ParseDuration(strconv.Itoa(int(uptime)) + "s")
	servers, derr := listServers()
	if derr != nil {
		plainmsg(w, r, 2, "Database query error: "+derr.Error())
		return
	}
	basicLayoutLookupRespond("index", w, r, map[string]interface{}{"LoadAvg": load, "VirtMem": virtmem, "Uptime": uptimetime, "Servers": servers})
}

func listDimensionsByServer(sid int) ([]DimStruct, error) {
	var dims []DimStruct
	derr := dbpool.QueryRow(context.Background(), `
		select
			json_agg(json_build_object('id', id)::jsonb ||
			json_build_object('name', name)::jsonb ||
			json_build_object('alias', alias)::jsonb)
		from dimensions
		where server = $1;`, sid).Scan(&dims)
	return dims, derr
}

func getDimensionByID(did int) (DimStruct, error) {
	var dim DimStruct
	derr := dbpool.QueryRow(context.Background(), `
		select json_build_object('id', id)::jsonb ||
			json_build_object('name', name)::jsonb ||
			json_build_object('alias', alias)::jsonb
		from dimensions
		where id = $1;`, did).Scan(&dim)
	return dim, derr
}

func serverHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	sids := params["sid"]
	sid, err := strconv.Atoi(sids)
	if err != nil {
		plainmsg(w, r, 2, "Bad server id: "+err.Error())
		return
	}
	server, derr := getServerByID(sid)
	if derr != nil {
		plainmsg(w, r, 2, "Database query error: "+derr.Error())
		return
	}
	dims, derr := listDimensionsByServer(sid)
	if derr != nil {
		plainmsg(w, r, 2, "Database query error: "+derr.Error())
		return
	}
	basicLayoutLookupRespond("server", w, r, map[string]interface{}{"Dims": dims, "Server": server})
}

func dimensionHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	sids := params["sid"]
	sid, err := strconv.Atoi(sids)
	if err != nil {
		plainmsg(w, r, 2, "Bad server id: "+err.Error())
		return
	}
	dids := params["did"]
	did, err := strconv.Atoi(dids)
	if err != nil {
		plainmsg(w, r, 2, "Bad dim id: "+err.Error())
		return
	}
	server, derr := getServerByID(sid)
	if derr != nil {
		plainmsg(w, r, 2, "Database query error: "+derr.Error())
		return
	}
	dim, derr := getDimensionByID(did)
	if derr != nil {
		plainmsg(w, r, 2, "Database query error: "+derr.Error())
		return
	}
	basicLayoutLookupRespond("dim", w, r, map[string]interface{}{"Dim": dim, "Server": server})
}
