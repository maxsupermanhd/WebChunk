package render

import (
	"image"

	"github.com/maxsupermanhd/go-vmc/v764/save"
)

type ChunkData interface {
	GetDimensionName() string
	GetDimension() *save.DimensionType
	Get() *save.Chunk
	GetNorth() *save.Chunk
	GetNorthEast() *save.Chunk
	GetEast() *save.Chunk
	GetEastSouth() *save.Chunk
	GetSouth() *save.Chunk
	GetSouthWest() *save.Chunk
	GetWest() *save.Chunk
	GetWestNorth() *save.Chunk
}

type DataNeeds struct {
	Dimension          bool
	NeighborsBordering bool
	NeighborsCorners   bool
}

type ChunkRenderer struct {
	Name   string
	Render func(ChunkData) *image.RGBA
	DataNeeds
}
