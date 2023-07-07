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
	"errors"
	"log"
	"sync"

	"github.com/maxsupermanhd/WebChunk/chunkStorage"
	"github.com/maxsupermanhd/WebChunk/chunkStorage/filesystemChunkStorage"
	"github.com/maxsupermanhd/WebChunk/chunkStorage/postgresChunkStorage"
	"github.com/maxsupermanhd/lac"
)

var (
	errStorageTypeNotImplemented = errors.New("storage type not implemented")
	storages                     map[string]chunkStorage.Storage
	storagesLock                 sync.Mutex
)

func storagesInit() error {
	log.Println("Initializing storages...")
	err := cfg.GetToStruct(&storages, "storages")
	if err != nil && !errors.Is(err, lac.ErrNoKey) {
		return err
	}
	if len(storages) == 0 {
		log.Println("No storages to initialize")
		cfg.Set(map[string]any{}, "storages")
		return nil
	}
	for k, v := range storages {
		d, err := newStorage(storages[k].Type, storages[k].Address)
		if err != nil {
			log.Println("Failed to initialize storage: " + err.Error())
			continue
		}
		ver, err := d.GetStatus()
		if err != nil {
			log.Println("Error getting storage status: " + err.Error())
			continue
		}
		v.Driver = d
		storages[k] = v
		log.Println("Storage initialized: " + ver)
	}
	return nil
}

func newStorage(storageStype, address string) (driver chunkStorage.ChunkStorage, err error) {
	switch storageStype {
	case "postgres":
		driver, err = postgresChunkStorage.NewPostgresChunkStorage(context.Background(), address)
		if err != nil {
			return nil, err
		}
		return driver, nil
	case "filesystem":
		driver, err = filesystemChunkStorage.NewFilesystemChunkStorage(address)
		if err != nil {
			return nil, err
		}
		return driver, nil
	default:
		return nil, errStorageTypeNotImplemented
	}
}

func findCapableStorage(storages map[string]chunkStorage.Storage, pref string) chunkStorage.ChunkStorage {
	p, ok := storages[pref]
	if ok {
		return p.Driver
	}
	for sn, s := range storages {
		if s.Driver == nil || sn == "" {
			continue
		}
		a := s.Driver.GetAbilities()
		if a.CanCreateWorldsDimensions && a.CanAddChunks {
			return s.Driver
		}
	}
	return nil
}

func listNamesWnD() map[string][]string {
	worlds := map[string][]string{}
	storagesLock.Lock()
	defer storagesLock.Unlock()
	for _, storage := range storages {
		if storage.Driver == nil {
			continue
		}
		dims, err := storage.Driver.ListDimensions()
		if err != nil {
			log.Println(err)
			continue
		}
		for _, d := range dims {
			world, ok := worlds[d.World]
			if ok {
				world = append(world, d.Name)
			} else {
				world = []string{d.Name}
			}
			worlds[d.World] = world
		}
	}
	return worlds
}
