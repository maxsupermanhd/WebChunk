/*
	WebChunk, web server for block game maps
	Copyright (C) 2022 Maxim Zhuchkov

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU Affero General Public License as published
	by the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU Affero General Public License for more details.

	You should have received a copy of the GNU Affero General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.

	Contact me via mail: q3.max.2011@yandex.ru or Discord: MaX#6717
*/

package filesystemChunkStorage

import (
	"bytes"
	"compress/gzip"
	"log"
	"os"
	"path"

	"github.com/maxsupermanhd/go-vmc/v762/nbt"
	"github.com/maxsupermanhd/go-vmc/v762/save"
)

// reads level data from file
func readSaveLevel(path string) (*save.LevelData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gf, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gf.Close()
	var r save.Level
	c := nbt.NewDecoder(gf)
	_, err = c.Decode(&r)
	if err != nil {
		return nil, err
	}
	return &r.Data, err
}

func writeSaveLevel(dir string, d save.LevelData) error {
	b, err := nbt.Marshal(save.Level{Data: d})
	if err != nil {
		return err
	}
	var cb bytes.Buffer
	w := gzip.NewWriter(&cb)
	_, err = w.Write(b)
	if err != nil {
		return err
	}
	w.Close()
	return os.WriteFile(path.Join(dir, "level.dat"), cb.Bytes(), 0666)
}

func createDefaultDims(wroot string) error {
	createDirs := []string{
		path.Join(wroot, "region"),
		path.Join(wroot, "DIM-1", "region"),
		path.Join(wroot, "DIM1", "region"),
	}
	for _, v := range createDirs {
		log.Printf("Creating directory [%s]", v)
		err := os.MkdirAll(v, 0764)
		if err != nil && err != os.ErrExist {
			return err
		}
	}
	return nil
}

//lint:ignore U1000 One day custom dims will be implemented
func createDefaultLevelDat(fname, levelname string) error {
	towrite := save.Level{
		Data: save.LevelData{
			AllowCommands:        1,
			BorderCenterX:        0,
			BorderCenterZ:        0,
			BorderDamagePerBlock: 0,
			BorderSafeZone:       0,
			BorderSize:           0,
			BorderSizeLerpTarget: 0,
			BorderSizeLerpTime:   0,
			BorderWarningBlocks:  0,
			BorderWarningTime:    0,
			ClearWeatherTime:     0,
			CustomBossEvents:     map[string]save.CustomBossEvent{},
			DataPacks: struct {
				Enabled  []string
				Disabled []string
			}{},
			DataVersion:      0,
			DayTime:          0,
			Difficulty:       0,
			DifficultyLocked: false,
			DimensionData: struct {
				TheEnd struct {
					DragonFight struct {
						Gateways         []int32
						DragonKilled     byte
						PreviouslyKilled byte
					}
				} "nbt:\"1\""
			}{},
			GameRules:        map[string]string{},
			WorldGenSettings: save.WorldGenSettings{},
			GameType:         0,
			HardCore:         false,
			Initialized:      false,
			LastPlayed:       0,
			LevelName:        levelname,
			MapFeatures:      false,
			Player:           map[string]interface{}{},
			Raining:          false,
			RainTime:         0,
			RandomSeed:       0,
			SizeOnDisk:       -1,
			SpawnX:           0,
			SpawnY:           64,
			SpawnZ:           0,
			Thundering:       false,
			ThunderTime:      0,
			Time:             0,
			Version: struct {
				ID       int32 "nbt:\"Id\""
				Name     string
				Series   string
				Snapshot byte
			}{},
			WanderingTraderId:          []int32{},
			WanderingTraderSpawnChance: 0,
			WanderingTraderSpawnDelay:  0,
			WasModded:                  false,
		},
	}
	data, err := nbt.Marshal(towrite)
	if err != nil {
		return err
	}
	return os.WriteFile(fname, data, 0666)
}

// checks that directory is a valid world directory
func checkValidWorld(path string) bool {
	dir, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, k := range dir {
		if k.Name() == "level.dat" && k.Type().IsRegular() {
			return true
		}
	}
	return false
}
