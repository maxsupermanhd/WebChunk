package imagecache

import (
	"image"
	"image/draw"
	"image/png"
	"os"
	"path"
	"strconv"
	"time"
)

type cacheTaskIO struct {
	loc ImageLocation
	img *CachedImage
	err error
}

func (c *ImageCache) processorIO(in <-chan *cacheTaskIO, out chan<- *cacheTaskIO) {
	for task := range in {
		if task.img == nil {
			task.img, task.err = c.cacheLoad(task.loc)
		} else {
			task.err = c.cacheSave(task.img.Img, task.loc)
		}
		out <- task
	}
}

func (c *ImageCache) processSave() {
	for k, v := range c.cache {
		if v.SyncedToDisk {
			continue
		}
		err := c.cacheSave(v.Img, k)
		if err != nil {
			c.logger.Printf("Failed to save cache of %s (%s): %v", k.String(), c.cacheGetFilenameLoc(k), err)
			continue
		}
	}
}

func (c *ImageCache) cacheGetFilename(world, dim, variant string, s, x, z int) string {
	return path.Join(".", c.root, world, dim, variant, strconv.FormatInt(int64(s), 10), strconv.FormatInt(int64(x), 10)+"x"+strconv.FormatInt(int64(z), 10)+".png")
}

func (c *ImageCache) cacheGetFilenameLoc(loc ImageLocation) string {
	return c.cacheGetFilename(loc.World, loc.Dimension, loc.Variant, loc.S, loc.X, loc.Z)
}

func (c *ImageCache) cacheSave(img *image.RGBA, loc ImageLocation) error {
	storePath := c.cacheGetFilenameLoc(loc)
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

func (c *ImageCache) cacheLoad(loc ImageLocation) (*CachedImage, error) {
	fp := c.cacheGetFilenameLoc(loc)
	f, err := os.Open(fp)
	if err != nil {
		if os.IsNotExist(err) { // weird
			return &CachedImage{
				Img:           nil,
				Loc:           loc,
				SyncedToDisk:  true,
				lastUse:       time.Now(),
				ModTime:       time.Time{},
				imageUnloaded: false,
			}, nil
		}
		return nil, err
	}
	defer f.Close()
	ii, err := png.Decode(f)
	if err != nil {
		os.Remove(fp)
		return nil, err
	}
	if iirgba, ok := ii.(*image.RGBA); ok {
		return &CachedImage{
			Img:          iirgba,
			Loc:          loc,
			SyncedToDisk: true,
			lastUse:      time.Now(),
			ModTime:      c.getModTimeFp(fp),
		}, nil
	}
	b := ii.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), ii, b.Min, draw.Src)
	return &CachedImage{
		Img:          dst,
		Loc:          loc,
		SyncedToDisk: true,
		lastUse:      time.Now(),
		ModTime:      c.getModTimeFp(fp),
	}, nil
}

func (c *ImageCache) getModTimeLoc(loc ImageLocation) time.Time {
	return c.getModTimeFp(c.cacheGetFilenameLoc(loc))
}

func (c *ImageCache) getModTimeFp(fp string) time.Time {
	info, err := os.Stat(fp)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
