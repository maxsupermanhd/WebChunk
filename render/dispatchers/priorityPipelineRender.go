package dispatchers

import (
	"log/slog"
	"sync"

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
	wg        sync.WaitGroup
	l         slog.Logger
	closeFn   func()
}

func NewPriorityRenderer(cfg *lac.ConfSubtree) *PriorityPipelineRender {
	closeChan := make(chan struct{})
	r := &PriorityPipelineRender{
		qnormal:   make(chan renderTask, cfg.GetDInt(64, "queueNormalLen")),
		qpriority: make(chan renderTask, cfg.GetDInt(128, "queuePriorityLen")),
		qfetched:  make(chan renderTask, cfg.GetDInt(32, "queueFetchedLen")),
		rends:     renderers.ConstructRenderers(cfg),
		wg:        sync.WaitGroup{},
		closeFn: sync.OnceFunc(func() {
			close(closeChan)
		}),
	}
	rendererThreadCount := cfg.GetDInt(4, "rendererThreadCount")
	r.wg.Add(rendererThreadCount)
	for i := 0; i < rendererThreadCount; i++ {
		go func() {
			r.workerRender(closeChan)
			r.wg.Done()
		}()
	}
	fetcherThreadCount := cfg.GetDInt(4, "fetcherThreadCount")
	r.wg.Add(fetcherThreadCount)
	for i := 0; i < fetcherThreadCount; i++ {
		go func() {
			r.workerFetch(closeChan)
			r.wg.Done()
		}()
	}
	return r
}

func (r *PriorityPipelineRender) workerRender(close <-chan struct{}) {
	for {
		select {
		case <-close:
			return
		case w := <-r.qfetched:
			r.render(w)
		}
	}
}

func (r *PriorityPipelineRender) workerFetch(close <-chan struct{}) {
	for {
		select {
		case <-close:
			return
		case w := <-r.qpriority:
			r.fetch(w)
			select {
			case <-close:
				return
			case r.qfetched <- w:
			}
		default:
		}
		select {
		case <-close:
			return
		case w := <-r.qpriority:
			r.fetch(w)
			select {
			case <-close:
				return
			case r.qfetched <- w:
			}
		case w := <-r.qnormal:
			r.fetch(w)
			select {
			case <-close:
				return
			case r.qfetched <- w:
			}
		}
	}
}

func (r *PriorityPipelineRender) fetch(work renderTask) {
	// TODO: fetch data
}

func (r *PriorityPipelineRender) render(work renderTask) {
	if work.data == nil {
		r.l.Error("render without data", "loc", work.loc)
	}
	// TODO: perform rendering
}

// stops and waits
func (r *PriorityPipelineRender) Close() {
	r.closeFn()
	r.wg.Wait()
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

func (r *PriorityPipelineRender) AddToRenderQueueWithData(loc primitives.ImageLocation, data render.ChunkData) {
	r.qfetched <- renderTask{
		loc:  loc,
		data: data,
	}
}
