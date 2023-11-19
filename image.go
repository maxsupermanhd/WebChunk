package main

// type imagingTask struct {
// 	target imagecache.ImageLocation
// 	in     chan imagecache.ImageCache
// 	out    chan *image.RGBA
// }

// var (
// 	imageScaleProcess = make(chan imagingTask, 256)
// )

// func imagingWorker(tasks <-chan imagingTask) {
// 	for t := range tasks {
// 		if t.target.S < imageCacheStorageLevel {
// 			from := <-t.in
// 			t.out <- imageScaleGetWithin(from.img, t.target)
// 		} else {
// 			log.Println("Unimplemented scaler > imageCacheStorageLevel")
// 		}
// 	}
// }

// func imagingProcessor(ctx context.Context) {
// 	var wg sync.WaitGroup

// 	sn := cfg.GetDSInt(4, "imaging_workers")
// 	wg.Add(sn)
// 	for i := 0; i < sn; i++ {
// 		go func() {
// 			imagingWorker(imageScaleProcess)
// 			wg.Done()
// 		}()
// 	}

// 	<-ctx.Done()
// 	log.Println("Image worker shutting down")
// 	close(imageScaleProcess)

// 	wg.Wait()
// 	log.Println("Image worker shutdown")
// }

// // gets subsection from image based in icIN
// func imageScaleGetWithin(from *image.RGBA, target imagecache.ImageLocation) *image.RGBA {
// 	// target image size
// 	is := powarr16[target.S]

// 	// TODO: probably reuse buffers with sync.Pool
// 	ret := image.NewRGBA(image.Rectangle{image.Point{0, 0}, image.Point{is, is}})

// 	// absolute position of the target
// 	ax, az := target.X*powarr[target.S], target.Z*powarr[target.S]

// 	// input relative position of the target
// 	ix, iz := icIN(ax, az)

// 	pt := image.Point{(ix / powarr[target.S]) * is, (iz / powarr[target.S]) * is}
// 	draw.Draw(ret, ret.Rect, from, pt, draw.Over)

// 	return ret
// }

// // stitches multiple images together
// func imageScaleGetFrom(from <-chan imageTask, target imagecache.ImageLocation) *image.RGBA {
// 	// TODO: probably reuse buffers with sync.Pool
// 	is := powarr16[target.S]

// 	ret := image.NewRGBA(image.Rectangle{image.Point{0, 0}, image.Point{is, is}})

// 	// TODO: scale down images
// 	return ret
// }

// func imageGetSync(loc imagecache.ImageLocation, ignoreCache bool, doDrawing bool) (*image.RGBA, error) {
// 	if !ignoreCache {
// 		i := imageCacheGetBlockingLoc(loc)
// 		if i != nil {
// 			return i, nil
// 		}
// 	}
// 	if doDrawing {

// 	}
// 	return nil, nil
// }
