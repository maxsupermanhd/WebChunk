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
	"image"
	"image/png"
	"os"
	"path"
	"path/filepath"
	"strconv"
)

func getImageCachePrefix() string {
	prefix := os.Getenv("CACHE_PREFIX")
	if prefix == "" {
		prefix = "imageCache"
	}
	return prefix
}

func saveImageCache(img *image.RGBA, world, dim, render string, s, x, z int) error {
	storePath := path.Join(".", getImageCachePrefix(), world, dim, render, strconv.Itoa(s), strconv.Itoa(x)+"x"+strconv.Itoa(z)+".png")
	err := os.MkdirAll(path.Dir(storePath), 0764)
	if err != nil {
		return err
	}
	file, err := os.Create(storePath)
	if err != nil {
		return err
	}
	err = png.Encode(file, img)
	if err != nil {
		return err
	}
	return file.Close()
}

func loadImageCache(world, dim, render string, s, x, z int) ([]byte, error) {
	prefix := os.Getenv("CACHE_PREFIX")
	if prefix == "" {
		prefix = "imageCache"
	}
	storePath := path.Join(".", getImageCachePrefix(), world, dim, render, strconv.Itoa(s), strconv.Itoa(x)+"x"+strconv.Itoa(z)+".png")
	return os.ReadFile(storePath)
}

func DirCountSize(path string) (count, size int64, err error) {
	err = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			count++
			size += info.Size()
		}
		return err
	})
	if os.IsNotExist(err) {
		return 0, 0, nil
	}
	return count, size, err
}

func getImageCacheCountSize(world, dim string) (int64, int64, error) {
	return DirCountSize(path.Join(".", getImageCachePrefix(), world, dim))
}
