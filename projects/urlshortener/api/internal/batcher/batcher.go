package batcher

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var ErrBatcherDead = errors.New("batcher is not running")

type Config struct {
	BatchSize    int
	FlushMs      int
	MaxWorkers   int
	DropOnFull   bool
	DrainTimeout time.Duration
}

type Result[Res any] struct {
	Value Res
	Err   error
}

type Request[Req, Res any] struct {
	Input    Req
	ResultCh chan Result[Res]
}

type FlushFunc[Req, Res any] func(ctx context.Context, batch []Request[Req, Res])

type Batcher[Req, Res any] struct {
	cfg       Config
	flushFn   FlushFunc[Req, Res]
	requestCh chan Request[Req, Res]
	workers   int
	alive     atomic.Bool
	dropped   atomic.Uint64
	wg        sync.WaitGroup
}

func New[Req, Res any](cfg Config, flushFn FlushFunc[Req, Res]) *Batcher[Req, Res] {
	workers := max(cfg.MaxWorkers, 1)
	if cfg.DrainTimeout == 0 {
		cfg.DrainTimeout = 5 * time.Second
	}
	return &Batcher[Req, Res]{
		cfg:       cfg,
		flushFn:   flushFn,
		requestCh: make(chan Request[Req, Res], cfg.BatchSize*workers),
		workers:   workers,
	}
}

func (b *Batcher[Req, Res]) Start(ctx context.Context) {
	b.alive.Store(true)
	for range b.workers {
		b.wg.Add(1)
		go b.runFlushLoop(ctx)
	}
}

func (b *Batcher[Req, Res]) Close() {
	b.alive.Store(false)
	b.wg.Wait()
}

func (b *Batcher[Req, Res]) Submit(ctx context.Context, input Req) (Res, error) {
	var zero Res
	if !b.alive.Load() {
		return zero, ErrBatcherDead
	}

	resultCh := make(chan Result[Res], 1)
	req := Request[Req, Res]{Input: input, ResultCh: resultCh}

	if b.cfg.DropOnFull {
		select {
		case b.requestCh <- req:
		default:
			b.dropped.Add(1)
			return zero, ErrBatcherDead
		}
	} else {
		select {
		case b.requestCh <- req:
		case <-ctx.Done():
			return zero, ctx.Err()
		}
	}

	select {
	case result := <-resultCh:
		return result.Value, result.Err
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

func (b *Batcher[Req, Res]) SubmitAsync(input Req) bool {
	if !b.alive.Load() {
		b.dropped.Add(1)
		return false
	}

	req := Request[Req, Res]{Input: input, ResultCh: nil}

	select {
	case b.requestCh <- req:
		return true
	default:
		b.dropped.Add(1)
		return false
	}
}

func (b *Batcher[Req, Res]) DroppedCount() uint64 {
	return b.dropped.Load()
}

func (b *Batcher[Req, Res]) SwapDroppedCount() uint64 {
	return b.dropped.Swap(0)
}

func (b *Batcher[Req, Res]) runFlushLoop(ctx context.Context) {
	defer b.wg.Done()

	ticker := time.NewTicker(time.Duration(b.cfg.FlushMs) * time.Millisecond)
	defer ticker.Stop()

	batch := b.newBatch()

	for {
		select {
		case <-ctx.Done():
			b.flushBatch(context.Background(), &batch)
			b.drainAndFlush()
			return
		case req := <-b.requestCh:
			batch = append(batch, req)
			if len(batch) >= b.cfg.BatchSize {
				b.flushBatch(ctx, &batch)
			}
		case <-ticker.C:
			b.flushBatch(ctx, &batch)
		}
	}
}

func (b *Batcher[Req, Res]) drainAndFlush() {
	batch := b.newBatch()
	for {
		select {
		case req := <-b.requestCh:
			batch = append(batch, req)
		default:
			if len(batch) > 0 {
				ctx, cancel := context.WithTimeout(context.Background(), b.cfg.DrainTimeout)
				b.flushFn(ctx, batch)
				cancel()
			}
			return
		}
	}
}

func (b *Batcher[Req, Res]) newBatch() []Request[Req, Res] {
	return make([]Request[Req, Res], 0, b.cfg.BatchSize)
}

func (b *Batcher[Req, Res]) flushBatch(ctx context.Context, batch *[]Request[Req, Res]) {
	if len(*batch) == 0 {
		return
	}
	b.flushFn(ctx, *batch)
	*batch = (*batch)[:0]
}
