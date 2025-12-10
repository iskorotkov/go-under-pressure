package attack

import (
	"fmt"
	"os"
	"time"

	vegeta "github.com/tsenart/vegeta/v12/lib"
)

type Config struct {
	BaseURL         string
	Codes           []string
	Rate            int
	Duration        time.Duration
	CreateRatio     float64
	Type            string
	RateLimitBypass string
}

func Run(cfg *Config) error {
	var targeter vegeta.Targeter

	switch cfg.Type {
	case "create":
		targeter = CreateTargeter(cfg.BaseURL, cfg.RateLimitBypass)
	case "redirect":
		if len(cfg.Codes) == 0 {
			return fmt.Errorf("redirect attack requires seeded codes")
		}
		targeter = RedirectTargeter(cfg.BaseURL, cfg.Codes, cfg.RateLimitBypass)
	case "mixed":
		if len(cfg.Codes) == 0 {
			return fmt.Errorf("mixed attack requires seeded codes")
		}
		targeter = MixedTargeter(cfg.BaseURL, cfg.Codes, cfg.CreateRatio, cfg.RateLimitBypass)
	default:
		return fmt.Errorf("unknown attack type: %s", cfg.Type)
	}

	rate := vegeta.Rate{Freq: cfg.Rate, Per: time.Second}
	attacker := vegeta.NewAttacker(
		vegeta.Redirects(-1),
		vegeta.KeepAlive(true),
		vegeta.Connections(10000),
		vegeta.Timeout(5*time.Second),
		vegeta.MaxBody(0),
		vegeta.HTTP2(false),
	)

	fmt.Printf("Starting %s attack: rate=%d/s duration=%s\n", cfg.Type, cfg.Rate, cfg.Duration)

	var metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, rate, cfg.Duration, cfg.Type) {
		metrics.Add(res)
	}
	metrics.Close()

	reporter := vegeta.NewTextReporter(&metrics)
	return reporter.Report(os.Stdout)
}
