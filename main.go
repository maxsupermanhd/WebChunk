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
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"github.com/maxsupermanhd/WebChunk/chunkStorage"
	imagecache "github.com/maxsupermanhd/WebChunk/imageCache"
	"github.com/maxsupermanhd/WebChunk/proxy"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/natefinch/lumberjack"
)

var (
	BuildTime  = "00000000.000000"
	CommitHash = "0000000"
	GoVersion  = "0.0"
	GitTag     = "0.0"
)

var (
	ic *imagecache.ImageCache
)

func customLogger(_ io.Writer, params handlers.LogFormatterParams) {
	r := params.Request
	ip := r.Header.Get("CF-Connecting-IP")
	if ip == "" {
		ip = r.RemoteAddr
	}
	geo := r.Header.Get("CF-IPCountry")
	if geo == "" {
		geo = "??"
	}
	ua := r.Header.Get("user-agent")
	log.Println("["+geo+" "+ip+"]", r.Method, params.StatusCode, r.RequestURI, "["+ua+"]")
}

func main() {
	if err := loadConfig(); err != nil {
		log.Println("Error loading config file: " + err.Error())
		log.Println("Defaults will be used.")
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	if buildinfo, ok := debug.ReadBuildInfo(); ok {
		GoVersion = buildinfo.GoVersion
	}
	lg := lumberjack.Logger{
		Filename: cfg.GetDSString("./logs/WebChunk.log", "logs_path"),
		MaxSize:  10,
		Compress: true,
	}
	log.SetOutput(io.MultiWriter(&lg, os.Stdout))
	log.Println()
	log.Println("WebChunk web server is starting up...")
	log.Printf("Built %s, Ver %s (%s) (%s)\n", BuildTime, GitTag, CommitHash, GoVersion)
	log.Println()

	var wg sync.WaitGroup
	ctx, ctxCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	if err := loadColors(cfg.GetDSString("./colors.gob", "colors_path")); err != nil {
		log.Fatal(err)
	}

	// log.Println("Starting image scaler")
	// wg.Add(1)
	// go func() {
	// 	imagingProcessor(ctx)
	// 	log.Println("Image scaler stopped")
	// 	wg.Done()
	// }()

	log.Println("Starting metrix dispatcher")
	wg.Add(2)
	go func() {
		metricsDispatcher()
		log.Println("Metrix dispatcher stopped")
		wg.Done()
	}()
	go func() {
		<-ctx.Done()
		closeMetrics()
		wg.Done()
	}()

	log.Println("Starting event router")
	wg.Add(1)
	go func() {
		globalEventRouter.Run(ctx)
		log.Println("Event router stopped")
		wg.Done()
	}()

	if err := storagesInit(); err != nil {
		log.Fatal("Failed to initialize storages: ", err)
	}

	log.Println("Starting template manager")
	wg.Add(1)
	go func() {
		templateManager(ctx, cfg.SubTree("web"))
		wg.Done()
	}()

	log.Println("Adding routes")
	// wg.Add(1)
	// go func() {
	// 	tasksProgressBroadcaster.Start()
	// 	wg.Done()
	// }()
	// defer tasksProgressBroadcaster.Stop()
	router := mux.NewRouter()
	router.PathPrefix("/static").Handler(http.StripPrefix("/static/", http.FileServer(hiddenFileSystem{http.Dir("./static")}))).Methods("GET")
	router.HandleFunc("/favicon.ico", faviconHandler).Methods("GET")
	router.HandleFunc("/robots.txt", robotsHandler).Methods("GET")

	router.HandleFunc("/", indexHandler).Methods("GET")
	router.HandleFunc("/stop", func(w http.ResponseWriter, _ *http.Request) {
		ctxCancel()
		w.WriteHeader(200)
		w.Write([]byte("Success"))
	}).Methods("GET")
	router.HandleFunc("/worlds/{world}/{dim}", dimensionHandler).Methods("GET")
	router.HandleFunc("/worlds/{world}/{dim}/chunk/info/{cx:-?[0-9]+}/{cz:-?[0-9]+}", terrainInfoHandler).Methods("GET")
	router.HandleFunc("/worlds/{world}/{dim}/tiles/{ttype}/{cs:[0-9]+}/{cx:-?[0-9]+}/{cz:-?[0-9]+}/{format}", tileRouterHandler).Methods("GET")
	router.HandleFunc("/view", basicTemplateResponseHandler("view")).Methods("GET")
	router.HandleFunc("/colors", colorsHandlerGET).Methods("GET")
	router.HandleFunc("/colors", colorsHandlerPOST).Methods("POST")
	router.HandleFunc("/colors/save", colorsSaveHandler).Methods("GET")
	router.HandleFunc("/cfg", cfgHandler).Methods("GET")

	router.HandleFunc("/api/v1/config/save", apiHandle(apiSaveConfig)).Methods("GET")

	router.HandleFunc("/api/v1/submit/chunk/{world}/{dim}", apiHandle(apiAddChunkHandler))
	router.HandleFunc("/api/v1/submit/region/{world}/{dim}", apiAddRegionHandler)

	router.HandleFunc("/api/v1/renderers", apiHandle(apiListRenderers)).Methods("GET")

	router.HandleFunc("/api/v1/storages", apiHandle(apiStoragesGET)).Methods("GET")
	router.HandleFunc("/api/v1/storages", apiHandle(apiStorageAdd)).Methods("PUT")
	router.HandleFunc("/api/v1/storages/{storage}/reinit", apiHandle(apiStorageReinit)).Methods("GET")

	router.HandleFunc("/api/v1/worlds", apiHandle(apiAddWorld)).Methods("POST")
	router.HandleFunc("/api/v1/worlds", apiHandle(apiListWorlds)).Methods("GET")

	router.HandleFunc("/api/v1/dims", apiHandle(apiAddDimension)).Methods("POST")
	router.HandleFunc("/api/v1/dims", apiHandle(apiListDimensions)).Methods("GET")

	router.HandleFunc("/api/v1/ws", wsClientHandlerWrapper(ctx))

	router1 := handlers.ProxyHeaders(router)
	router2 := handlers.CompressHandler(router1)
	router3 := handlers.CustomLoggingHandler(os.Stdout, router2, customLogger)
	router4 := handlers.RecoveryHandler(handlers.PrintRecoveryStack(true))(router3)

	chunkChannel := make(chan *proxy.ProxiedChunk, 12*12)
	wg.Add(1)
	go func() {
		addr := cfg.GetDSString("0.0.0.0:3002", "web", "listen_addr")
		if addr == "" {
			log.Println("Not starting web server because listen address is empty")
			return
		}
		websrv := http.Server{
			Addr:    addr,
			Handler: router4,
		}
		log.Println("Starting web server (http://" + addr + "/)")
		wg.Add(1)
		go func() {
			if err := websrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Web server returned an error: %s\n", err)
			}
			wg.Done()
		}()
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := websrv.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("Server Shutdown Failed:%+v", err)
		} else {
			log.Println("Web server stopped")
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		addr := cfg.GetDSString("0.0.0.0:25566", "proxy", "listen_addr")
		if addr == "" {
			log.Println("Not starting proxy because listen address is empty")
			return
		}
		log.Println("Starting proxy")
		proxy.RunProxy(ctx, cfg.SubTree("proxy"), chunkChannel)
		log.Println("Proxy stopped")
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		chunkConsumer(ctx, chunkChannel)
		log.Println("Chunk consumer stopped")
		wg.Done()
	}()
	// go func() {
	// 	if loadedConfig.Reconstructor.Listen == "" {
	// 		log.Println("Not starting reconstructor because listen address is empty")
	// 		return
	// 	}
	// 	log.Println("Starting reconstructor")
	// 	viewer.StartReconstructor(storages, &loadedConfig.Reconstructor)
	// }()
	wg.Add(1)
	go func() {
		ic = imagecache.NewImageCache(log.Default(), cfg.SubTree("imageCache"), ctx)
		ic.WaitExit()
		log.Println("Image cache stopped")
		wg.Done()
	}()

	<-ctx.Done()
	log.Println("Interrupt recieved, shutting down...")
	ctxCancel()
	wg.Wait()
	wsClients.Wait()
	log.Println("Shutting down storages...")
	chunkStorage.CloseStorages(storages)
	log.Println("Storages closed.")
	lg.Close()
	log.Println("Shutdown complete, bye!")
}
