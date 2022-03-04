package main

import (
	"image"
	"image/png"
	"os"
	"path"
	"strconv"
)

func saveImageCache(img *image.RGBA, server, dim, render string, s, x, z int) error {
	prefix := os.Getenv("CACHE_PREFIX")
	if prefix == "" {
		prefix = "imageCache"
	}
	storePath := path.Join(".", prefix, server, dim, render, strconv.Itoa(s), strconv.Itoa(x)+"x"+strconv.Itoa(z)+".png")
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
	storePath := path.Join(".", prefix, server, dim, render, strconv.Itoa(s), strconv.Itoa(x)+"x"+strconv.Itoa(z)+".png")
	return os.ReadFile(storePath)
}
