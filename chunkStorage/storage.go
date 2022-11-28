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

package chunkStorage

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Tnze/go-mc/save"
)

var (
	ErrNotImplemented = errors.New("not implemented")
	ErrAlreadyExists  = errors.New("already exists")
	ErrReadOnly       = errors.New("storage is read-only")
	ErrNoWorld        = errors.New("world not found")
	ErrNoDim          = errors.New("dimension not found")
)

type SWorld struct {
	Name       string // unique
	Alias      string
	IP         string
	CreatedAt  time.Time
	ModifiedAt time.Time
	Data       save.LevelData
}

type SDim struct {
	Name       string // unique per world
	World      string // name of the world
	CreatedAt  time.Time
	ModifiedAt time.Time
	Data       save.DimensionType
}

type ChunkData struct {
	X, Z int
	Data interface{}
}

type StorageAbilities struct {
	CanCreateWorldsDimensions   bool
	CanAddChunks                bool
	CanPreserveOldChunks        bool
	CanStoreUnlimitedDimensions bool
}

// Everything returns empty slice/nil if specified
// object is not found, error only in case of abnormal things.
type ChunkStorage interface {
	GetAbilities() StorageAbilities
	GetStatus() (string, error)
	GetChunksCount() (uint64, error)
	GetChunksSize() (uint64, error)

	ListWorlds() ([]SWorld, error)
	ListWorldNames() ([]string, error)
	GetWorld(wname string) (*SWorld, error)
	AddWorld(world SWorld) error
	SetWorldAlias(wname, newalias string) error
	SetWorldIP(wname, newip string) error
	SetWorldData(wname string, data save.LevelData) error

	ListWorldDimensions(wname string) ([]SDim, error)
	ListDimensions() ([]SDim, error)
	AddDimension(wname string, dim SDim) error
	GetDimension(wname, dname string) (*SDim, error)
	SetDimensionData(wname, dname string, data save.DimensionType) error
	GetDimensionChunksCount(wname, dname string) (uint64, error)
	GetDimensionChunksSize(wname, dname string) (uint64, error)

	AddChunk(wname, dname string, cx, cz int, col save.Chunk) error
	AddChunkRaw(wname, dname string, cx, cz int, dat []byte) error
	GetChunk(wname, dname string, cx, cz int) (*save.Chunk, error)
	GetChunkRaw(wname, dname string, cx, cz int) ([]byte, error)
	// Warning, chunk data array may be real big!
	GetChunksRegion(wname, dname string, cx0, cz0, cx1, cz1 int) ([]ChunkData, error)
	// Warning, chunk data array may be real big!
	GetChunksRegionRaw(wname, dname string, cx0, cz0, cx1, cz1 int) ([]ChunkData, error)
	// Warning, chunk data array may be real big!
	GetChunksCountRegion(wname, dname string, cx0, cz0, cx1, cz1 int) ([]ChunkData, error)

	Close() error
}

type Storage struct {
	Name    string       `json:"name"`
	Type    string       `json:"type"`
	Address string       `json:"addr"`
	Driver  ChunkStorage `json:"-"`
}

func CloseStorages(s []Storage) {
	for _, c := range s {
		if c.Driver != nil {
			err := c.Driver.Close()
			if err != nil {
				log.Printf("Error closing storage [%v] of type %v: %v", c.Name, c.Type, err)
			}
			c.Driver = nil
		}
	}
}

func ListWorlds(storages []Storage) []SWorld {
	worlds := []SWorld{}
	for _, s := range storages {
		if s.Driver != nil {
			w, err := s.Driver.ListWorlds()
			if err != nil {
				log.Printf("Failed to list worlds on storage %s: %s", s.Name, err.Error())
			}
			worlds = append(worlds, w...)
		}
	}
	return worlds
}

func ListDimensions(storages []Storage, wname string) ([]SDim, error) {
	dims := []SDim{}
	if wname == "" {
		for _, s := range storages {
			if s.Driver != nil {
				d, err := s.Driver.ListDimensions()
				if err != nil {
					log.Printf("Failed to list dims on storage %s: %s", s.Name, err.Error())
				}
				dims = append(dims, d...)
			}
		}
	} else {
		_, s, err := GetWorldStorage(storages, wname)
		if err != nil {
			return dims, err
		}
		if s == nil {
			return dims, fmt.Errorf("world storage not found")
		}
		dims, err = s.ListWorldDimensions(wname)
		if err != nil {
			return dims, err
		}
	}
	return dims, nil
}

func GetWorldStorage(storages []Storage, wname string) (*SWorld, ChunkStorage, error) {
	for _, s := range storages {
		if s.Driver != nil {
			w, err := s.Driver.GetWorld(wname)
			if err != nil {
				return nil, nil, err
			}
			if w != nil {
				return w, s.Driver, nil
			}
		}
	}
	return nil, nil, nil
}

func CreateDefaultLevelData(LevelName string) save.LevelData {
	return save.LevelData{
		AllowCommands:        1,
		BorderCenterX:        0,
		BorderCenterZ:        0,
		BorderDamagePerBlock: 0.2,
		BorderSafeZone:       5,
		BorderSize:           59999968,
		BorderSizeLerpTarget: 59999968,
		BorderSizeLerpTime:   0,
		BorderWarningBlocks:  5,
		BorderWarningTime:    15,
		ClearWeatherTime:     0,
		CustomBossEvents:     map[string]save.CustomBossEvent{},
		DataPacks: struct {
			Enabled  []string
			Disabled []string
		}{
			Enabled:  []string{"vanilla"},
			Disabled: []string{},
		},
		DataVersion:      3120,
		DayTime:          0,
		Difficulty:       2,
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
		GameRules: map[string]string{
			"forgiveDeadPlayers":         "true",
			"doInsomnia":                 "true",
			"fallDamage":                 "true",
			"doDaylightCycle":            "true",
			"spawnRadius":                "10",
			"doWeatherCycle":             "true",
			"doPatrolSpawning":           "true",
			"maxCommandChainLength":      "65536",
			"universalAnger":             "false",
			"fireDamage":                 "true",
			"doImmediateRespawn":         "false",
			"playersSleepingPercentage":  "100",
			"maxEntityCramming":          "24",
			"doMobSpawning":              "true",
			"showDeathMessages":          "true",
			"announceAdvancements":       "true",
			"disableRaids":               "false",
			"naturalRegeneration":        "true",
			"reducedDebugInfo":           "false",
			"drowningDamage":             "true",
			"sendCommandFeedback":        "true",
			"doLimitedCrafting":          "false",
			"commandBlockOutput":         "true",
			"doTraderSpawning":           "true",
			"doFireTick":                 "true",
			"mobGriefing":                "true",
			"spectatorsGenerateChunks":   "true",
			"doEntityDrops":              "true",
			"doTileDrops":                "true",
			"keepInventory":              "false",
			"randomTickSpeed":            "3",
			"doWardenSpawning":           "true",
			"freezeDamage":               "true",
			"doMobLoot":                  "true",
			"disableElytraMovementCheck": "false",
			"logAdminCommands":           "true",
		},
		WorldGenSettings: save.WorldGenSettings{
			BonusChest:       false,
			GenerateFeatures: true,
			Seed:             0,
			Dimensions: map[string]save.DimensionGenerator{
				"minecraft:overworld": {
					Type: "minecraft:overworld",
					Generator: map[string]interface{}{
						"biome_source": map[string]interface{}{
							"preset": "minecraft:overworld",
							"type":   "minecraft:multi_noise",
						},
						"settings": "minecraft:overworld",
						"type":     "minecraft:noise",
					},
				},
				"minecraft:the_end": {
					Type: "minecraft:the_end",
					Generator: map[string]interface{}{
						"biome_source": map[string]interface{}{
							"type": "minecraft:the_end",
						},
						"settings": "minecraft:end",
						"type":     "minecraft:noise",
					},
				},
				"minecraft:the_nether": {
					Type: "minecraft:the_nether",
					Generator: map[string]interface{}{
						"biome_source": map[string]interface{}{
							"preset": "minecraft:nether",
							"type":   "minecraft:multi_noise",
						},
						"settings": "minecraft:nether",
						"type":     "minecraft:noise",
					},
				},
			},
		},
		GameType:    0,
		HardCore:    false,
		Initialized: false,
		LastPlayed:  time.Now().Unix(),
		LevelName:   LevelName,
		MapFeatures: true,
		Player:      map[string]interface{}{},
		Raining:     false,
		RainTime:    15000,
		RandomSeed:  0,
		SizeOnDisk:  0,
		SpawnX:      0,
		SpawnY:      95,
		SpawnZ:      0,
		Thundering:  false,
		ThunderTime: 15000,
		Time:        0,
		Version: struct {
			ID       int32 "nbt:\"Id\""
			Name     string
			Series   string
			Snapshot byte
		}{
			ID:       3120,
			Name:     "1.19.2",
			Series:   "main",
			Snapshot: 0,
		},
		WanderingTraderId:          []int32{},
		WanderingTraderSpawnChance: 25,
		WanderingTraderSpawnDelay:  19200,
		WasModded:                  false,
	}
}

func GuessDimTypeFromName(dname string) save.DimensionType {
	if strings.HasPrefix(dname, "minecraft:") {
		dt, ok := save.DefaultDimensionsTypes[dname]
		if !ok {
			return save.DefaultDimensionsTypes["minecraft:overworld"]
		}
		return dt
	} else {
		dt, ok := save.DefaultDimensionsTypes["minecraft:"+dname]
		if !ok {
			return save.DefaultDimensionsTypes["minecraft:overworld"]
		}
		return dt
	}
}
