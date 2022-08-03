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
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/Tnze/go-mc/nbt"
	_ "github.com/Tnze/go-mc/save/region"
	"github.com/davecgh/go-spew/spew"
	"github.com/gorilla/mux"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
)

func logChunkNbt(d []byte) {
	var err error
	var r io.Reader = bytes.NewReader(d[1:])
	switch d[0] {
	default:
		err = errors.New("unknown compression")
	case 1:
		r, err = gzip.NewReader(r)
	case 2:
		r, err = zlib.NewReader(r)
	}
	if err != nil {
		log.Println(err)
	} else {
		var sss map[string]interface{}
		dat, err := io.ReadAll(r)
		if err != nil {
			log.Println(err)
		}
		err = nbt.Unmarshal(dat, &sss)
		if err != nil {
			log.Println(err)
		}
		log.Print(spew.Sdump(sss))
		err = os.WriteFile("out.nbt", dat, 0666)
		if err != nil {
			log.Println(err)
		}

	}
}

func apiAddChunkHandler(w http.ResponseWriter, r *http.Request) (int, string) {
	params := mux.Vars(r)
	dname := params["dim"]
	wname := params["world"]
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return http.StatusBadRequest, fmt.Sprintf("Error reading request: %s", err)
	}
	col, err := chunkStorage.ConvFlexibleNBTtoSave(body)
	if err != nil {
		return http.StatusBadRequest, fmt.Sprintf("Error parsing chunk data: %s", err)
	}
	world, s, err := chunkStorage.GetWorldStorage(storages, wname)
	if err != nil {
		return http.StatusInternalServerError, fmt.Sprintf("Error checking world: %s", err)
	}
	if s == nil {
		s = findCapableStorage(storages, loadedConfig.API.FallbackStorageName)
		if s == nil {
			return http.StatusNotFound, fmt.Sprintf("Failed to find storage that has world [%s], named [%s] or has ability to add chunks, chunk [%d:%d] is LOST.", wname, loadedConfig.API.FallbackStorageName, col.XPos, col.ZPos)
		}
		world, err = s.AddWorld(wname, wname)
		if err != nil {
			return http.StatusInternalServerError, fmt.Sprintf("Error creating world in fallback storage: %s", err)
		}
	}
	if world == nil {
		if loadedConfig.API.CreateWorlds {
			world, err = s.AddWorld(wname, wname)
			if err != nil {
				return http.StatusInternalServerError, fmt.Sprintf("Error creating world: %s", err)
			}
		}
		return http.StatusInternalServerError, "Unable to find/create world"
	}
	dim, err := s.GetDimension(wname, dname)
	if err != nil {
		return http.StatusInternalServerError, fmt.Sprintf("Error checking dim: %s", err)
	}
	if dim == nil {
		if loadedConfig.API.CreateDimensions {
			dim, err = s.AddDimension(chunkStorage.DimStruct{
				Name:       dname,
				Alias:      dname,
				World:      world.Name,
				Spawnpoint: [3]int64{0, 0, 0},
				LowestY:    0,
				BuildLimit: 256,
			})
			if err != nil {
				return http.StatusInternalServerError, fmt.Sprintf("Error creating dim: %s", err)
			}
			if dim == nil {
				return http.StatusInternalServerError, "Tried to create dim but got nil"
			}
		} else {
			return http.StatusNotFound, fmt.Sprintf("Dimension not found: %s", err)
		}
	}
	err = s.AddChunkRaw(wname, dname, int64(col.XPos), int64(col.ZPos), body)
	if err != nil {
		log.Printf("Failed to submit chunk %v:%v world %v dimension %v: %v", col.XPos, col.ZPos, wname, dname, err.Error())
		return http.StatusInternalServerError, fmt.Sprintf("Failed to add chunk to storage: %s", err.Error())
	}
	log.Print("Submitted chunk ", col.XPos, col.ZPos, " world ", wname, " dimension ", dname)
	dTTYPE := r.Header.Get("WebChunk-DrawTTYPE")
	if dTTYPE != "" {
		var dPainter chunkPainterFunc
		if dTTYPE == "default" {
			for i := range ttypes {
				if i.IsDefault {
					dTTYPE = i.Name
					drawTTYPE := ttypes[i]
					_, dPainter = drawTTYPE(s)
					break
				}
			}
		} else {
			for i := range ttypes {
				if i.Name == dTTYPE {
					drawTTYPE := ttypes[i]
					_, dPainter = drawTTYPE(s)
					break
				}
			}
		}
		if dPainter == nil {
			return http.StatusBadRequest, "Requested terrain type not found!"
		}
		w.WriteHeader(http.StatusOK)
		img := dPainter(col)
		writeImage(w, "png", img)
		imageCacheSave(img, wname, dname, dTTYPE, 0, int64(col.XPos), int64(col.ZPos))
		return -1, ""
	}
	return http.StatusOK, fmt.Sprintf("Chunk %d:%d of %s:%s submitted. Thank you for your contribution!\n", col.XPos, col.ZPos, wname, dname)
}

func apiAddRegionHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
	// params := mux.Vars(r)
	// dids := params["did"]
	// did, err := strconv.Atoi(dids)
	// if err != nil {
	// 	errmsg := fmt.Sprintf("Bad dim id: %s", err)
	// 	w.Write([]byte(errmsg))
	// 	log.Print(errmsg)
	// 	w.WriteHeader(http.StatusBadRequest)
	// 	return
	// }
	// body, err := ioutil.ReadAll(r.Body)
	// if err != nil {
	// 	errmsg := fmt.Sprintf("Error reading request: %s", err)
	// 	w.Write([]byte(errmsg))
	// 	log.Print(errmsg)
	// 	w.WriteHeader(http.StatusBadRequest)
	// 	return
	// }
	// f, err := os.CreateTemp("", "upload")
	// if err != nil {
	// 	errmsg := fmt.Sprintf("Error creating region file: %s", err)
	// 	w.Write([]byte(errmsg))
	// 	log.Print(errmsg)
	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	return
	// }
	// defer os.Remove(f.Name())
	// if n, err := f.Write(body); err != nil || n != len(body) {
	// 	errmsg := fmt.Sprintf("Error writing region file: %s", err)
	// 	w.Write([]byte(errmsg))
	// 	log.Print(errmsg)
	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	return
	// }
	// if err := f.Close(); err != nil {
	// 	errmsg := fmt.Sprintf("Error closing region file: %s", err)
	// 	w.Write([]byte(errmsg))
	// 	log.Print(errmsg)
	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	return
	// }
	// region, err := region.Open(f.Name())
	// if err != nil {
	// 	errmsg := fmt.Sprintf("Error opening region file: %s", err)
	// 	w.Write([]byte(errmsg))
	// 	log.Print(errmsg)
	// 	w.WriteHeader(http.StatusBadRequest)
	// 	return
	// }
	// for x := 0; x < 32; x++ {
	// 	for z := 0; z < 32; z++ {
	// 		if !region.ExistSector(x, z) {
	// 			continue
	// 		}
	// 		data, err := region.ReadSector(x, z)
	// 		if err != nil {
	// 			log.Printf("Read sector (%d.%d) error: %v", x, z, err)
	// 		}
	// 		var col save.Column
	// 		col.Load(data)
	// 		tag, err := dbpool.Exec(context.Background(), `insert into chunks (dim, x, z, data) values ($1, $2, $3, $4)`, did, col.Level.PosX, col.Level.PosZ, data)
	// 		if err != nil {
	// 			w.WriteHeader(http.StatusInternalServerError)
	// 			log.Print(err.Error())
	// 			return
	// 		}
	// 		// log.Print("Submitted chunk ", col.Level.PosX, col.Level.PosZ)
	// 		if tag.RowsAffected() != 1 {
	// 			log.Print("Rows affected ", tag.RowsAffected())
	// 		}
	// 	}
	// }
	// region.Close()
	// w.WriteHeader(http.StatusOK)
	// w.Write([]byte(fmt.Sprintf("Region submitted. Thank you for your contribution!\n")))
	// return
}

func apiStoragesGET(w http.ResponseWriter, r *http.Request) (int, string) {
	ret := []struct {
		Name   string
		Type   string
		Online bool
	}{}
	for i := range storages {
		ret = append(ret, struct {
			Name   string
			Type   string
			Online bool
		}{
			Name:   storages[i].Name,
			Type:   storages[i].Type,
			Online: storages[i].Driver != nil,
		})
	}
	return marshalOrFail(200, ret)
}

func apiStorageReinit(w http.ResponseWriter, r *http.Request) (int, string) {
	sname := mux.Vars(r)["storage"]
	for i := range storages {
		if storages[i].Name == sname {
			if storages[i].Driver != nil {
				return 200, "Already initialized"
			} else {
				var err error
				storages[i].Driver, err = initStorage(storages[i].Type, storages[i].Address)
				if err != nil {
					return 500, err.Error()
				}
				var ver string
				ver, err = storages[i].Driver.GetStatus()
				if err != nil {
					return 500, err.Error()
				}
				return 200, ver
			}
		}
	}
	return 404, ""
}

func apiStorageAdd(w http.ResponseWriter, r *http.Request) (int, string) {
	name := r.FormValue("name")
	if name == "" {
		return 400, "Empty name"
	}
	address := r.FormValue("address")
	if address == "" {
		return 400, "Empty address"
	}
	t := r.FormValue("type")
	if t == "" {
		return 400, "Empty type"
	}
	for i := range storages {
		if storages[i].Name == name {
			return 400, "Storage with that name already exists"
		}
	}
	driver, err := initStorage(t, address)
	if err != nil {
		if err == errStorageTypeNotImplemented {
			return 400, err.Error()
		}
		return 500, err.Error()
	}
	ver, err := driver.GetStatus()
	if err != nil {
		return 500, err.Error()
	}
	storages = append(storages, chunkStorage.Storage{
		Name:    name,
		Type:    t,
		Address: address,
		Driver:  driver,
	})
	return 200, ver
}

func apiListRenderers(w http.ResponseWriter, r *http.Request) (int, string) {
	keys := make([]ttype, 0, len(ttypes))
	for t := range ttypes {
		keys = append(keys, t)
	}
	sort.Slice(keys, func(i, j int) bool { return strings.Compare(keys[i].Name, keys[j].Name) > 0 })
	return marshalOrFail(200, keys)
}
