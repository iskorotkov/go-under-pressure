package batcher

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatcher_FlushOnBatchSize(t *testing.T) {
	var totalItems atomic.Int32

	b := New(Config{
		BatchSize:  3,
		FlushMs:    1000,
		MaxWorkers: 1,
	}, func(_ context.Context, batch []Request[int, int]) {
		totalItems.Add(int32(len(batch)))
		for _, req := range batch {
			if req.ResultCh != nil {
				req.ResultCh <- Result[int]{Value: req.Input * 2}
			}
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	b.Start(ctx)

	results := make([]int, 3)
	for i := range 3 {
		res, err := b.Submit(ctx, i+1)
		require.NoError(t, err)
		results[i] = res
	}

	assert.Equal(t, []int{2, 4, 6}, results)
	assert.Equal(t, int32(3), totalItems.Load())

	cancel()
	b.Close()
}

func TestBatcher_FlushOnTimer(t *testing.T) {
	var flushCount atomic.Int32

	b := New(Config{
		BatchSize:  100,
		FlushMs:    50,
		MaxWorkers: 1,
	}, func(_ context.Context, batch []Request[int, int]) {
		flushCount.Add(1)
		for _, req := range batch {
			if req.ResultCh != nil {
				req.ResultCh <- Result[int]{Value: req.Input}
			}
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	b.Start(ctx)

	res, err := b.Submit(ctx, 42)
	require.NoError(t, err)
	assert.Equal(t, 42, res)
	assert.GreaterOrEqual(t, flushCount.Load(), int32(1))

	cancel()
	b.Close()
}

func TestBatcher_SubmitAsync(t *testing.T) {
	var processed atomic.Int32

	b := New(Config{
		BatchSize:  2,
		FlushMs:    50,
		MaxWorkers: 1,
	}, func(_ context.Context, batch []Request[int, int]) {
		for _, req := range batch {
			processed.Add(1)
			if req.ResultCh != nil {
				req.ResultCh <- Result[int]{Value: req.Input}
			}
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	b.Start(ctx)

	ok := b.SubmitAsync(1)
	assert.True(t, ok)
	ok = b.SubmitAsync(2)
	assert.True(t, ok)

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(2), processed.Load())

	cancel()
	b.Close()
}

func TestBatcher_DropOnFull(t *testing.T) {
	b := New(Config{
		BatchSize:  1,
		FlushMs:    1000,
		MaxWorkers: 1,
		DropOnFull: true,
	}, func(_ context.Context, batch []Request[int, int]) {
		time.Sleep(100 * time.Millisecond)
		for _, req := range batch {
			if req.ResultCh != nil {
				req.ResultCh <- Result[int]{Value: req.Input}
			}
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	b.Start(ctx)

	b.SubmitAsync(1)

	for range 10 {
		b.SubmitAsync(999)
	}

	dropped := b.DroppedCount()
	assert.Greater(t, dropped, uint64(0))

	cancel()
	b.Close()
}

func TestBatcher_ContextCancellation(t *testing.T) {
	b := New(Config{
		BatchSize:  100,
		FlushMs:    1000,
		MaxWorkers: 1,
	}, func(_ context.Context, batch []Request[int, int]) {
		for _, req := range batch {
			if req.ResultCh != nil {
				req.ResultCh <- Result[int]{Value: req.Input}
			}
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	b.Start(ctx)

	shortCtx, shortCancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer shortCancel()

	_, err := b.Submit(shortCtx, 1)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	cancel()
	b.Close()
}

func TestBatcher_DrainOnClose(t *testing.T) {
	var flushedItems atomic.Int32

	b := New(Config{
		BatchSize:    100,
		FlushMs:      1000,
		MaxWorkers:   1,
		DrainTimeout: time.Second,
	}, func(_ context.Context, batch []Request[int, int]) {
		flushedItems.Add(int32(len(batch)))
		for _, req := range batch {
			if req.ResultCh != nil {
				req.ResultCh <- Result[int]{Value: req.Input}
			}
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	b.Start(ctx)

	for range 5 {
		b.SubmitAsync(1)
	}

	cancel()
	b.Close()

	assert.Equal(t, int32(5), flushedItems.Load())
}

func TestBatcher_ConcurrentSubmit(t *testing.T) {
	var totalProcessed atomic.Int32

	b := New(Config{
		BatchSize:  10,
		FlushMs:    10,
		MaxWorkers: 4,
	}, func(_ context.Context, batch []Request[int, int]) {
		totalProcessed.Add(int32(len(batch)))
		for _, req := range batch {
			if req.ResultCh != nil {
				req.ResultCh <- Result[int]{Value: req.Input * 2}
			}
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	b.Start(ctx)

	var wg sync.WaitGroup
	numGoroutines := 10
	submitsPerGoroutine := 100

	for range numGoroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range submitsPerGoroutine {
				res, err := b.Submit(ctx, j)
				assert.NoError(t, err)
				assert.Equal(t, j*2, res)
			}
		}()
	}

	wg.Wait()
	cancel()
	b.Close()

	assert.Equal(t, int32(numGoroutines*submitsPerGoroutine), totalProcessed.Load())
}

func TestBatcher_SubmitAfterClose(t *testing.T) {
	b := New(Config{
		BatchSize:  10,
		FlushMs:    100,
		MaxWorkers: 1,
	}, func(_ context.Context, batch []Request[int, int]) {
		for _, req := range batch {
			if req.ResultCh != nil {
				req.ResultCh <- Result[int]{Value: req.Input}
			}
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	b.Start(ctx)
	cancel()
	b.Close()

	_, err := b.Submit(context.Background(), 1)
	assert.ErrorIs(t, err, ErrBatcherDead)

	ok := b.SubmitAsync(1)
	assert.False(t, ok)
	assert.Equal(t, uint64(1), b.DroppedCount())
}

func TestBatcher_SwapDroppedCount(t *testing.T) {
	b := New(Config{
		BatchSize:  1,
		FlushMs:    1000,
		MaxWorkers: 1,
		DropOnFull: true,
	}, func(_ context.Context, _ []Request[int, int]) {
		time.Sleep(50 * time.Millisecond)
	})

	ctx, cancel := context.WithCancel(context.Background())
	b.Start(ctx)

	b.SubmitAsync(1)
	time.Sleep(10 * time.Millisecond)

	for range 5 {
		b.SubmitAsync(999)
	}

	dropped := b.SwapDroppedCount()
	assert.Greater(t, dropped, uint64(0))
	assert.Equal(t, uint64(0), b.DroppedCount())

	cancel()
	b.Close()
}

func TestBatcher_ErrorPropagation(t *testing.T) {
	expectedErr := assert.AnError

	b := New(Config{
		BatchSize:  1,
		FlushMs:    100,
		MaxWorkers: 1,
	}, func(_ context.Context, batch []Request[int, int]) {
		for _, req := range batch {
			if req.ResultCh != nil {
				req.ResultCh <- Result[int]{Err: expectedErr}
			}
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	b.Start(ctx)

	_, err := b.Submit(ctx, 1)
	assert.ErrorIs(t, err, expectedErr)

	cancel()
	b.Close()
}
