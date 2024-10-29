package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TODO: make this a configurable setting
var retryLimit int = 5

type PoolWorker struct {
	ctx         context.Context
	queue       *Queue
	config      *Config
	waitGroup   sync.WaitGroup
	workChannel chan Video
	workers     []*Worker
}

// TODO: add process output in this
type ProcessVideoOutput struct {
	skip                   bool
	outputFileAlreadyExist bool
	videoNotFound          bool
	err                    error
}

func NewPoolWorker(ctx context.Context, queue *Queue,
	config *Config, hub *Hub) *PoolWorker {
	poolWorker := PoolWorker{
		ctx:         ctx,
		queue:       queue,
		config:      config,
		waitGroup:   sync.WaitGroup{},
		workChannel: make(chan Video),
		workers:     nil,
	}

	workers := make([]*Worker, config.Workers)
	for i := 0; i < config.Workers; i++ {
		// Setup Worker Logger
		logger, err := CreateLogger(fmt.Sprintf("worker%d", i))
		if err != nil {
			log.Panicf("Couldn't create logger for worker: %d", i)
		}

		workers[i] = NewWorker(i, logger, &poolWorker, hub)
	}

	poolWorker.workers = workers
	return &poolWorker
}

func (p *PoolWorker) RunDispatcherBlocking() {
	for i := 0; i < p.config.Workers; i++ {
		go p.workers[i].start()
	}

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			video, ok := p.queue.Peek()
			if ok {
				select {
				case p.workChannel <- video:
					p.queue.Dequeue()
				default:
					time.Sleep(100 * time.Millisecond)
				}
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

func (p *PoolWorker) GetWorkerInfos() []WorkerInfo {
	var info []WorkerInfo
	for _, worker := range p.workers {
		info = append(info, worker.GetInfo())
	}

	return info
}
