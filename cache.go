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

func saveImageCache(img *image.RGBA, server, dim, render string, s, x, z int) error {
	storePath := path.Join(".", getImageCachePrefix(), server, dim, render, strconv.Itoa(s), strconv.Itoa(x)+"x"+strconv.Itoa(z)+".png")
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

func loadImageCache(server, dim, render string, s, x, z int) ([]byte, error) {
	prefix := os.Getenv("CACHE_PREFIX")
	if prefix == "" {
		prefix = "imageCache"
	}
	storePath := path.Join(".", getImageCachePrefix(), server, dim, render, strconv.Itoa(s), strconv.Itoa(x)+"x"+strconv.Itoa(z)+".png")
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

func getImageCacheCountSize(server, dim string) (int64, int64, error) {
	return DirCountSize(path.Join(".", getImageCachePrefix(), server, dim))
}
