-- PostgreSQL/TimescaleDB internal metrics (hypertable)
CREATE TABLE IF NOT EXISTS pg_metrics (
    time                    TIMESTAMPTZ NOT NULL,
    -- Connection stats
    active_connections      INT,
    idle_connections        INT,
    max_connections         INT,
    -- Transaction stats (cumulative)
    xact_commit             BIGINT,
    xact_rollback           BIGINT,
    -- Buffer stats
    blks_read               BIGINT,
    blks_hit                BIGINT,
    buffer_hit_ratio        DOUBLE PRECISION,
    -- Tuple operations (cumulative)
    tup_inserted            BIGINT,
    tup_updated             BIGINT,
    tup_deleted             BIGINT,
    tup_fetched             BIGINT,
    -- Scan stats (cumulative)
    seq_scan                BIGINT,
    idx_scan                BIGINT,
    -- Dead tuples
    dead_tuples             BIGINT,
    -- Lock counts
    locks_total             INT,
    -- TimescaleDB
    hypertable_count        INT,
    chunk_count             INT,
    total_hypertable_bytes  BIGINT,
    -- Table/Index sizes
    total_table_bytes       BIGINT,
    total_index_bytes       BIGINT,
    total_toast_bytes       BIGINT,
    total_rows              BIGINT
);

SELECT create_hypertable('pg_metrics', by_range('time', INTERVAL '1 day'), if_not_exists => TRUE);
SELECT add_retention_policy('pg_metrics', INTERVAL '30 days', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_pg_metrics_time ON pg_metrics (time DESC);

-- Per-table metrics hypertable
CREATE TABLE IF NOT EXISTS pg_metrics_per_table (
    time              TIMESTAMPTZ NOT NULL,
    schema_name       TEXT NOT NULL,
    table_name        TEXT NOT NULL,
    seq_scans         BIGINT,
    idx_scans         BIGINT,
    n_live_tup        BIGINT,
    n_dead_tup        BIGINT,
    table_size_bytes  BIGINT,
    index_size_bytes  BIGINT
);

SELECT create_hypertable('pg_metrics_per_table', by_range('time', INTERVAL '1 day'), if_not_exists => TRUE);
SELECT add_retention_policy('pg_metrics_per_table', INTERVAL '30 days', if_not_exists => TRUE);
CREATE INDEX IF NOT EXISTS idx_pg_metrics_per_table_time ON pg_metrics_per_table (time DESC);
CREATE INDEX IF NOT EXISTS idx_pg_metrics_per_table_name ON pg_metrics_per_table (table_name, time DESC);

-- Collector function for TimescaleDB job scheduler
CREATE OR REPLACE FUNCTION collect_pg_metrics(job_id INT, config JSONB)
RETURNS VOID AS $$
DECLARE
    v_active INT;
    v_idle INT;
    v_max INT;
    v_xact_commit BIGINT;
    v_xact_rollback BIGINT;
    v_blks_read BIGINT;
    v_blks_hit BIGINT;
    v_buffer_hit_ratio DOUBLE PRECISION;
    v_tup_inserted BIGINT;
    v_tup_updated BIGINT;
    v_tup_deleted BIGINT;
    v_tup_fetched BIGINT;
    v_seq_scan BIGINT;
    v_idx_scan BIGINT;
    v_dead_tuples BIGINT;
    v_locks_total INT;
    v_hypertable_count INT;
    v_chunk_count INT;
    v_total_bytes BIGINT;
    v_table_bytes BIGINT;
    v_index_bytes BIGINT;
    v_toast_bytes BIGINT;
    v_total_rows BIGINT;
BEGIN
    -- Connection stats from pg_stat_activity
    SELECT
        COUNT(*) FILTER (WHERE state = 'active'),
        COUNT(*) FILTER (WHERE state = 'idle'),
        (SELECT setting::int FROM pg_settings WHERE name = 'max_connections')
    INTO v_active, v_idle, v_max
    FROM pg_stat_activity
    WHERE datname = current_database();

    -- Database stats from pg_stat_database
    SELECT
        xact_commit,
        xact_rollback,
        blks_read,
        blks_hit,
        CASE WHEN blks_read + blks_hit > 0
             THEN blks_hit::float / (blks_read + blks_hit)
             ELSE 0
        END,
        tup_inserted,
        tup_updated,
        tup_deleted,
        tup_fetched
    INTO
        v_xact_commit,
        v_xact_rollback,
        v_blks_read,
        v_blks_hit,
        v_buffer_hit_ratio,
        v_tup_inserted,
        v_tup_updated,
        v_tup_deleted,
        v_tup_fetched
    FROM pg_stat_database
    WHERE datname = current_database();

    -- Table stats (aggregated) from pg_stat_user_tables
    SELECT
        COALESCE(SUM(seq_scan), 0),
        COALESCE(SUM(idx_scan), 0),
        COALESCE(SUM(n_dead_tup), 0)
    INTO v_seq_scan, v_idx_scan, v_dead_tuples
    FROM pg_stat_user_tables;

    -- Lock count from pg_locks
    SELECT COUNT(*)
    INTO v_locks_total
    FROM pg_locks
    WHERE database = (SELECT oid FROM pg_database WHERE datname = current_database());

    -- TimescaleDB hypertable stats
    SELECT COUNT(*)
    INTO v_hypertable_count
    FROM timescaledb_information.hypertables;

    SELECT COUNT(*)
    INTO v_chunk_count
    FROM timescaledb_information.chunks;

    -- Get total size using hypertable_size() function
    SELECT COALESCE(SUM(hypertable_size(format('%I.%I', hypertable_schema, hypertable_name)::regclass)), 0)
    INTO v_total_bytes
    FROM timescaledb_information.hypertables;

    -- Table/Index/TOAST sizes and row counts from pg_stat_user_tables
    SELECT
        COALESCE(SUM(pg_table_size(schemaname || '.' || relname)), 0),
        COALESCE(SUM(pg_indexes_size(schemaname || '.' || relname)), 0),
        COALESCE(SUM(pg_total_relation_size(schemaname || '.' || relname)
                   - pg_table_size(schemaname || '.' || relname)
                   - pg_indexes_size(schemaname || '.' || relname)), 0),
        COALESCE(SUM(n_live_tup), 0)
    INTO v_table_bytes, v_index_bytes, v_toast_bytes, v_total_rows
    FROM pg_stat_user_tables;

    -- Insert metric row
    INSERT INTO pg_metrics (
        time,
        active_connections,
        idle_connections,
        max_connections,
        xact_commit,
        xact_rollback,
        blks_read,
        blks_hit,
        buffer_hit_ratio,
        tup_inserted,
        tup_updated,
        tup_deleted,
        tup_fetched,
        seq_scan,
        idx_scan,
        dead_tuples,
        locks_total,
        hypertable_count,
        chunk_count,
        total_hypertable_bytes,
        total_table_bytes,
        total_index_bytes,
        total_toast_bytes,
        total_rows
    ) VALUES (
        NOW(),
        v_active,
        v_idle,
        v_max,
        v_xact_commit,
        v_xact_rollback,
        v_blks_read,
        v_blks_hit,
        v_buffer_hit_ratio,
        v_tup_inserted,
        v_tup_updated,
        v_tup_deleted,
        v_tup_fetched,
        v_seq_scan,
        v_idx_scan,
        v_dead_tuples,
        v_locks_total,
        v_hypertable_count,
        v_chunk_count,
        v_total_bytes,
        v_table_bytes,
        v_index_bytes,
        v_toast_bytes,
        v_total_rows
    );

    -- Per-table metrics
    INSERT INTO pg_metrics_per_table (
        time, schema_name, table_name, seq_scans, idx_scans,
        n_live_tup, n_dead_tup, table_size_bytes, index_size_bytes
    )
    SELECT
        NOW(),
        schemaname,
        relname,
        seq_scan,
        COALESCE(idx_scan, 0),
        n_live_tup,
        n_dead_tup,
        pg_table_size(schemaname || '.' || relname),
        pg_indexes_size(schemaname || '.' || relname)
    FROM pg_stat_user_tables;
END;
$$ LANGUAGE plpgsql;

-- Schedule the collector to run every 10 seconds
-- Note: add_job doesn't have if_not_exists, so we check manually
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM timescaledb_information.jobs
        WHERE proc_name = 'collect_pg_metrics'
    ) THEN
        PERFORM add_job('collect_pg_metrics', '10 seconds');
    END IF;
END $$;
