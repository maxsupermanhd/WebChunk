package main

import (
	"image"

	imagecache "github.com/maxsupermanhd/WebChunk/imageCache"
)

func imageCacheGetBlockingLoc(loc imagecache.ImageLocation) *image.RGBA {
	return ic.GetCachedImageBlocking(loc).Img
}

func imageCacheGetBlocking(wname, dname, variant string, cs, cx, cz int) *image.RGBA {
	return ic.GetCachedImageBlocking(imagecache.ImageLocation{
		World:     wname,
		Dimension: dname,
		Variant:   variant,
		S:         cs,
		X:         cx,
		Z:         cz,
	}).Img
}

func imageCacheSaveLoc(img *image.RGBA, loc imagecache.ImageLocation) {
	ic.SetCachedImage(loc, img)
}

func imageCacheSave(img *image.RGBA, wname, dname, variant string, cs, cx, cz int) {
	imageCacheSaveLoc(img, imagecache.ImageLocation{
		World:     wname,
		Dimension: dname,
		Variant:   variant,
		S:         cs,
		X:         cx,
		Z:         cz,
	})
}
