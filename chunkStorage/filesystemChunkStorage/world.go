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

package FilesystemChunkStorage

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/Tnze/go-mc/save"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
)

func readSaveLevel(root string) (save.Level, error) {
	ret := save.Level{}
	s, err := os.Stat(root)
	if err != nil {
		return ret, err
	}
	if !s.IsDir() {
		return ret, fmt.Errorf("specified root points to file, not directory with world")
	}
	e, err := ioutil.ReadDir(root)
	if err != nil {
		return ret, err
	}
	fnames := map[string]fs.FileInfo{}
	for _, f := range e {
		fnames[f.Name()] = f
	}
	if leveldat, ok := fnames["level.dat"]; ok {
		if leveldat.IsDir() {
			return ret, fmt.Errorf("level.dat is a directory")
		}
		f, err := os.Open(path.Join(root, "level.dat"))
		if err != nil {
			return ret, err
		}
		return save.ReadLevel(f)
	} else {
		return ret, fmt.Errorf("level.dat not found in [%s]", root)
	}
}

func (s *FilesystemChunkStorage) ListWorlds() ([]chunkStorage.WorldStruct, error) {
	worlds := []chunkStorage.WorldStruct{}
	levels := []save.Level{}
	lev, err := readSaveLevel(s.Root)
	if err != nil {
		e, err := ioutil.ReadDir(s.Root)
		if err != nil {
			return nil, err
		}
		atLeastOne := false
		for _, f := range e {
			if f.IsDir() {
				l, err := readSaveLevel(path.Join(s.Root, f.Name()))
				if err == nil {
					atLeastOne = true
					levels = append(levels, l)
				}
			}
		}
		if atLeastOne {
			err = nil
		}
	} else {
		levels = append(levels, lev)
	}
	sort.Slice(levels, func(i, j int) bool {
		return strings.Compare(levels[i].Data.LevelName, levels[j].Data.LevelName) > 0
	})
	for i := range levels {
		worlds = append(worlds, chunkStorage.WorldStruct{
			IP:   "local-" + levels[i].Data.LevelName,
			Name: levels[i].Data.LevelName,
		})
	}
	return worlds, err
}

// func (s *FilesystemChunkStorage) GetWorldByID(sid int) (chunkStorage.WorldStruct, error) {
// 	world := chunkStorage.WorldStruct{}
// 	worlds, err := s.ListWorlds()
// 	if err != nil {
// 		return world, err
// 	}
// 	for _, v := range worlds {
// 		if v.ID == sid {
// 			world = v
// 			break
// 		}
// 	}
// 	return world, err
// }

func (s *FilesystemChunkStorage) GetWorldByName(worldname string) (chunkStorage.WorldStruct, error) {
	world := chunkStorage.WorldStruct{}
	worlds, err := s.ListWorlds()
	if err != nil {
		return world, err
	}
	for _, v := range worlds {
		if v.Name == worldname {
			world = v
			break
		}
	}
	return world, err
}

func (s *FilesystemChunkStorage) AddWorld(name, ip string) (chunkStorage.WorldStruct, error) {
	world := chunkStorage.WorldStruct{}
	return world, fmt.Errorf("not implemented")
}
