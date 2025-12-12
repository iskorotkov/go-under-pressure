# URL Shortener

High-performance URL shortening service.

## Quick Start

```bash
docker compose up
```

API available at http://localhost:8080

Grafana available at http://localhost:3000

## API

### Health Check
```
GET /api/v1/health -> {"status": "ok"}
```

### Create Short URL
```
POST /api/v1/urls
{"url": "https://example.com"}
```

Response:
```json
{"short_code": "abc123", "short_url": "http://localhost:8080/abc123", "original_url": "https://example.com"}
```

### Batch Create
```
POST /api/v1/urls/batch
{"urls": ["https://example.com", "https://example.org"]}
```

### Redirect
```
GET /:code -> 302 redirect
```

## Configuration

### API
| Variable | Default | Description |
|----------|---------|-------------|
| SERVER_HOST | localhost | Server bind address |
| API_PORT | 8080 | Server port |
| BASE_URL | http://localhost:8080 | Base URL for short links |
| POSTGRES_HOST | localhost | Database host |
| POSTGRES_PORT | 5432 | Database port |
| POSTGRES_USER | postgres | Database user |
| POSTGRES_PASSWORD | postgres | Database password |
| POSTGRES_DB | urlshortener | Database name |
| POSTGRES_SSLMODE | disable | PostgreSQL SSL mode |
| DB_POOL_MAX_CONNS | 50 | Max database connections |
| DB_POOL_MIN_CONNS | 25 | Min database connections |
| CACHE_MAX_SIZE_POW2 | 27 | Cache size as 2^n (27=128MB) |
| SERVER_MAX_CONNECTIONS | 10000 | Max concurrent connections |
| PPROF_ENABLED | false | Enable pprof profiling |
| PPROF_SECRET | (empty) | Secret for pprof access |

### SSL/TLS
| Variable | Default | Description |
|----------|---------|-------------|
| SSL_ENABLED | false | Enable TLS |
| API_TLS_PORT | 8443 | HTTPS port |
| API_HOST | localhost | API hostname for cert |
| GRAFANA_HOST | localhost | Grafana hostname for cert |

### Grafana
| Variable | Default | Description |
|----------|---------|-------------|
| GRAFANA_USER | admin | Admin username |
| GRAFANA_PASSWORD | admin | Admin password |
| GRAFANA_PORT | 3000 | Grafana port |
| GRAFANA_PROTOCOL | http | http or https |

### Benchmark
| Variable | Default | Description |
|----------|---------|-------------|
| SEED_COUNT | 100000 | URLs to seed before benchmark |
| SEED_BATCH_SIZE | 5000 | Batch size for seeding |
| BENCH_RATE | 30000 | Requests per second |
| BENCH_DURATION | 30s | Benchmark duration |
| BENCH_CREATE_RATIO | 0.1 | Ratio of create vs redirect (0.1 = 10% creates) |
| BENCH_TYPE | mixed | Benchmark type: create, redirect, mixed |
| BENCH_CONNECTIONS | 10000 | Concurrent connections |
| BENCH_MAX_WORKERS | 10000 | Max worker goroutines |

## Benchmarking

```bash
docker compose --profile bench up bench
```

Optimized benchmarking (reduces Docker overhead from ~40% to ~5-10%):
```bash
docker compose -f docker-compose.yml -f docker-compose.bench.yml --profile bench up
```

Custom settings:
```bash
SEED_COUNT=1000000 BENCH_RATE=50000 docker compose --profile bench up bench
```
