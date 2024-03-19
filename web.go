package main

import (
	"context"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func createRouter(exitchan <-chan struct{}) http.Handler {
	router := mux.NewRouter()
	router.PathPrefix("/static").Handler(http.StripPrefix("/static/", http.FileServer(hiddenFileSystem{http.Dir("./static")}))).Methods("GET")
	router.HandleFunc("/favicon.ico", faviconHandler).Methods("GET")
	router.HandleFunc("/robots.txt", robotsHandler).Methods("GET")

	router.HandleFunc("/", indexHandler).Methods("GET")
	router.HandleFunc("/stop", func(w http.ResponseWriter, _ *http.Request) {
		mainCtxCancel()
		w.WriteHeader(200)
		w.Write([]byte("Success"))
	}).Methods("GET")
	router.HandleFunc("/worlds/{world}/{dim}", dimensionHandler).Methods("GET")
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

	router.HandleFunc("/api/v1/ws", wsClientHandlerWrapper(exitchan))

	router.HandleFunc("/debug/chunk/{world}/{dim}/{cx:-?[0-9]+}/{cz:-?[0-9]+}", terrainInfoHandler).Methods("GET")
	router.HandleFunc("/debug/pprof/", pprof.Index)
	router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	router.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	router.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	router.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	router.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	router.Handle("/debug/pprof/block", pprof.Handler("block"))
	router.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	router.HandleFunc("/debug/gc", func(w http.ResponseWriter, r *http.Request) {
		runtime.GC()
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	router1 := handlers.ProxyHeaders(router)
	router2 := handlers.CompressHandler(router1)
	router3 := handlers.CustomLoggingHandler(os.Stdout, router2, customLogger)
	router4 := handlers.RecoveryHandler(handlers.PrintRecoveryStack(true))(router3)
	return router4
}

func runWeb(exitchan <-chan struct{}) {
	addr := cfg.GetDSString("0.0.0.0:3002", "web", "listen_addr")
	if addr == "" {
		log.Println("Not starting web server because listen address is empty")
		return
	}
	websrv := http.Server{
		Addr:    addr,
		Handler: createRouter(exitchan),
	}
	log.Println("Web server listens on " + addr)
	go func() {
		if err := websrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Web server returned an error: %s\n", err)
		}
	}()
	<-exitchan
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := websrv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}
}
