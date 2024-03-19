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
	"os"
	"os/signal"
	"runtime/debug"
	rpprof "runtime/pprof"
	"syscall"

	"github.com/maxsupermanhd/WebChunk/chunkStorage"
	imagecache "github.com/maxsupermanhd/WebChunk/imageCache"
	"github.com/maxsupermanhd/WebChunk/proxy"
)

var (
	BuildTime  = "00000000.000000"
	CommitHash = "0000000"
	GoVersion  = "0.0"
	GitTag     = "0.0"
)

var (
	ic            *imagecache.ImageCache
	chunkChannel  = make(chan *proxy.ProxiedChunk, 12*12)
	mainCtxCancel context.CancelFunc
)

func main() {
	if err := loadConfig(); err != nil {
		log.Println("Error loading config file: " + err.Error())
		log.Println("Defaults will be used.")
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	if buildinfo, ok := debug.ReadBuildInfo(); ok {
		GoVersion = buildinfo.GoVersion
	}
	log.SetOutput(io.MultiWriter(createLogger(), os.Stdout))
	log.Println()
	log.Println("WebChunk server is starting up...")
	log.Printf("Built %s, Ver %s (%s) (%s)\n", BuildTime, GitTag, CommitHash, GoVersion)
	log.Println()

	profileCPU := cfg.GetDBool(false, "cpuprofile")
	if profileCPU {
		f, err := os.Create("webchunk.prof")
		if err != nil {
			log.Fatal(err)
		}
		rpprof.StartCPUProfile(f)
	}

	if err := storagesInit(); err != nil && cfg.GetDSBool(false, "ignore_failed_storages") {
		log.Fatal("Failed to initialize storages: ", err)
	}
	if err := loadColors(cfg.GetDSString("./colors.gob", "colors_path")); err != nil {
		log.Fatal(err)
	}

	var ctx context.Context
	ctx, mainCtxCancel = signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	bgsMetrics := startBackgroundRoutine("metrics dispatcher", metricsDispatcher)
	bgsEventRouter := startBackgroundRoutine("event router", globalEventRouter.Run)
	bgsTemplateManager := startBackgroundRoutine("template manager", func(ec <-chan struct{}) { templateManager(ec, cfg.SubTree("web")) })
	bgsChunkConsumer := startBackgroundRoutine("chunk consumer", chunkConsumer)
	bgsImageCache := startBackgroundRoutine("image cache", func(c <-chan struct{}) {
		imageCacheCtx, imageCacheCtxCancel := context.WithCancel(context.Background())
		go func() {
			<-c
			imageCacheCtxCancel()
		}()
		ic = imagecache.NewImageCache(log.Default(), cfg.SubTree("imageCache"), imageCacheCtx)
		ic.WaitExit()
	})

	bgsProxy := startBackgroundRoutine("proxy", func(c <-chan struct{}) {
		proxyCtx, proxyCtxCancel := context.WithCancel(context.Background())
		go func() {
			<-c
			proxyCtxCancel()
		}()
		proxy.RunProxy(proxyCtx, cfg.SubTree("proxy"), chunkChannel)
	})
	bgsWeb := startBackgroundRoutine("web server", runWeb)

	<-ctx.Done()
	log.Println("Interrupt recieved, shutting down...")

	bgsWeb()
	log.Println("Waiting for websocket clients to drop...")
	wsClients.Wait()

	bgsProxy()
	bgsImageCache()
	bgsChunkConsumer()
	bgsTemplateManager()
	bgsEventRouter()
	bgsMetrics()

	log.Println("Shutting down storages...")
	chunkStorage.CloseStorages(storages)
	log.Println("Storages closed.")

	if profileCPU {
		log.Println("Stopping profiler...")
		rpprof.StopCPUProfile()
	}
	log.Println("Shutdown complete, bye!")
}
