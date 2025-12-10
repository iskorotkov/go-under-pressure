package metrics

import "time"

type HTTPMetric struct {
	Time       time.Time
	Method     string
	Path       string
	StatusCode int
	DurationMs float64
	ClientIP   string
	Error      string
}

type BusinessMetric struct {
	Time       time.Time
	MetricName string
	Value      float64
	Labels     map[string]string
}

type InfraMetric struct {
	Time          time.Time
	PoolAcquired  int
	PoolIdle      int
	PoolTotal     int
	PoolMax       int
	CacheHits     int64
	CacheMisses   int64
	CacheHitRatio float64
	Goroutines    int
	HeapAllocMB   float64
}
