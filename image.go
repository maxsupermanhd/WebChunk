package main

import (
	"image"
	"image/draw"
	"log"
	"runtime/debug"

	"github.com/maxsupermanhd/WebChunk/chunkStorage"
	"github.com/maxsupermanhd/WebChunk/primitives"
	"github.com/nfnt/resize"
)

func imageGetSync(loc primitives.ImageLocation, ignoreCache bool) (*image.RGBA, error) {
	if !ignoreCache {
		i := imageCacheGetBlockingLoc(loc)
		if i != nil {
			return i, nil
		}
	}
	img, err := renderTile(loc)
	if err != nil {
		return img, err
	}
	if img != nil {
		imageCacheSaveLoc(img, loc)
	}
	return img, err
}

func renderTile(loc primitives.ImageLocation) (*image.RGBA, error) {

	f := findTTypeProviderFunc(loc)
	if f == nil {
		log.Printf("Image variant %q was not found", loc.Variant)
		return nil, nil
	}
	ff := *f

	_, s, err := chunkStorage.GetWorldStorage(storages, loc.World)
	if err != nil {
		return nil, nil
	}
	getter, painter := ff(s)

	scale := 1
	if loc.S > 0 {
		scale = int(2 << (loc.S - 1)) // because math.Pow is very slow (43.48 vs 0.1881 ns/op)
	}

	imagesize := scale * 16
	if imagesize > 512 {
		imagesize = 512
	}

	img := image.NewRGBA(image.Rect(0, 0, int(imagesize), int(imagesize)))
	imagescale := int(imagesize / scale)
	offsetx := loc.X * scale
	offsety := loc.Z * scale
	cc, err := getter(loc.World, loc.Dimension, loc.X*scale, loc.Z*scale, loc.X*scale+scale, loc.Z*scale+scale)
	if err != nil {
		return nil, err
	}
	if len(cc) == 0 {
		return nil, nil
	}
	for _, c := range cc {
		// TODO: break on cancel
		placex := int(c.X - offsetx)
		placey := int(c.Z - offsety)
		var chunk *image.RGBA
		chunk = func(d interface{}) *image.RGBA {
			defer func() {
				if err := recover(); err != nil {
					log.Println(loc.X, loc.Z, err) // TODO: pass error outwards
					debug.PrintStack()
				}
				chunk = nil
			}()
			var ret *image.RGBA
			ret = nil
			ret = painter(d)
			return ret
		}(c.Data)
		if chunk == nil {
			continue
		}
		tile := resize.Resize(uint(imagescale), uint(imagescale), chunk, resize.NearestNeighbor)
		draw.Draw(img, image.Rect(placex*int(imagescale), placey*int(imagescale), placex*int(imagescale)+imagescale, placey*int(imagescale)+imagescale),
			tile, image.Pt(0, 0), draw.Over)
	}
	return img, nil
}

func findTTypeProviderFunc(loc primitives.ImageLocation) *ttypeProviderFunc {
	for tt := range ttypes {
		if tt.Name == loc.Variant {
			f := ttypes[tt]
			return &f // TODO: fix this ugly thing
		}
	}
	return nil
}
