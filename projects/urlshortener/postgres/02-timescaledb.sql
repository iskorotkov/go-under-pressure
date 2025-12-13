-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- ============================================================================
-- HTTP request metrics (hypertable)
-- ============================================================================
CREATE TABLE IF NOT EXISTS http_metrics (
    time        TIMESTAMPTZ NOT NULL,
    method      TEXT NOT NULL,
    path        TEXT NOT NULL,
    status_code SMALLINT NOT NULL,
    duration_ms REAL NOT NULL,
    client_ip   TEXT,
    error       TEXT
);

-- Create hypertable with 1-hour chunks for faster compression
SELECT create_hypertable('http_metrics', by_range('time', INTERVAL '1 hour'), if_not_exists => TRUE);

-- Enable compression with automatic inline compression after 1 hour
ALTER TABLE http_metrics SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'method, path',
    timescaledb.compress_orderby = 'time DESC',
    timescaledb.compress_chunk_time_interval = '1 hour'
);

-- Backup compression policy (catches anything missed by inline compression)
SELECT add_compression_policy('http_metrics', INTERVAL '2 hours', if_not_exists => TRUE);

-- ============================================================================
-- Business metrics (hypertable)
-- ============================================================================
CREATE TABLE IF NOT EXISTS business_metrics (
    time        TIMESTAMPTZ NOT NULL,
    metric_name TEXT NOT NULL,
    value       DOUBLE PRECISION NOT NULL,
    labels      JSONB
);

-- Create hypertable with 1-hour chunks
SELECT create_hypertable('business_metrics', by_range('time', INTERVAL '1 hour'), if_not_exists => TRUE);

-- Enable compression with automatic inline compression
ALTER TABLE business_metrics SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'metric_name',
    timescaledb.compress_orderby = 'time DESC',
    timescaledb.compress_chunk_time_interval = '1 hour'
);

SELECT add_compression_policy('business_metrics', INTERVAL '2 hours', if_not_exists => TRUE);

-- ============================================================================
-- Infrastructure metrics (hypertable)
-- ============================================================================
CREATE TABLE IF NOT EXISTS infra_metrics (
    time            TIMESTAMPTZ NOT NULL,
    pool_acquired   INT,
    pool_idle       INT,
    pool_total      INT,
    pool_max        INT,
    cache_hits      BIGINT,
    cache_misses    BIGINT,
    cache_hit_ratio REAL,
    goroutines      INT,
    heap_alloc_mb   REAL
);

-- Create hypertable with 1-hour chunks
SELECT create_hypertable('infra_metrics', by_range('time', INTERVAL '1 hour'), if_not_exists => TRUE);

-- Enable compression with automatic inline compression
ALTER TABLE infra_metrics SET (
    timescaledb.compress,
    timescaledb.compress_orderby = 'time DESC',
    timescaledb.compress_chunk_time_interval = '1 hour'
);

SELECT add_compression_policy('infra_metrics', INTERVAL '2 hours', if_not_exists => TRUE);

-- ============================================================================
-- Continuous aggregate for HTTP metrics
-- ============================================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS http_metrics_agg
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
    method,
    path,
    COUNT(*) AS request_count,
    COUNT(*) FILTER (WHERE status_code >= 200 AND status_code < 300) AS count_2xx,
    COUNT(*) FILTER (WHERE status_code >= 300 AND status_code < 400) AS count_3xx,
    COUNT(*) FILTER (WHERE status_code >= 400 AND status_code < 500) AS count_4xx,
    COUNT(*) FILTER (WHERE status_code >= 500) AS error_count,
    AVG(duration_ms) AS avg_duration_ms,
    percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms) AS p50_ms,
    percentile_cont(0.90) WITHIN GROUP (ORDER BY duration_ms) AS p90_ms,
    percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms) AS p99_ms
FROM http_metrics
GROUP BY bucket, method, path
WITH NO DATA;

ALTER MATERIALIZED VIEW http_metrics_agg SET (timescaledb.materialized_only = false);

SELECT add_continuous_aggregate_policy('http_metrics_agg',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '10 seconds',
    schedule_interval => INTERVAL '10 seconds',
    if_not_exists => TRUE);

-- ============================================================================
-- Retention policies (30 days raw, 90 days aggregated)
-- ============================================================================
SELECT add_retention_policy('http_metrics', INTERVAL '30 days', if_not_exists => TRUE);
SELECT add_retention_policy('business_metrics', INTERVAL '30 days', if_not_exists => TRUE);
SELECT add_retention_policy('infra_metrics', INTERVAL '30 days', if_not_exists => TRUE);
SELECT add_retention_policy('http_metrics_agg', INTERVAL '90 days', if_not_exists => TRUE);

-- ============================================================================
-- Indexes for common queries
-- ============================================================================
CREATE INDEX IF NOT EXISTS idx_http_metrics_path_time ON http_metrics (path, time DESC);
CREATE INDEX IF NOT EXISTS idx_business_metrics_name_time ON business_metrics (metric_name, time DESC);

-- Partial indexes for high-volume business metrics
CREATE INDEX IF NOT EXISTS idx_business_metrics_redirects_time
ON business_metrics (time DESC)
WHERE metric_name = 'redirects';

CREATE INDEX IF NOT EXISTS idx_business_metrics_unique_visitors_time
ON business_metrics (time DESC)
WHERE metric_name = 'unique_visitors';

CREATE INDEX IF NOT EXISTS idx_business_metrics_cache_time
ON business_metrics (time DESC)
WHERE metric_name IN ('cache_hit', 'cache_miss');

-- ============================================================================
-- Continuous aggregate for redirects (pre-aggregates per short_code)
-- ============================================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS business_metrics_redirects_agg
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 minute', time) AS bucket,
    labels->>'short_code' AS short_code,
    labels->>'original_url' AS original_url,
    SUM(value) AS total_redirects,
    COUNT(*) AS count
FROM business_metrics
WHERE metric_name = 'redirects'
GROUP BY bucket, labels->>'short_code', labels->>'original_url'
WITH NO DATA;

ALTER MATERIALIZED VIEW business_metrics_redirects_agg SET (timescaledb.materialized_only = false);

SELECT add_continuous_aggregate_policy('business_metrics_redirects_agg',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '10 seconds',
    schedule_interval => INTERVAL '10 seconds',
    if_not_exists => TRUE);

-- ============================================================================
-- Continuous aggregate for unique visitors
-- ============================================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS business_metrics_visitors_agg
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 minute', time) AS bucket,
    COUNT(*) AS total_visits
FROM business_metrics
WHERE metric_name = 'unique_visitors'
GROUP BY bucket
WITH NO DATA;

ALTER MATERIALIZED VIEW business_metrics_visitors_agg SET (timescaledb.materialized_only = false);

SELECT add_continuous_aggregate_policy('business_metrics_visitors_agg',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '10 seconds',
    schedule_interval => INTERVAL '10 seconds',
    if_not_exists => TRUE);

-- ============================================================================
-- Continuous aggregate for cache metrics
-- ============================================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS business_metrics_cache_agg
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 minute', time) AS bucket,
    SUM(CASE WHEN metric_name = 'cache_hit' THEN value ELSE 0 END) AS cache_hits,
    SUM(CASE WHEN metric_name = 'cache_miss' THEN value ELSE 0 END) AS cache_misses
FROM business_metrics
WHERE metric_name IN ('cache_hit', 'cache_miss')
GROUP BY bucket
WITH NO DATA;

ALTER MATERIALIZED VIEW business_metrics_cache_agg SET (timescaledb.materialized_only = false);

SELECT add_continuous_aggregate_policy('business_metrics_cache_agg',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '10 seconds',
    schedule_interval => INTERVAL '10 seconds',
    if_not_exists => TRUE);

-- ============================================================================
-- Continuous aggregate for URLs created
-- ============================================================================
CREATE MATERIALIZED VIEW IF NOT EXISTS business_metrics_urls_agg
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('5 minutes', time) AS bucket,
    labels->>'method' AS method,
    SUM(value) AS total_created,
    COUNT(*) AS count
FROM business_metrics
WHERE metric_name = 'urls_created'
GROUP BY bucket, labels->>'method'
WITH NO DATA;

ALTER MATERIALIZED VIEW business_metrics_urls_agg SET (timescaledb.materialized_only = false);

SELECT add_continuous_aggregate_policy('business_metrics_urls_agg',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '10 seconds',
    schedule_interval => INTERVAL '10 seconds',
    if_not_exists => TRUE);

-- ============================================================================
-- Retention policy for aggregates (90 days)
-- ============================================================================
SELECT add_retention_policy('business_metrics_redirects_agg', INTERVAL '90 days', if_not_exists => TRUE);
SELECT add_retention_policy('business_metrics_visitors_agg', INTERVAL '90 days', if_not_exists => TRUE);
SELECT add_retention_policy('business_metrics_cache_agg', INTERVAL '90 days', if_not_exists => TRUE);
SELECT add_retention_policy('business_metrics_urls_agg', INTERVAL '90 days', if_not_exists => TRUE);
