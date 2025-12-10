package metrics

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"urlshortener/internal/config"
)

type Recorder struct {
	pool          *pgxpool.Pool
	logger        *slog.Logger
	cfg           *config.MetricsConfig
	httpCh        chan HTTPMetric
	businessCh    chan BusinessMetric
	infraCh       chan InfraMetric
	wg            sync.WaitGroup
	shutdownOnce  sync.Once
	shutdownCh    chan struct{}
}

func NewRecorder(pool *pgxpool.Pool, cfg *config.MetricsConfig, logger *slog.Logger) *Recorder {
	return &Recorder{
		pool:       pool,
		logger:     logger,
		cfg:        cfg,
		httpCh:     make(chan HTTPMetric, cfg.BufferSize),
		businessCh: make(chan BusinessMetric, cfg.BufferSize),
		infraCh:    make(chan InfraMetric, cfg.BufferSize),
		shutdownCh: make(chan struct{}),
	}
}

func (r *Recorder) RecordHTTP(m HTTPMetric) {
	if !r.cfg.Enabled {
		return
	}
	select {
	case r.httpCh <- m:
	default:
		r.logger.Warn("http metrics buffer full, dropping metric")
	}
}

func (r *Recorder) RecordBusiness(name string, value float64, labels map[string]string) {
	if !r.cfg.Enabled {
		return
	}
	m := BusinessMetric{
		Time:       time.Now(),
		MetricName: name,
		Value:      value,
		Labels:     labels,
	}
	select {
	case r.businessCh <- m:
	default:
		r.logger.Warn("business metrics buffer full, dropping metric")
	}
}

func (r *Recorder) RecordInfra(m InfraMetric) {
	if !r.cfg.Enabled {
		return
	}
	select {
	case r.infraCh <- m:
	default:
		r.logger.Warn("infra metrics buffer full, dropping metric")
	}
}

func (r *Recorder) Start(ctx context.Context) {
	if !r.cfg.Enabled {
		r.logger.Info("metrics recording disabled")
		return
	}

	flushInterval := time.Duration(r.cfg.FlushInterval) * time.Millisecond

	r.wg.Add(3)
	go r.flushHTTPMetrics(ctx, flushInterval)
	go r.flushBusinessMetrics(ctx, flushInterval)
	go r.flushInfraMetrics(ctx, flushInterval)

	r.logger.Info("metrics recorder started",
		slog.Int("buffer_size", r.cfg.BufferSize),
		slog.Int("flush_interval_ms", r.cfg.FlushInterval))
}

func (r *Recorder) Close() {
	r.shutdownOnce.Do(func() {
		close(r.shutdownCh)
		r.wg.Wait()
	})
}

func (r *Recorder) flushHTTPMetrics(ctx context.Context, interval time.Duration) {
	defer r.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	batch := make([]HTTPMetric, 0, r.cfg.BufferSize)

	for {
		select {
		case <-ctx.Done():
			r.drainAndFlushHTTP(batch)
			return
		case <-r.shutdownCh:
			r.drainAndFlushHTTP(batch)
			return
		case m := <-r.httpCh:
			batch = append(batch, m)
			if len(batch) >= r.cfg.FlushThreshold {
				r.writeHTTPBatch(ctx, batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				r.writeHTTPBatch(ctx, batch)
				batch = batch[:0]
			}
		}
	}
}

func (r *Recorder) drainAndFlushHTTP(batch []HTTPMetric) {
	for {
		select {
		case m := <-r.httpCh:
			batch = append(batch, m)
		default:
			if len(batch) > 0 {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				r.writeHTTPBatch(ctx, batch)
				cancel()
			}
			return
		}
	}
}

func (r *Recorder) writeHTTPBatch(ctx context.Context, batch []HTTPMetric) {
	if len(batch) == 0 {
		return
	}

	rows := make([][]any, len(batch))
	for i, m := range batch {
		rows[i] = []any{m.Time, m.Method, m.Path, m.StatusCode, m.DurationMs, m.ClientIP, m.Error}
	}

	_, err := r.pool.CopyFrom(ctx,
		pgx.Identifier{"http_metrics"},
		[]string{"time", "method", "path", "status_code", "duration_ms", "client_ip", "error"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		r.logger.Error("failed to write http metrics batch", slog.String("error", err.Error()))
	}
}

func (r *Recorder) flushBusinessMetrics(ctx context.Context, interval time.Duration) {
	defer r.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	batch := make([]BusinessMetric, 0, r.cfg.BufferSize)

	for {
		select {
		case <-ctx.Done():
			r.drainAndFlushBusiness(batch)
			return
		case <-r.shutdownCh:
			r.drainAndFlushBusiness(batch)
			return
		case m := <-r.businessCh:
			batch = append(batch, m)
			if len(batch) >= r.cfg.FlushThreshold {
				r.writeBusinessBatch(ctx, batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				r.writeBusinessBatch(ctx, batch)
				batch = batch[:0]
			}
		}
	}
}

func (r *Recorder) drainAndFlushBusiness(batch []BusinessMetric) {
	for {
		select {
		case m := <-r.businessCh:
			batch = append(batch, m)
		default:
			if len(batch) > 0 {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				r.writeBusinessBatch(ctx, batch)
				cancel()
			}
			return
		}
	}
}

func (r *Recorder) writeBusinessBatch(ctx context.Context, batch []BusinessMetric) {
	if len(batch) == 0 {
		return
	}

	rows := make([][]any, len(batch))
	for i, m := range batch {
		labelsJSON, _ := json.Marshal(m.Labels)
		rows[i] = []any{m.Time, m.MetricName, m.Value, labelsJSON}
	}

	_, err := r.pool.CopyFrom(ctx,
		pgx.Identifier{"business_metrics"},
		[]string{"time", "metric_name", "value", "labels"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		r.logger.Error("failed to write business metrics batch", slog.String("error", err.Error()))
	}
}

func (r *Recorder) flushInfraMetrics(ctx context.Context, interval time.Duration) {
	defer r.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	batch := make([]InfraMetric, 0, r.cfg.BufferSize)

	for {
		select {
		case <-ctx.Done():
			r.drainAndFlushInfra(batch)
			return
		case <-r.shutdownCh:
			r.drainAndFlushInfra(batch)
			return
		case m := <-r.infraCh:
			batch = append(batch, m)
			if len(batch) >= r.cfg.FlushThreshold {
				r.writeInfraBatch(ctx, batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				r.writeInfraBatch(ctx, batch)
				batch = batch[:0]
			}
		}
	}
}

func (r *Recorder) drainAndFlushInfra(batch []InfraMetric) {
	for {
		select {
		case m := <-r.infraCh:
			batch = append(batch, m)
		default:
			if len(batch) > 0 {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				r.writeInfraBatch(ctx, batch)
				cancel()
			}
			return
		}
	}
}

func (r *Recorder) writeInfraBatch(ctx context.Context, batch []InfraMetric) {
	if len(batch) == 0 {
		return
	}

	rows := make([][]any, len(batch))
	for i, m := range batch {
		rows[i] = []any{
			m.Time, m.PoolAcquired, m.PoolIdle, m.PoolTotal, m.PoolMax,
			m.CacheHits, m.CacheMisses, m.CacheHitRatio, m.Goroutines, m.HeapAllocMB,
		}
	}

	_, err := r.pool.CopyFrom(ctx,
		pgx.Identifier{"infra_metrics"},
		[]string{
			"time", "pool_acquired", "pool_idle", "pool_total", "pool_max",
			"cache_hits", "cache_misses", "cache_hit_ratio", "goroutines", "heap_alloc_mb",
		},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		r.logger.Error("failed to write infra metrics batch", slog.String("error", err.Error()))
	}
}
