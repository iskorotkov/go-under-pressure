-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- HTTP request metrics (hypertable)
CREATE TABLE IF NOT EXISTS http_metrics (
    time        TIMESTAMPTZ NOT NULL,
    method      TEXT NOT NULL,
    path        TEXT NOT NULL,
    status_code INT NOT NULL,
    duration_ms DOUBLE PRECISION NOT NULL,
    client_ip   TEXT,
    error       TEXT
);

SELECT create_hypertable('http_metrics', by_range('time', INTERVAL '1 day'), if_not_exists => TRUE);

-- Business metrics (hypertable)
CREATE TABLE IF NOT EXISTS business_metrics (
    time        TIMESTAMPTZ NOT NULL,
    metric_name TEXT NOT NULL,
    value       DOUBLE PRECISION NOT NULL,
    labels      JSONB
);

SELECT create_hypertable('business_metrics', by_range('time', INTERVAL '1 day'), if_not_exists => TRUE);

-- Infrastructure metrics (hypertable)
CREATE TABLE IF NOT EXISTS infra_metrics (
    time            TIMESTAMPTZ NOT NULL,
    pool_acquired   INT,
    pool_idle       INT,
    pool_total      INT,
    pool_max        INT,
    cache_hits      BIGINT,
    cache_misses    BIGINT,
    cache_hit_ratio DOUBLE PRECISION,
    goroutines      INT,
    heap_alloc_mb   DOUBLE PRECISION
);

SELECT create_hypertable('infra_metrics', by_range('time', INTERVAL '1 day'), if_not_exists => TRUE);

-- Hourly continuous aggregate for HTTP metrics
CREATE MATERIALIZED VIEW IF NOT EXISTS http_metrics_hourly
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
    method,
    path,
    COUNT(*) AS request_count,
    COUNT(*) FILTER (WHERE status_code >= 500) AS error_count,
    AVG(duration_ms) AS avg_duration_ms,
    percentile_cont(0.50) WITHIN GROUP (ORDER BY duration_ms) AS p50_ms,
    percentile_cont(0.90) WITHIN GROUP (ORDER BY duration_ms) AS p90_ms,
    percentile_cont(0.99) WITHIN GROUP (ORDER BY duration_ms) AS p99_ms
FROM http_metrics
GROUP BY bucket, method, path
WITH NO DATA;

-- Refresh policy: every 10 minutes, materialize up to 1 hour ago
-- start_offset - end_offset must cover at least 2 buckets (2 hours for hourly buckets)
SELECT add_continuous_aggregate_policy('http_metrics_hourly',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '10 minutes',
    if_not_exists => TRUE);

-- Retention policies (30 days raw, 90 days aggregated)
SELECT add_retention_policy('http_metrics', INTERVAL '30 days', if_not_exists => TRUE);
SELECT add_retention_policy('business_metrics', INTERVAL '30 days', if_not_exists => TRUE);
SELECT add_retention_policy('infra_metrics', INTERVAL '30 days', if_not_exists => TRUE);
SELECT add_retention_policy('http_metrics_hourly', INTERVAL '90 days', if_not_exists => TRUE);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_http_metrics_path_time ON http_metrics (path, time DESC);
CREATE INDEX IF NOT EXISTS idx_business_metrics_name_time ON business_metrics (metric_name, time DESC);
