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
)

func apiAddChunkHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	dname := params["dim"]
	sname := params["server"]
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		errmsg := fmt.Sprintf("Error reading request: %s", err)
		w.Write([]byte(errmsg))
		log.Print(errmsg)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var col save.Chunk
	err = col.Load(body)
	if err != nil {
		errmsg := fmt.Sprintf("Error parsing chunk data: %s", err)
		w.Write([]byte(errmsg))
		log.Print(errmsg)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	err = storage.AddChunk(dname, sname, col.XPos, col.ZPos, col)
	if err != nil {
		log.Printf("Failed to submit chunk %v:%v server %v dimension %v: %v", col.XPos, col.ZPos, sname, dname, err.Error())
		w.WriteHeader(http.StatusInternalServerError)
	}
	log.Print("Submitted chunk ", col.XPos, col.ZPos, " server ", sname, " dimension ", dname)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Chunk %d:%d of %s:%s submitted. Thank you for your contribution!\n", col.XPos, col.ZPos, sname, dname)))
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
