package metrics

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"urlshortener/internal/batcher"
	"urlshortener/internal/config"
)

type Recorder struct {
	pool         *pgxpool.Pool
	logger       *slog.Logger
	cfg          *config.MetricsConfig
	httpBatcher  *batcher.Batcher[HTTPMetric, struct{}]
	busiBatcher  *batcher.Batcher[BusinessMetric, struct{}]
	infraBatcher *batcher.Batcher[InfraMetric, struct{}]
}

func NewRecorder(pool *pgxpool.Pool, cfg *config.MetricsConfig, logger *slog.Logger) *Recorder {
	r := &Recorder{
		pool:   pool,
		logger: logger,
		cfg:    cfg,
	}

	batcherCfg := batcher.Config{
		BatchSize:    cfg.FlushThreshold,
		FlushMs:      cfg.FlushInterval,
		MaxWorkers:   1,
		DropOnFull:   true,
		DrainTimeout: 5 * time.Second,
	}

	r.httpBatcher = batcher.New(batcherCfg, r.flushHTTP)
	r.busiBatcher = batcher.New(batcherCfg, r.flushBusiness)
	r.infraBatcher = batcher.New(batcherCfg, r.flushInfra)

	return r
}

func (r *Recorder) RecordHTTP(m HTTPMetric) {
	if !r.cfg.Enabled {
		return
	}
	r.httpBatcher.SubmitAsync(m)
}

func (r *Recorder) RecordBusiness(t time.Time, name string, value float64, labelsJSON []byte) {
	if !r.cfg.Enabled {
		return
	}
	r.busiBatcher.SubmitAsync(BusinessMetric{
		Time:       t,
		MetricName: name,
		Value:      value,
		LabelsJSON: labelsJSON,
	})
}

func (r *Recorder) RecordInfra(m InfraMetric) {
	if !r.cfg.Enabled {
		return
	}
	r.infraBatcher.SubmitAsync(m)
}

func (r *Recorder) Start(ctx context.Context) {
	if !r.cfg.Enabled {
		r.logger.Info("metrics recording disabled")
		return
	}

	r.httpBatcher.Start(ctx)
	r.busiBatcher.Start(ctx)
	r.infraBatcher.Start(ctx)

	r.logger.Info("metrics recorder started",
		slog.Int("flush_threshold", r.cfg.FlushThreshold),
		slog.Int("flush_interval_ms", r.cfg.FlushInterval))
}

func (r *Recorder) Close() {
	r.httpBatcher.Close()
	r.busiBatcher.Close()
	r.infraBatcher.Close()

	r.logDropped("http", r.httpBatcher.DroppedCount())
	r.logDropped("business", r.busiBatcher.DroppedCount())
	r.logDropped("infra", r.infraBatcher.DroppedCount())
}

func (r *Recorder) logDropped(name string, count uint64) {
	if count > 0 {
		r.logger.Warn("dropped metrics on shutdown", slog.String("type", name), slog.Uint64("count", count))
	}
}

func (r *Recorder) flushHTTP(ctx context.Context, batch []batcher.Request[HTTPMetric, struct{}]) {
	rows := make([][]any, len(batch))
	for i, req := range batch {
		m := req.Input
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

	if dropped := r.httpBatcher.SwapDroppedCount(); dropped > 0 {
		r.logger.Warn("dropped http metrics", slog.Uint64("count", dropped))
	}
}

func (r *Recorder) flushBusiness(ctx context.Context, batch []batcher.Request[BusinessMetric, struct{}]) {
	rows := make([][]any, len(batch))
	for i, req := range batch {
		m := req.Input
		rows[i] = []any{m.Time, m.MetricName, m.Value, m.LabelsJSON}
	}

	_, err := r.pool.CopyFrom(ctx,
		pgx.Identifier{"business_metrics"},
		[]string{"time", "metric_name", "value", "labels"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		r.logger.Error("failed to write business metrics batch", slog.String("error", err.Error()))
	}

	if dropped := r.busiBatcher.SwapDroppedCount(); dropped > 0 {
		r.logger.Warn("dropped business metrics", slog.Uint64("count", dropped))
	}
}

func (r *Recorder) flushInfra(ctx context.Context, batch []batcher.Request[InfraMetric, struct{}]) {
	rows := make([][]any, len(batch))
	for i, req := range batch {
		m := req.Input
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

	if dropped := r.infraBatcher.SwapDroppedCount(); dropped > 0 {
		r.logger.Warn("dropped infra metrics", slog.Uint64("count", dropped))
	}
}
