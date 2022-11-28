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
	"os"
	"path"
	"time"

	"github.com/Tnze/go-mc/save"
	"github.com/maxsupermanhd/WebChunk/chunkStorage"
)

//lint:ignore U1000 Perhaps for later
func dirExists(p string) bool {
	fi, err := os.Stat(p)
	if err == nil {
		return fi.IsDir()
	} else {
		return false
	}
}

func dirModtime(p string) time.Time {
	fi, err := os.Stat(p)
	if err == nil {
		return fi.ModTime()
	}
	return time.Time{}
}

func (s *FilesystemChunkStorage) ListWorldDimensions(wname string) ([]chunkStorage.SDim, error) {
	wpath := path.Join(s.Root, wname)
	dims := []chunkStorage.SDim{}
	winfo, err := os.Stat(wpath)
	if err != nil || !winfo.IsDir() {
		return dims, chunkStorage.ErrNoWorld
	}
	dims = []chunkStorage.SDim{
		{
			Name:       "overworld",
			World:      wname,
			Data:       save.DefaultDimensionsTypes["minecraft:overworld"],
			ModifiedAt: winfo.ModTime(),
		},
		{
			Name:       "the_nether",
			World:      wname,
			Data:       save.DefaultDimensionsTypes["minecraft:the_nether"],
			ModifiedAt: dirModtime(path.Join(wpath, "DIM-1")),
		},
		{
			Name:       "the_end",
			World:      wname,
			Data:       save.DefaultDimensionsTypes["minecraft:the_end"],
			ModifiedAt: dirModtime(path.Join(wpath, "DIM1")),
		},
	}
	return dims, nil
}

func (s *FilesystemChunkStorage) ListDimensions() ([]chunkStorage.SDim, error) {
	dims := []chunkStorage.SDim{}
	wnames, err := s.ListWorldNames()
	if err != nil {
		return dims, err
	}
	for _, wname := range wnames {
		d, err := s.ListWorldDimensions(wname)
		if err != nil {
			dims = append(dims, d...)
		}
	}
	return dims, nil
}

func (s *FilesystemChunkStorage) AddDimension(wname string, dim chunkStorage.SDim) error {
	return chunkStorage.ErrNotImplemented
}

func (s *FilesystemChunkStorage) GetDimension(wname, dname string) (*chunkStorage.SDim, error) {
	wpath := path.Join(s.Root, wname)
	winfo, err := os.Stat(wpath)
	if err != nil || !winfo.IsDir() {
		return nil, chunkStorage.ErrNoWorld
	}
	switch dname {
	case "overworld":
		return &chunkStorage.SDim{
			Name:       "overworld",
			World:      wname,
			ModifiedAt: winfo.ModTime(),
			Data:       save.DefaultDimensionsTypes["overworld"],
		}, nil
	case "the_nether":
		return &chunkStorage.SDim{
			Name:       "the_nether",
			World:      wname,
			ModifiedAt: dirModtime(path.Join(wpath, "DIM-1")),
			Data:       save.DefaultDimensionsTypes["the_nether"],
		}, nil
	case "the_end":
		return &chunkStorage.SDim{
			Name:       "the_end",
			World:      wname,
			ModifiedAt: dirModtime(path.Join(wpath, "DIM1")),
			Data:       save.DefaultDimensionsTypes["the_end"],
		}, nil
	default:
		return nil, chunkStorage.ErrNoDim
	}
}

func (s *FilesystemChunkStorage) SetDimensionData(wname, dname string, data save.DimensionType) error {
	return chunkStorage.ErrNotImplemented
}
