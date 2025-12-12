# URL Shortener

## Project Structure
- `api/` - Go HTTP API (Echo framework)
- `bench/` - Go benchmarking tool (Vegeta)
- `certinit/` - SSL certificate generation scripts
- `grafana/` - Grafana dashboards and provisioning
- `postgres/` - PostgreSQL init SQL

## Commands
- Build: `docker compose build`
- Run: `docker compose up`
- Benchmark: `docker compose --profile bench up bench`
- Benchmark (optimized): `docker compose -f docker-compose.yml -f docker-compose.bench.yml --profile bench up`
- Lint API: `cd api && golangci-lint run`
- Lint Bench: `cd bench && golangci-lint run`
- Test API: `cd api && go test ./...`
- Test Bench: `cd bench && go test ./...`
- Generate Mocks: `cd api && go generate ./...`

## Environment Variables
See README.md for full list.

## API Endpoints
- `POST /api/v1/urls` - Create short URL
- `POST /api/v1/urls/batch` - Batch create URLs
- `GET /:code` - Redirect to original URL
- `GET /api/v1/health` - Health check
