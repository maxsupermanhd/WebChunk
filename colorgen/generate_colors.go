package main

import (
	"archive/zip"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/Tnze/go-mc/level/block"
	"github.com/davecgh/go-spew/spew"
)

var (
	JARpath = `/home/max/.minecraft/versions/1.18.2/1.18.2.jar`
)

// type blockDescription struct {
// 	state string
// 	id    string
// }

func must(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

// func genBlockDescription(id string) []blockDescription {
// 	if block.FromID[id] == nil {
// 		return []blockDescription{}
// 	}
// 	params := map[string][]string{}
// 	bp := reflect.New(reflect.ValueOf(block.FromID[id]).Type())
// 	b := bp.Elem()
// 	for m := 0; m < b.NumField(); m++ {
// 		fieldName := b.Type().Field(m).Name
// 	}
// }

// func getBlockFromDescription(name, description string) block.Block {
// 	if block.FromID[name] == nil {
// 		return nil
// 	}
// 	if description == "" {
// 		return block.FromID[name]
// 	}
// 	params := map[string]string{}
// 	for _, d := range strings.Split(description, ",") {
// 		kv := strings.Split(d, "=")
// 		if len(kv) != 2 {
// 			panic("kv pair is not a pair")
// 		}
// 		params[kv[0]] = kv[1]
// 	}
// 	bp := reflect.New(reflect.ValueOf(block.FromID[name]).Type())
// 	b := bp.Elem()
// 	for m := 0; m < b.NumField(); m++ {
// 		fieldName := b.Type().Field(m).Name
// 		methodUnmarshal := -1
// 		for methodI := 0; methodI < b.Field(m).Addr().NumMethod(); methodI++ {
// 			if b.Field(m).Addr().Type().Method(methodI).Name == "UnmarshalText" {
// 				methodUnmarshal = methodI
// 				break
// 			}
// 		}
// 		if methodUnmarshal == -1 {
// 			log.Printf("Failed to find method UnmarshalText in type [%s]", b.Field(m).Type().Name())
// 			continue
// 		}
// 		for k, v := range params {
// 			if k == strings.ToLower(fieldName) {
// 				b.Field(m).Addr().Method(methodUnmarshal).Call([]reflect.Value{reflect.ValueOf([]byte(v))})
// 				break
// 			}
// 		}
// 	}
// 	return b.Interface().(block.Block)
// }

func blockMatches(b *block.Block, id string, d string) bool {
	if d == "" {
		return true
	}
	if b == nil {
		panic(fmt.Errorf("block-description matcher called with nil pointer to a block"))
	}
	if (*b).ID() != id {
		return false
	}
	params := map[string]string{}
	for _, e := range strings.Split(d, ",") {
		kv := strings.Split(e, "=")
		if len(kv) != 2 {
			panic("block-description matcher called with invalid description")
		}
		params[kv[0]] = kv[1]
	}
	r := reflect.ValueOf(*b)
	for m := 0; m < r.Type().NumField(); m++ {
		fieldName := r.Type().Field(m).Name
		methodString := -1
		for methodI := 0; methodI < r.Field(m).NumMethod(); methodI++ {
			if r.Field(m).Type().Method(methodI).Name == "String" {
				methodString = methodI
				break
			}
		}
		if methodString == -1 {
			if r.Field(m).Type().Name() == "Boolean" {
				methodString = -2
			} else if r.Field(m).Type().Name() == "Integer" {
				methodString = -3
			} else {
				log.Printf("Failed to find method String in type [%s]", r.Field(m).Type().Name())
				continue
			}
		}
		for k, v := range params {
			if k == strings.ToLower(fieldName) {
				if methodString == -2 {
					if fmt.Sprint(r.Field(m).Bool()) != v {
						return false
					}
				} else if methodString == -3 {
					if fmt.Sprint(r.Field(m).Int()) != v {
						return false
					}

				} else {
					e := r.Field(m).Method(methodString).Call([]reflect.Value{})
					if len(e) != 1 {
						panic(fmt.Errorf("block-description matcher got wrong return from String method"))
					}
					if e[0].String() != v {
						return false
					}
					break
				}
			}
		}
	}
	return true
}

func getModel(vv interface{}) (map[string]interface{}, error) {
	statedesc, ok := vv.(map[string]interface{})
	if !ok {
		statedesca, ok := vv.([]interface{})
		if !ok {
			return map[string]interface{}{}, fmt.Errorf("bad state description: %##v", vv)
		}
		if len(statedesc) < 0 {
			return map[string]interface{}{}, fmt.Errorf("bad state description: %##v", vv)
		}
		statedesc, ok = statedesca[0].(map[string]interface{})
		if !ok {
			return map[string]interface{}{}, fmt.Errorf("bad state description: %##v", vv)
		}
	}
	return statedesc, nil
	// model, ok := statedesc["model"]
	// if !ok {
	// 	return map[string]interface{}{}, fmt.Errorf("bad state description: %##v", vv)
	// }
	// modelpath, ok := model.(string)
	// if !ok {
	// 	return map[string]interface{}{}, fmt.Errorf("bad state description: %##v", vv)
	// }
}

func main() {
	log.SetFlags(log.Lshortfile)
	spew.Config.Indent = "   "

	filepaths := map[string]*zip.File{}
	// colors := map[string]color.RGBA64{}

	blockstateRegex := regexp.MustCompile("assets/minecraft/blockstates/([A-Za-z_]+).json")

	log.Printf("Opening jar [%s]", JARpath)
	r, err := zip.OpenReader(JARpath)
	must(err)
	defer r.Close()
	log.Print("Enumerating files in jar...")
	for i := 0; i < len(r.File); i++ {
		filepaths[r.File[i].Name] = r.File[i]
	}
	log.Printf("Mapped %d filenames", len(r.File))

	keys := make([]string, 0, len(filepaths))
	for k := range filepaths {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return strings.Compare(keys[i], keys[j]) > 0
	})

	// for i, _ := range block.FromID {
	// 	prefix := "assets/minecraft/textures/block/"
	// 	blockname := strings.ReplaceAll(i, "minecraft:", "")
	// 	var f *zip.File
	// 	for j, _ := range filepaths {
	// 		textureRegex := regexp.MustCompile("assets/minecraft/textures/block/([A-Za-z_]+).png")

	// 	}
	// 	// f, ok := filepaths[prefix+blockname+"_top.png"]
	// 	// if !ok {
	// 	// 	f, ok = filepaths[prefix+blockname+".png"]
	// 	// 	if !ok {
	// 	// 		log.Print("Failed to find texture for [" + blockname + "]")
	// 	// 		colors[i] = color.RGBA64{0, 0, 0, 0}
	// 	// 		continue
	// 	// 	}
	// 	// }
	// 	d, err := f.Open()
	// 	must(err)
	// 	defer d.Close()
	// 	colors[i] = *findColor(d)
	// }

	// log.Printf("Loaded %d/%d", len(colors), len(block.FromID))

	log.Print("Mapping blockstates to models...")
	statemodel := map[block.Block]map[string]interface{}{}
	failed := []string{}
	failedcount := 0
	for fnid := range keys {
		fname := keys[fnid]
		f := filepaths[fname]
		fmatch := blockstateRegex.FindStringSubmatch(fname)
		if fmatch == nil {
			continue
		}
		if fmatch[1] == "item_frame" || fmatch[1] == "glow_item_frame" {
			continue
		}
		fr, err := f.Open()
		must(err)
		c, err := ioutil.ReadAll(fr)
		must(err)
		fr.Close()
		v := map[string]interface{}{}
		must(json.Unmarshal(c, &v))
		_, ok := v["variants"]
		if !ok {
			spew.Dump(v)
			// log.Print(spew.Sprint(v))
			failed = append(failed, fmatch[1])
			for stateid := range block.StateList {
				if block.StateList[stateid].ID() == "minecraft:"+fmatch[1] {
					failedcount++
				}
			}
			continue
		}
		for k, vv := range v["variants"].(map[string]interface{}) {
			// log.Printf("%s: %##v", fmatch[1], k)
			matchingBlocks := []block.Block{}
			for stateid := range block.StateList {
				if block.StateList[stateid].ID() != "minecraft:"+fmatch[1] {
					continue
				}
				if blockMatches(&block.StateList[stateid], "minecraft:"+fmatch[1], k) {
					matchingBlocks = append(matchingBlocks, block.StateList[stateid])
				}
			}
			if len(matchingBlocks) != 0 {
				modelpath, err := getModel(vv)
				must(err)
				for mbi := range matchingBlocks {
					statemodel[matchingBlocks[mbi]] = modelpath
				}
			} else {
				log.Printf("Failed to create blockstate [%s] for block [%s]", k, "minecraft:"+fmatch[1])
				return
			}
			// log.Printf("%##v: %##v", len(matchingBlocks), vv)
		}
	}
	spew.Dump(failed)
	log.Printf("States parsed/actually %v/%v (%v)", len(statemodel), len(block.StateList), failedcount)

	// for i, j := range block.StateList {

	// }

	// var wg sync.WaitGroup
	// tasks := make(chan *block.Block)
	// results := map[string]color.RGBA64{}

	// for i := 0; i < runtime.NumCPU(); i++ {
	// 	go func() {
	// 		for b := range tasks {
	// 			results[b.ID] = findColor(trie, b)
	// 			wg.Done()
	// 		}
	// 	}()
	// }

	// wg.Add(len(block.ByID))
	// for _, b := range block.ByID {
	// 	tasks <- b
	// }
	// close(tasks)
	// wg.Wait()
	// for i, v := range results {
	// 	if v == nil {
	// 		results[i] = &color.RGBA64{R: 0, G: 0, B: 0, A: 0}
	// 	}
	// }
	// results[8] = &color.RGBA64{
	// 	R: 97 * math.MaxUint16 / 256,
	// 	G: 157 * math.MaxUint16 / 256,
	// 	B: 54 * math.MaxUint16 / 256,
	// 	A: 255 * math.MaxUint16 / 256,
	// }
	// fmt.Println(results)

	// size := int(math.Ceil(math.Sqrt(float64(len(results)))))
	// fmt.Println("Size: ", size)
	// img := image.NewRGBA(image.Rect(0, 0, size, size))
	// for i, v := range results {
	// 	if v != nil {
	// 		img.Set(i%size, i/size, v)
	// 	}
	// }
	// f, err := os.Create("res.png")
	// if err != nil {
	// 	panic(err)
	// }
	// defer f.Close()
	// if err := png.Encode(f, img); err != nil {
	// 	panic(err)
	// }

	// output(results)
}

// func find(root *trieNode, name string) []int {
// 	if len(root.values) > 0 {
// 		return root.values
// 	}
// 	if len(name) == 0 {
// 		return nil
// 	}
// 	return find(root.next[[]rune(name)[0]], name[1:])
// }

func findColor(f io.ReadCloser) *color.RGBA64 {
	img, err := png.Decode(f)
	if err != nil {
		panic(fmt.Errorf("decode error: %w", err))
	}
	bounds := img.Bounds()
	var rr, gg, bb, aa, count float64
	for i := bounds.Min.X; i < bounds.Max.X; i++ {
		for j := bounds.Min.Y; j < bounds.Max.Y; j++ {
			col := img.At(i, j)
			rrr, ggg, bbb, aaa := col.RGBA()
			rr += float64(rrr) * float64(aaa)
			gg += float64(ggg) * float64(aaa)
			bb += float64(bbb) * float64(aaa)
			aa += float64(aaa)
			count++
		}
	}
	return &color.RGBA64{
		R: uint16(rr / aa),
		G: uint16(gg / aa),
		B: uint16(bb / aa),
		A: uint16(aa / count),
	}
}

func output(colors map[string]color.RGBA64) {
	f, err := os.Create("colors.gob")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := gob.NewEncoder(f).Encode(colors); err != nil {
		panic(err)
	}
}
