package dispatchers

import (
	"github.com/maxsupermanhd/WebChunk/primitives"
	"github.com/maxsupermanhd/WebChunk/render"
	"github.com/maxsupermanhd/WebChunk/render/renderers"
	"github.com/maxsupermanhd/lac"
)

type renderTask struct {
	loc  primitives.ImageLocation
	data render.ChunkData
}

type PriorityPipelineRender struct {
	qnormal   chan renderTask
	qpriority chan renderTask
	qfetched  chan renderTask
	rends     []render.ChunkRenderer
}

func NewPriorityRenderer(cfg *lac.ConfSubtree) *PriorityPipelineRender {
	ret := &PriorityPipelineRender{
		rends: renderers.ConstructRenderers(cfg),
	}
	return ret
}

func (r *PriorityPipelineRender) AddToRenderQueue(loc primitives.ImageLocation) {
	r.qnormal <- renderTask{
		loc:  loc,
		data: nil,
	}
}

func (r *PriorityPipelineRender) AddToPriorityRenderQueue(loc primitives.ImageLocation) {
	r.qpriority <- renderTask{
		loc:  loc,
		data: nil,
	}
}
