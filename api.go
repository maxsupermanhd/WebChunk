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
	"io/ioutil"
	"log"
	"net/http"
	_ "os"
	_ "strconv"

	"github.com/Tnze/go-mc/save"
	_ "github.com/Tnze/go-mc/save/region"
	"github.com/gorilla/mux"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
)

func logreply(w http.ResponseWriter, status int, msg string) {
	w.Write([]byte(msg))
	log.Print(msg)
	w.WriteHeader(status)
}

func apiAddChunkHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	dname := params["dim"]
	wname := params["world"]
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logreply(w, http.StatusBadRequest, fmt.Sprintf("Error reading request: %s", err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var col save.Chunk
	err = col.Load(body)
	if err != nil {
		logreply(w, http.StatusBadRequest, fmt.Sprintf("Error parsing chunk data: %s", err))
		return
	}
	_, s, err := chunkStorage.GetWorldStorage(storages, wname)
	if err != nil {
		logreply(w, http.StatusInternalServerError, fmt.Sprintf("Error checking world: %s", err))
		return
	}
	if s == nil {
		logreply(w, http.StatusNotFound, fmt.Sprintf("World not found: %s", err))
		return
	}
	err = s.AddChunk(dname, wname, int(col.XPos), int(col.ZPos), col)
	if err != nil {
		logreply(w, http.StatusInternalServerError, fmt.Sprintf("Failed to add chunk to storage: %s", err.Error()))
		log.Printf("Failed to submit chunk %v:%v world %v dimension %v: %v", col.XPos, col.ZPos, wname, dname, err.Error())
		return
	}
	log.Print("Submitted chunk ", col.XPos, col.ZPos, " world ", wname, " dimension ", dname)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Chunk %d:%d of %s:%s submitted. Thank you for your contribution!\n", col.XPos, col.ZPos, wname, dname)))
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
				return 200, ""
			}
		}
	}
	return 404, ""
}
