package main

import (
	"archive/zip"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/maxsupermanhd/go-vmc/v764/level/block"
)

var (
	JARpath = flag.String("jar", "~/.minecraft/versions/1.20.2.jar", "path to jar")
)

func must(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

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
				vv := strings.Split(v, "|")
				if len(vv) > 1 {
					match := false
					for _, vi := range vv {
						if methodString == -2 {
							if fmt.Sprint(r.Field(m).Bool()) == vi {
								match = true
							}
						} else if methodString == -3 {
							if fmt.Sprint(r.Field(m).Int()) == vi {
								match = true
							}
						} else {
							e := r.Field(m).Method(methodString).Call([]reflect.Value{})
							if len(e) != 1 {
								panic(fmt.Errorf("block-description matcher got wrong return from String method"))
							}
							if e[0].String() == vi {
								match = true
							}
							break
						}
					}
					if !match {
						return false
					}
				} else {
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
	flag.Parse()
	log.SetFlags(log.Lshortfile)
	spew.Config.Indent = "   "

	filepaths := map[string]*zip.File{}

	blockstateRegex := regexp.MustCompile("assets/minecraft/blockstates/([A-Za-z_]+).json")

	log.Printf("Opening jar [%s]", *JARpath)
	r, err := zip.OpenReader(*JARpath)
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

	log.Print("Mapping blockstates to models...")
	statemodel := map[block.Block]map[string]interface{}{}
	for fnid := range keys {
		fname := keys[fnid]
		f := filepaths[fname]
		fmatch := blockstateRegex.FindStringSubmatch(fname)
		if fmatch == nil {
			continue
		}
		if fmatch[1] == "item_frame" ||
			fmatch[1] == "glow_item_frame" ||
			fmatch[1] == "piglin_wall_head" ||
			fmatch[1] == "piglin_head" ||
			strings.HasSuffix(fmatch[1], "_hanging_sign") ||
			fmatch[1] == "stripped_bamboo_block" {
			continue
		}
		fr, err := f.Open()
		must(err)
		c, err := ioutil.ReadAll(fr)
		must(err)
		fr.Close()
		v := map[string]interface{}{}
		must(json.Unmarshal(c, &v))
		if _, ok := v["multipart"]; ok {
			catchallmodel := map[string]interface{}{}
			filteredmodel := map[block.Block]map[string]interface{}{}
			for _, vv := range v["multipart"].([]interface{}) {
				partcase, ok := vv.(map[string]interface{})
				if !ok {
					panic(spew.Sdump(vv))
				}
				found, ok := partcase["apply"].(map[string]interface{})
				if !ok {
					found = (partcase["apply"].([]interface{}))[0].(map[string]interface{})
				}
				whencase, ok := partcase["when"]
				if !ok {
					catchallmodel = found
				} else {
					whencase := whencase.(map[string]interface{})
					ored, ok := whencase["OR"]
					matches := []string{}
					if ok {
						ored := ored.([]interface{})
						for _, orv := range ored {
							subm := []string{}
							orv := orv.(map[string]interface{})
							for whi, whk := range orv {
								subm = append(subm, whi+"="+whk.(string))
							}
							matches = append(matches, strings.Join(subm, ","))
						}
						foundcase := false
						for stateid := range block.StateList {
							state := block.StateList[stateid]
							if state.ID() == "minecraft:"+fmatch[1] {
								doesmatch := false
								for _, mv := range matches {
									if blockMatches(&block.StateList[stateid], "minecraft:"+fmatch[1], mv) {
										doesmatch = true
										foundcase = true
									}
								}
								if doesmatch {
									filteredmodel[state] = found
								}
							}
						}
						if !foundcase {
							log.Printf("No matching blockstate for description condition %v", spew.Sdump(whencase))
						}
					} else {
						for whi, whv := range whencase {
							whvs, ok := whv.(string)
							if !ok {
								continue
							}
							matches = append(matches, whi+"="+whvs)
						}
						for stateid := range block.StateList {
							state := block.StateList[stateid]
							if state.ID() == "minecraft:"+fmatch[1] {
								desc := strings.Join(matches, ",")
								if blockMatches(&block.StateList[stateid], "minecraft:"+fmatch[1], desc) {
									filteredmodel[state] = found
								}
							}
						}
					}
				}
			}
			for stateid := range block.StateList {
				state := block.StateList[stateid]
				if state.ID() == "minecraft:"+fmatch[1] {
					filtered, ok := filteredmodel[state]
					if ok {
						statemodel[state] = filtered
					} else {
						statemodel[state] = catchallmodel
					}
				}
			}
		} else if _, ok := v["variants"]; ok {
			for k, vv := range v["variants"].(map[string]interface{}) {
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
					// return
				}
			}
		} else {
			log.Printf("Weird json you have here [%v], skipping", fname)
		}
	}
	log.Printf("States parsed/actually %v/%v", len(statemodel), len(block.StateList))
	log.Print("Mapping blockstates to textures...")

	statetextures := map[block.Block][]string{}
	for s, m := range statemodel {
		if s.ID() == "minecraft:air" || s.ID() == "minecraft:cave_air" || s.ID() == "minecraft:void_air" {
			continue
		}
		modelname, ok := m["model"].(string)
		if !ok {
			log.Printf("State %v: %v does not have model %v", s.ID(), spew.Sdump(s), spew.Sprint(m))
			continue
		}
		fname := "assets/minecraft/models/" + modelname[10:] + ".json"
		f, ok := filepaths[fname]
		if !ok {
			log.Printf("Model %v path not found!", spew.Sprint(m))
			continue
		}
		fr, err := f.Open()
		must(err)
		c, err := ioutil.ReadAll(fr)
		must(err)
		fr.Close()
		v := map[string]interface{}{}
		must(json.Unmarshal(c, &v))
		if t, ok := v["textures"]; ok {
			textures := []string{}
			for _, tex := range t.(map[string]interface{}) {
				textures = append(textures, tex.(string))
			}
			statetextures[s] = textures
		} else {
			log.Printf("Texture not found for block %v", spew.Sdump(s))
		}
	}
	log.Printf("Loaded %v/%v models (should be %v total)", len(statetextures), len(statemodel), len(block.StateList))

	colors := map[int]color.RGBA64{}
	cachedcolors := map[string]color.RGBA64{}
	colornotfound := 0
	for i, j := range block.StateList {
		texturename, ok := statetextures[j]
		avgcolor := color.RGBA64{R: 0, G: 0, B: 0, A: 0}
		avgcolorc := uint16(0)
		if ok {
			for _, tex := range texturename {
				tex = strings.TrimPrefix(tex, "minecraft:")
				fp := "assets/minecraft/textures/" + tex + ".png"
				f, ok := filepaths[fp]
				if !ok {
					log.Printf("File not found: %v", fp)
					continue
				}
				cached, ok := cachedcolors[fp]
				readedcolor := color.RGBA64{R: 0, G: 0, B: 0, A: 0}
				if ok {
					readedcolor = cached
				} else {
					r, err := f.Open()
					must(err)
					readedcolor = *findColor(r)
					r.Close()
					cachedcolors[fp] = readedcolor
				}
				avgcolor.R += readedcolor.R
				avgcolor.G += readedcolor.G
				avgcolor.B += readedcolor.B
				avgcolor.A += readedcolor.A
				avgcolorc++
			}
		}
		if avgcolorc == 0 {
			colors[i] = avgcolor
			colornotfound++
		} else {
			colors[i] = color.RGBA64{R: avgcolor.R / avgcolorc, G: avgcolor.G / avgcolorc, B: avgcolor.B / avgcolorc, A: avgcolor.A / avgcolorc}
		}
	}

	log.Printf("Colors matched %v/%v", len(colors)-colornotfound, len(block.StateList))

	size := int(math.Ceil(math.Sqrt(float64(len(block.StateList)))))
	fmt.Println("Size: ", size)
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for i, v := range colors {
		img.Set(i%size, i/size, v)
	}
	f, err := os.Create("res.png")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}

	towrite := []color.RGBA64{}
	for i := 0; i < len(block.StateList); i++ {
		towrite = append(towrite, colors[i])
	}

	output(towrite)
}

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

func output(colors []color.RGBA64) {
	f, err := os.Create("colors.gob")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := gob.NewEncoder(f).Encode(colors); err != nil {
		panic(err)
	}
}
