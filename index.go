package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
)

var prevCPUIdle uint64
var prevCPUTotal uint64
var prevTime = time.Now()
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
		Dim        chunkStorage.SDim
		ChunkSize  string
		ChunkCount uint64
		CacheSize  string
		CacheCount int64
	}
	type WorldData struct {
		World chunkStorage.SWorld
		Dims  []DimData
	}
	type StorageData struct {
		Name   string
		S      chunkStorage.Storage
		Worlds []WorldData
		Online bool
	}
	st := []StorageData{}
	for sn, s := range storages {
		worlds := []WorldData{}
		if s.Driver == nil {
			st = append(st, StorageData{Name: sn, S: s, Worlds: worlds, Online: false})
			// log.Println("Skipping storage " + s.Name + " because driver is uninitialized")
			continue
		}
		achunksCount, _ := s.Driver.GetChunksCount()
		achunksSizeBytes, _ := s.Driver.GetChunksSize()
		chunksCount += achunksCount
		chunksSizeBytes += achunksSizeBytes
		worldss, err := s.Driver.ListWorlds()
		if err != nil {
			plainmsg(w, r, plainmsgColorRed, "Error listing worlds of storage "+sn+": "+err.Error())
			return
		}
		for _, wrld := range worldss {
			wd := WorldData{World: wrld, Dims: []DimData{}}
			dims, err := s.Driver.ListWorldDimensions(wrld.Name)
			if err != nil {
				plainmsg(w, r, plainmsgColorRed, "Error listing dimensions of world "+wrld.Name+" of storage "+sn+": "+err.Error())
				return
			}
			for _, dim := range dims {
				dimChunksCount, err := s.Driver.GetDimensionChunksCount(wrld.Name, dim.Name)
				if err != nil {
					plainmsg(w, r, plainmsgColorRed, "Error getting chunk count of dim "+dim.Name+" of world "+wrld.Name+" of storage "+sn+": "+err.Error())
					return
				}
				dimChunksSize, err := s.Driver.GetDimensionChunksSize(wrld.Name, dim.Name)
				if err != nil {
					plainmsg(w, r, plainmsgColorRed, "Error getting chunks size of dim "+dim.Name+" of world "+wrld.Name+" of storage "+sn+": "+err.Error())
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
		st = append(st, StorageData{Name: sn, S: s, Worlds: worlds, Online: true})
	}
	chunksSize := humanize.Bytes(chunksSizeBytes)
	templateRespond("index", w, r, map[string]interface{}{
		"LoadAvg":     load,
		"VirtMem":     virtmem,
		"Uptime":      uptimetime,
		"ChunksCount": chunksCount,
		"ChunksSize":  chunksSize,
		"CPUReport":   CPUReport,
		"Storages":    st,
	})
}

func getCPUSample() (idle, total uint64) {
	contents, err := os.ReadFile("/proc/stat")
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
