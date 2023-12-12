package main

import (
	"image"

	"github.com/maxsupermanhd/WebChunk/primitives"
)

func imageCacheGetBlockingLoc(loc primitives.ImageLocation) *image.RGBA {
	return ic.GetCachedImageBlocking(loc).Img
}

func imageCacheGetBlocking(wname, dname, variant string, cs, cx, cz int) *image.RGBA {
	return ic.GetCachedImageBlocking(primitives.ImageLocation{
		World:     wname,
		Dimension: dname,
		Variant:   variant,
		S:         cs,
		X:         cx,
		Z:         cz,
	}).Img
}

func imageCacheSaveLoc(img *image.RGBA, loc primitives.ImageLocation) {
	ic.SetCachedImage(loc, img)
}

func imageCacheSave(img *image.RGBA, wname, dname, variant string, cs, cx, cz int) {
	imageCacheSaveLoc(img, primitives.ImageLocation{
		World:     wname,
		Dimension: dname,
		Variant:   variant,
		S:         cs,
		X:         cx,
		Z:         cz,
	})
}
