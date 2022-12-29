package main

import (
	"sync"
)

// based on some stackoverflow answer

type ProgressReport struct {
	TaskID        int64
	TaskTotal     int64
	TaskCompleted int64
	TaskStatus    string
	ETA           int64
	Speed         int64
	Done          bool
}

var nextTaskID int64
var nextTaskLock sync.Mutex

func GenNewTaskID() int64 {
	nextTaskLock.Lock()
	id := nextTaskID
	nextTaskID++
	nextTaskLock.Unlock()
	return id
}

type ProgressBroadcaster struct {
	stopCh    chan struct{}
	publishCh chan ProgressReport
	subCh     chan chan ProgressReport
	unsubCh   chan chan ProgressReport
}

func NewBroadcaster() *ProgressBroadcaster {
	return &ProgressBroadcaster{
		stopCh:    make(chan struct{}),
		publishCh: make(chan ProgressReport, 1),
		subCh:     make(chan chan ProgressReport, 1),
		unsubCh:   make(chan chan ProgressReport, 1),
	}
}

func (b *ProgressBroadcaster) Start() {
	subs := map[chan ProgressReport]struct{}{}
	tasks := map[int64]ProgressReport{}
	for {
		select {
		case <-b.stopCh:
			return
		case msgCh := <-b.subCh:
			subs[msgCh] = struct{}{}
			for i := range tasks {
				msgCh <- tasks[i]
			}
		case msgCh := <-b.unsubCh:
			delete(subs, msgCh)
		case msg := <-b.publishCh:
			if msg.Done {
				delete(tasks, msg.TaskID)
			} else {
				tasks[msg.TaskID] = msg
			}
			for msgCh := range subs {
				select {
				case msgCh <- msg:
				default:
				}
			}
		}
	}
}

func (b *ProgressBroadcaster) Stop() {
	close(b.stopCh)
}

func (b *ProgressBroadcaster) Subscribe() chan ProgressReport {
	msgCh := make(chan ProgressReport, 16)
	b.subCh <- msgCh
	return msgCh
}

func (b *ProgressBroadcaster) Unsubscribe(msgCh chan ProgressReport) {
	b.unsubCh <- msgCh
}

func (b *ProgressBroadcaster) Publish(msg ProgressReport) {
	b.publishCh <- msg
}

func NewTask[T *any, R *any](threads int, estimated int64, buffer int,
	supply chan T,
	consumer func(taskID int64, threadID int, tasks <-chan T, results chan<- R),
	results chan<- R) {
	tid := GenNewTaskID()
	tasksProgressBroadcaster.Publish(ProgressReport{
		TaskID:        tid,
		TaskTotal:     estimated,
		TaskCompleted: 0,
		TaskStatus:    "Starting",
		ETA:           -1,
		Speed:         -1,
		Done:          false,
	})
	wg := new(sync.WaitGroup)
	wg.Add(threads)
	tasks := make(chan T, buffer)
	pproxy := make(chan R, buffer)
	for i := 0; i < threads; i++ {
		ii := i
		go func() {
			defer wg.Done()
			consumer(tid, ii, tasks, results)
		}()
	}
	for {
		t, more := <-tasks
		if !more {
			break
		}
		tasks <- t
	}
	close(tasks)
	wg.Wait()
	close(pproxy)
}
