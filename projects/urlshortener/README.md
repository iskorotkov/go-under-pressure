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
GET /health -> {"status": "ok"}
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
| SERVER_PORT | 8080 | Server port |
| POSTGRES_HOST | localhost | Database host |
| POSTGRES_PORT | 5432 | Database port |
| POSTGRES_USER | postgres | Database user |
| POSTGRES_PASSWORD | postgres | Database password |
| POSTGRES_DB | urlshortener | Database name |
| BASE_URL | http://localhost:8080 | Base URL for short links |
| CACHE_MAX_SIZE_POW2 | 0 | Cache size as 2^n bytes (0=disabled, 27=128MB) |

### Benchmark
| Variable | Default | Description |
|----------|---------|-------------|
| SEED_COUNT | 100000 | URLs to seed before benchmark |
| SEED_BATCH_SIZE | 5000 | Batch size for seeding |
| BENCH_RATE | 30000 | Requests per second |
| BENCH_DURATION | 30s | Benchmark duration |
| BENCH_CREATE_RATIO | 0.1 | Ratio of create vs redirect (0.1 = 10% creates) |
| BENCH_TYPE | mixed | Benchmark type: create, redirect, mixed |

## Benchmarking

```bash
docker compose --profile bench up bench
```

Custom settings:
```bash
SEED_COUNT=1000000 BENCH_RATE=50000 docker compose --profile bench up bench
```
