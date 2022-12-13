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

package filesystemChunkStorage

import (
	"encoding/json"
	"log"
	"os"
	"path"

	"github.com/Tnze/go-mc/save"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
)

func (s *FilesystemChunkStorage) ListWorlds() ([]chunkStorage.SWorld, error) {
	worlds := []chunkStorage.SWorld{}
	dir, err := os.ReadDir(s.Root)
	for _, d := range dir {
		if !d.IsDir() {
			continue
		}
		w, err := s.GetWorld(d.Name())
		if err != nil {
			log.Printf("Failed to get world [%s]", err)
		}
		worlds = append(worlds, *w)
	}
	return worlds, err
}

func (s *FilesystemChunkStorage) ListWorldNames() ([]string, error) {
	dir, err := os.ReadDir(s.Root)
	if err != nil {
		return []string{}, err
	}
	names := []string{}
	for _, d := range dir {
		if !d.IsDir() {
			continue
		}
		if checkValidWorld(path.Join(s.Root, d.Name())) {
			names = append(names, d.Name())
		}
	}
	return names, nil
}

func (s *FilesystemChunkStorage) GetWorld(wname string) (*chunkStorage.SWorld, error) {
	wdir := path.Join(s.Root, wname)
	if _, err := os.Stat(wdir); os.IsNotExist(err) {
		return nil, nil
	}
	var w chunkStorage.SWorld
	w.Name = wname
	meta, err := readWorldMeta(wdir)
	if err != nil {
		log.Printf("Failed to read world meta file for world [%s]: %v", wname, err)
	} else {
		w.Alias = meta.Alias
		w.IP = meta.IP
	}
	data, err := readSaveLevel(path.Join(wdir, "level.dat"))
	if err != nil {
		log.Printf("Failed to read world data for world [%s]: %v", wname, err)
		return nil, err
	} else {
		w.Data = *data
	}
	return &w, nil

}

func (s *FilesystemChunkStorage) AddWorld(world chunkStorage.SWorld) error {
	wpath := path.Join(s.Root, world.Name)
	err := os.MkdirAll(wpath, 0777)
	if err != nil {
		return err
	}
	err = writeWorldMeta(wpath, worldMeta{
		Alias: world.Alias,
		IP:    world.IP,
	})
	if err != nil {
		return err
	}
	return writeSaveLevel(wpath, world.Data)
}

func (s *FilesystemChunkStorage) SetWorldAlias(wname, newalias string) error {
	wpath := path.Join(s.Root, wname)
	meta, err := readWorldMeta(wpath)
	if err != nil {
		return err
	}
	meta.Alias = newalias
	return writeWorldMeta(wpath, *meta)

}

func (s *FilesystemChunkStorage) SetWorldIP(wname, newip string) error {
	wpath := path.Join(s.Root, wname)
	meta, err := readWorldMeta(wpath)
	if err != nil {
		return err
	}
	meta.IP = newip
	return writeWorldMeta(wpath, *meta)
}

func (s *FilesystemChunkStorage) SetWorldData(wname string, data save.LevelData) error {
	return writeSaveLevel(s.GetWorldPath(wname), data)
}

func (s *FilesystemChunkStorage) GetWorldPath(wname string) string {
	return path.Join(s.Root, wname)
}

type worldMeta struct {
	Alias string
	IP    string
}

func getWorldDirMetaPath(wdir string) string {
	return path.Join(wdir, "WebChunk.json")
}

func readWorldMeta(wdir string) (*worldMeta, error) {
	var d worldMeta
	b, err := os.ReadFile(getWorldDirMetaPath(wdir))
	if err != nil {
		if os.IsNotExist(err) {
			return &d, writeWorldMeta(wdir, d)
		}
		return nil, err
	}
	err = json.Unmarshal(b, &d)
	if err != nil {
		return nil, err
	}
	return &d, err
}

func writeWorldMeta(wdir string, d worldMeta) error {
	b, err := json.MarshalIndent(d, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(getWorldDirMetaPath(wdir), b, 0666)
}
