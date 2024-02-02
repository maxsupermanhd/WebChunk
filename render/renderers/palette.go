package renderers

import (
	"github.com/maxsupermanhd/go-vmc/v764/level"
	"github.com/maxsupermanhd/go-vmc/v764/level/block"
	"github.com/maxsupermanhd/go-vmc/v764/save"
)

func prepareSectionBlockstates(s *save.Section) *level.PaletteContainer[block.StateID] {
	statePalette := s.BlockStates.Palette
	stateRawPalette := make([]block.StateID, len(statePalette))
	for i, v := range statePalette {
		b, ok := block.FromID[v.Name]
		if !ok {
			b, ok = block.FromID["minecraft:"+v.Name]
			if !ok {
				return nil
			}
		}
		if v.Properties.Data != nil {
			err := v.Properties.Unmarshal(&b)
			if err != nil {
				// log.Printf("Error unmarshling properties of block [%v] from [%v]: %v", v.Name, v.Properties.String(), err.Error())
				return nil
			}
		}
		s := block.ToStateID[b]
		stateRawPalette[i] = s
	}
	return level.NewStatesPaletteContainerWithData(16*16*16, s.BlockStates.Data, stateRawPalette)
}

func prepareSectionBlockIDs(s *save.Section) *level.PaletteContainer[block.StateID] {
	statePalette := s.BlockStates.Palette
	stateRawPalette := make([]block.StateID, len(statePalette))
	for i, v := range statePalette {
		b, ok := block.FromID[v.Name]
		if !ok {
			b, ok = block.FromID["minecraft:"+v.Name]
			if !ok {
				// log.Printf("Can not find block from id [%v]", v.Name)
				return nil
			}
		}
		stateRawPalette[i] = block.ToStateID[b]
	}
	return level.NewStatesPaletteContainerWithData(16*16*16, s.BlockStates.Data, stateRawPalette)
}

// func prepareSectionBiomesBitStorage(s *save.Section) {
// 	level.NewBitStorage(bits.Len(uint(len(s.Biomes.Palette))), len(s.Biomes.Palette), s.Biomes.Data)
// 	// s.Biomes.Data
// }
