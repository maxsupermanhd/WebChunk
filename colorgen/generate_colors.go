package main

import (
	"encoding/gob"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/Tnze/go-mc/data/block"
)

type trieNode struct {
	next   map[rune]*trieNode
	values []int
}

var de []fs.DirEntry
var texturesDir = `/home/max/Desktop/block/`

func main() {
	trie := &trieNode{}
	for _, b := range block.ByID {
		trie := trie
		for _, c := range b.Name {
			if trie.next == nil {
				trie.next = make(map[rune]*trieNode)
			}
			if trie.next[c] == nil {
				trie.next[c] = new(trieNode)
			}
			trie = trie.next[c]
		}
		trie.values = make([]int, 0, 4)
	}

	var err error
	if de, err = os.ReadDir(texturesDir); err != nil {
		panic(err)
	}
	for i, d := range de {
		trie := trie
		name := d.Name()
		for _, c := range name {
			if trie.next == nil {
				trie.next = make(map[rune]*trieNode)
			}
			if trie.next[c] == nil {
				trie.next[c] = new(trieNode)
			}
			trie = trie.next[c]

			if trie.values != nil {
				trie.values = append(trie.values, i)
				break
			}
		}
	}

	var wg sync.WaitGroup
	tasks := make(chan *block.Block)
	results := make([]*color.RGBA64, len(block.ByID))

	for i := 0; i < runtime.NumCPU(); i++ {
		go func() {
			for b := range tasks {
				results[b.ID] = findColor(trie, b)
				wg.Done()
			}
		}()
	}

	wg.Add(len(block.ByID))
	for _, b := range block.ByID {
		tasks <- b
	}
	close(tasks)
	wg.Wait()
	for i, v := range results {
		if v == nil {
			results[i] = &color.RGBA64{R: 0, G: 0, B: 0, A: 0}
		}
	}
	results[8] = &color.RGBA64{
		R: 97 * math.MaxUint16 / 256,
		G: 157 * math.MaxUint16 / 256,
		B: 54 * math.MaxUint16 / 256,
		A: 255 * math.MaxUint16 / 256,
	}
	fmt.Println(results)

	size := int(math.Ceil(math.Sqrt(float64(len(results)))))
	fmt.Println("Size: ", size)
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for i, v := range results {
		if v != nil {
			img.Set(i%size, i/size, v)
		}
	}
	f, err := os.Create("res.png")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}

	output(results)
}

func find(root *trieNode, name string) []int {
	if len(root.values) > 0 {
		return root.values
	}
	if len(name) == 0 {
		return nil
	}
	return find(root.next[[]rune(name)[0]], name[1:])
}

func findColor(t *trieNode, b *block.Block) *color.RGBA64 {
	node := find(t, b.Name)
	if len(node) <= 0 {
		return nil
	}
	pic := de[node[0]]
	for i := range node {
		f := de[node[i]]
		if strings.Contains(f.Name(), "top") && filepath.Ext(f.Name()) != ".mcdata" {
			pic = de[node[i]]
			break
		}
	}
	f, err := os.Open(filepath.Join(texturesDir, pic.Name()))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		panic(fmt.Errorf("decode %s error: %w", pic.Name(), err))
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

func output(colors []*color.RGBA64) {
	f, err := os.Create("colors.gob")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := gob.NewEncoder(f).Encode(colors); err != nil {
		panic(err)
	}
}
