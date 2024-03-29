package imagecache

import (
	"container/list"
	"context"
	"image"
	"image/draw"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maxsupermanhd/WebChunk/primitives"
	"github.com/maxsupermanhd/lac"
)

var (
	powarr   = []int{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096}
	powarr16 = []int{1 * 16, 2 * 16, 4 * 16, 8 * 16, 16 * 16, 32 * 16, 64 * 16, 128 * 16, 256 * 16, 512 * 16, 1024 * 16, 2048 * 16, 4096 * 16}

// powarr16m1 = []int{1*16 - 1, 2*16 - 1, 4*16 - 1, 8*16 - 1, 16*16 - 1, 32*16 - 1, 64*16 - 1, 128*16 - 1, 256*16 - 1, 512*16 - 1, 1024*16 - 1, 2048*16 - 1, 4096*16 - 1}
)

const (
	StorageLevel           = int(5)
	DefaultTaskQueueLen    = int(256)
	DefaultIOProcessors    = int(4)
	DefaultIOTasksQueueLen = int(256)
)

func AT(cx, cz int) (int, int) {
	return cx >> StorageLevel, cz >> StorageLevel
}

func IN(cx, cz int) (int, int) {
	// return cx & powarr16m1[StorageLevel], cz & powarr16m1[StorageLevel]
	return cx & 31, cz & 31
}

type CachedImage struct {
	Img           *image.RGBA
	Loc           primitives.ImageLocation
	SyncedToDisk  bool
	lastUse       time.Time
	ModTime       time.Time
	imageUnloaded bool
}

type cacheTask struct {
	loc primitives.ImageLocation
	img *image.RGBA
	ret chan *CachedImage
}

type ImageCache struct {
	ctx                 context.Context
	logger              *log.Logger
	cfg                 *lac.ConfSubtree
	root                string
	tasks               chan *cacheTask
	ioTasks             chan *cacheTaskIO
	ioReturn            chan *cacheTaskIO
	cache               map[primitives.ImageLocation]*CachedImage
	cacheReturn         map[primitives.ImageLocation][]*cacheTask
	backlog             *list.List
	wg                  sync.WaitGroup
	cacheStatLen        atomic.Int64
	cacheStatUncommited atomic.Int64
}

func NewImageCache(logger *log.Logger, cfg *lac.ConfSubtree, ctx context.Context) *ImageCache {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	taskQueueLen := gtzero(logger, cfg, DefaultTaskQueueLen, "taskQueueLen")
	ioQueueLen := gtzero(logger, cfg, DefaultIOTasksQueueLen, "ioQueueLen")
	ioProcessors := gtzero(logger, cfg, DefaultIOProcessors, "ioProcessors")
	c := &ImageCache{
		ctx:         ctx,
		logger:      logger,
		cfg:         cfg,
		root:        cfg.GetDSString("cachedImages", "root"),
		tasks:       make(chan *cacheTask, taskQueueLen),
		ioTasks:     make(chan *cacheTaskIO, ioQueueLen),
		ioReturn:    make(chan *cacheTaskIO, ioQueueLen),
		cache:       map[primitives.ImageLocation]*CachedImage{},
		cacheReturn: map[primitives.ImageLocation][]*cacheTask{},
		backlog:     list.New(),
	}
	c.wg.Add(ioProcessors)
	for i := 0; i < ioProcessors; i++ {
		go func() {
			c.processorIO(c.ioTasks, c.ioReturn)
			c.wg.Done()
		}()
	}
	go c.processor()
	return c
}

func (c *ImageCache) WaitExit() {
	c.wg.Wait()
}

func (c *ImageCache) processor() {
	autosaveTimer := time.NewTicker(time.Duration(c.cfg.GetDSInt(15, "autosaveInterval")) * time.Second)
	unloadTimer := time.NewTicker(time.Duration(c.cfg.GetDSInt(10, "unloadInterval")) * time.Second)

processorLoop:
	for {
		select {
		case <-c.ctx.Done():
			break processorLoop
		case task := <-c.tasks:
			c.processTask(task)
		case ret := <-c.ioReturn:
			c.processReturn(ret)
		case <-autosaveTimer.C:
			c.processSave()
		case <-unloadTimer.C:
			c.processUnload()
		}
	}

	c.processSave()

	close(c.ioTasks)

	c.wg.Wait()
}

func (c *ImageCache) processUnload() {
	interval := time.Duration(c.cfg.GetDSInt(30, "unusedUnload")) * time.Second
	notsynced := int64(0)
	for k, v := range c.cache {
		if v.SyncedToDisk {
			if time.Since(v.lastUse) > interval {
				delete(c.cache, k)
			}
		} else {
			notsynced++
		}
	}
	c.cacheStatLen.Store(int64(len(c.cache)))
	c.cacheStatUncommited.Store(notsynced)
}

func (c *ImageCache) processTask(task *cacheTask) {
	if task.img == nil {
		c.processImageGet(task)
	} else {
		c.processImageSet(task)
	}
}

func (c *ImageCache) processImageGet(task *cacheTask) {
	if task.loc.S > StorageLevel {
		c.logger.Printf("Requested larger than storage level get (%s)", task.loc.String())
		task.ret <- &CachedImage{
			Img:     nil,
			Loc:     task.loc,
			lastUse: time.Time{},
			ModTime: time.Time{},
		}
		return
	}
	if task.loc.S == StorageLevel {
		c.processNativeImageGet(task)
	} else { // task.loc.S < StorageLevel
		c.processSmallerImageGet(task)
	}
}

func (c *ImageCache) processNativeImageGet(task *cacheTask) {
	l, ok := c.cache[task.loc]
	if ok {
		c.logger.Printf("Processing native image get, cache hit %s", task.loc.String())
		task.ret <- copyCachedImage(l)
		return
	}
	c.logger.Printf("Processing native image get, not in cache, scheduling io %s", task.loc.String())
	r, ok := c.cacheReturn[task.loc]
	if ok {
		r = append(r, task)
	} else {
		r = []*cacheTask{task}
	}
	c.cacheReturn[task.loc] = r
	c.ioTasks <- &cacheTaskIO{
		loc: task.loc,
		img: nil,
		err: nil,
	}
}

func (c *ImageCache) processSmallerImageGet(task *cacheTask) {
	loc := getStorageLevelLoc(task.loc)
	l, ok := c.cache[loc]
	if ok {
		if l.imageUnloaded {
			c.logger.Printf("Processing smaller image get, io waiting on %s for %s", loc.String(), task.loc.String())
		} else {
			c.logger.Printf("Processing smaller image get, cache hit %s", task.loc.String())
			task.ret <- copySmallerCachedImage(l, task.loc)
			return
		}
	} else {
		c.logger.Printf("Processing smaller image get, not in cache, scheduling io %s for %s", loc.String(), task.loc.String())
		c.ioTasks <- &cacheTaskIO{
			loc: loc,
			img: nil,
			err: nil,
		}
		l = &CachedImage{
			Img:           nil,
			Loc:           loc,
			lastUse:       time.Now(),
			imageUnloaded: true,
		}
		c.cache[loc] = l
		c.cacheStatLen.Add(1)
	}
	r, ok := c.cacheReturn[loc]
	if ok {
		r = append(r, task)
	} else {
		r = []*cacheTask{task}
	}
	c.cacheReturn[loc] = r
}

func getStorageLevelLoc(loc primitives.ImageLocation) primitives.ImageLocation {
	rx, rz := AT(loc.X*powarr[loc.S], loc.Z*powarr[loc.S])
	return primitives.ImageLocation{
		World:     loc.World,
		Dimension: loc.Dimension,
		Variant:   loc.Variant,
		S:         StorageLevel,
		X:         rx,
		Z:         rz,
	}
}

func copySmallerCachedImage(img *CachedImage, target primitives.ImageLocation) *CachedImage {
	return &CachedImage{
		Img:           copyFragmentRGBA(img.Img, target),
		Loc:           img.Loc,
		SyncedToDisk:  img.SyncedToDisk,
		lastUse:       img.lastUse,
		ModTime:       img.ModTime,
		imageUnloaded: false,
	}
}

func copyFragmentRGBA(from *image.RGBA, target primitives.ImageLocation) *image.RGBA {
	if from == nil {
		return nil
	}
	ax, az := IN(target.X*powarr[target.S], target.Z*powarr[target.S])
	to := image.NewRGBA(image.Rect(0, 0, powarr16[target.S], powarr16[target.S]))
	draw.DrawMask(to, to.Rect, from, image.Point{X: ax * 16, Y: az * 16}, nil, image.Point{}, draw.Src)
	return to
}

func copyCachedImage(img *CachedImage) *CachedImage {
	return &CachedImage{
		Img:           copyRGBA(img.Img),
		SyncedToDisk:  img.SyncedToDisk,
		lastUse:       img.lastUse,
		ModTime:       img.ModTime,
		imageUnloaded: false,
	}
}

func copyRGBA(from *image.RGBA) *image.RGBA {
	if from == nil {
		return nil
	}
	dx := from.Rect.Dx()
	dy := from.Rect.Dy()
	to := image.NewRGBA(image.Rect(0, 0, dx, dy))
	draw.DrawMask(to, to.Rect, from, image.Point{}, nil, image.Point{}, draw.Src)
	return to
}

func (c *ImageCache) processImageSet(task *cacheTask) {
	if task.loc.S != StorageLevel && task.loc.S != 0 {
		c.logger.Printf("Set of non-native and non-zero scaled image %s", task.loc.String())
		return
	}
	t, ok := c.cache[task.loc]
	if !ok {
		c.ioTasks <- &cacheTaskIO{
			loc: task.loc,
			img: nil,
			err: nil,
		}
		t = &CachedImage{
			Img:           image.NewRGBA(image.Rect(0, 0, 512, 512)),
			Loc:           getStorageLevelLoc(task.loc),
			lastUse:       time.Now(),
			imageUnloaded: true,
		}
		c.cache[task.loc] = t
		c.cacheStatUncommited.Add(1)
		c.cacheStatLen.Add(1)
	}
	t.SyncedToDisk = false
	if t.Img == nil {
		t.Img = image.NewRGBA(image.Rect(0, 0, 512, 512))
	}
	if task.loc.S == 0 {
		rx, rz := IN(task.loc.X, task.loc.Z)
		r := image.Rect(rx*16, rz*16, rx*16+16, rz*16+16)
		draw.Draw(t.Img, r, task.img, image.Point{}, draw.Src)
	} else if task.loc.S == StorageLevel {
		draw.Draw(t.Img, t.Img.Rect, task.img, image.Point{}, draw.Src)
	}
}

func (c *ImageCache) processReturn(task *cacheTaskIO) {
	if task.err != nil {
		c.logger.Printf("Error reading image at %s", task.loc.String())
		return
	}
	t, ok := c.cache[task.loc]
	if !ok {
		c.cache[task.loc] = task.img
		c.cacheStatUncommited.Add(1)
		c.cacheStatLen.Add(1)
	} else {
		c.processCacheLoad(t, task)
	}

	ret, ok := c.cacheReturn[task.loc]
	if !ok {
		c.logger.Printf("Unexpected IO return at %s", task.loc.String())
		return
	}
	for _, v := range ret {
		c.processTask(v)
	}
	delete(c.cacheReturn, task.loc)
}

func (c *ImageCache) processCacheLoad(t *CachedImage, task *cacheTaskIO) {
	if task.img == nil || task.img.Img == nil {
		t.imageUnloaded = false
		return
	}
	if !t.imageUnloaded {
		c.logger.Printf("IO return at %s but already have loaded image in cache", task.loc.String())
		return
	}
	if t.Img != nil {
		draw.Draw(task.img.Img, task.img.Img.Bounds(), t.Img, image.Point{}, draw.Src)
	}
	t.Img = task.img.Img
	t.imageUnloaded = false
	t.SyncedToDisk = true
}

func (c *ImageCache) SetCachedImage(loc primitives.ImageLocation, img *image.RGBA) {
	if img == nil {
		return // dumbass
	}
	c.tasks <- &cacheTask{
		loc: loc,
		img: img,
		ret: nil,
	}
}

func (c *ImageCache) GetCachedImageBlocking(loc primitives.ImageLocation) *CachedImage {
	ret := make(chan *CachedImage)
	c.tasks <- &cacheTask{
		loc: loc,
		img: nil,
		ret: ret,
	}
	return <-ret
}

func (c *ImageCache) GetCachedImage(loc primitives.ImageLocation, ret chan *CachedImage) {
	if ret == nil {
		return // wtf do you expect?
	}
	c.tasks <- &cacheTask{
		loc: loc,
		img: nil,
		ret: ret,
	}
}

func (c *ImageCache) GetCachedImageModTime(loc primitives.ImageLocation) time.Time {
	return c.getModTimeLoc(loc)
}

func (c *ImageCache) GetStats() map[string]any {
	return map[string]any{
		"root":                c.root,
		"io queue capacity":   cap(c.ioTasks),
		"io queue length":     len(c.ioTasks),
		"task queue capacity": cap(c.tasks),
		"task queue length":   len(c.tasks),
		"cached images":       c.cacheStatLen.Load(),
		"unwritten images":    c.cacheStatUncommited.Load(),
	}
}

func gtzero(l *log.Logger, c *lac.ConfSubtree, d int, p ...string) int {
	v := c.GetDSInt(d, p...)
	if v > 0 {
		return v
	}
	l.Printf("Negative %v, defaulting to %d!", p, d)
	return d
}
