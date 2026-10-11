package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
)

var (
	ErrParseConfig   = errors.New("parse config")
	ErrInvalidConfig = errors.New("invalid config")
)

type rawConfig struct {
	Port             string  `env:"PORT,required"`
	ID               string  `env:"ID,required"`
	IsAvailable      bool    `env:"IS_AVAILABLE,required"`
	PingDelayMs      int     `env:"PING_DELAY_MS"         envDefault:"0"`
	PongDelayMs      int     `env:"PONG_DELAY_MS"         envDefault:"0"`
	PingFailureRate  float64 `env:"PING_FAILURE_RATE"     envDefault:"0"`
	PongFailureRate  float64 `env:"PONG_FAILURE_RATE"     envDefault:"0"`
	InjectStatusCode int     `env:"INJECT_STATUS_CODE"    envDefault:"500"`
	MaxChainDepth    int     `env:"MAX_CHAIN_DEPTH"       envDefault:"10"`
	HopTimeoutMs     int     `env:"HOP_TIMEOUT_MS"        envDefault:"5000"`
}

type Config struct {
	Port             string
	ID               string
	IsAvailable      bool
	PingDelay        time.Duration
	PongDelay        time.Duration
	PingFailureRate  float64
	PongFailureRate  float64
	InjectStatusCode int
	MaxChainDepth    int
	HopTimeout       time.Duration
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	var raw rawConfig
	if err := env.Parse(&raw); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrParseConfig, err)
	}

	if err := raw.validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	cfg := raw.parsed()

	return &cfg, nil
}

func (c *rawConfig) validate() error {
	for _, check := range []func() error{
		func() error { return checkMin("PING_DELAY_MS", c.PingDelayMs, 0) },
		func() error { return checkMin("PONG_DELAY_MS", c.PongDelayMs, 0) },
		func() error { return checkRate("PING_FAILURE_RATE", c.PingFailureRate) },
		func() error { return checkRate("PONG_FAILURE_RATE", c.PongFailureRate) },
		func() error { return checkRange("INJECT_STATUS_CODE", c.InjectStatusCode, 100, 599) },
		func() error { return checkMin("MAX_CHAIN_DEPTH", c.MaxChainDepth, 1) },
		func() error { return checkMin("HOP_TIMEOUT_MS", c.HopTimeoutMs, 1) },
	} {
		if err := check(); err != nil {
			return err
		}
	}

	return nil
}

func checkMin(name string, got, min int) error {
	if got < min {
		return fmt.Errorf("%s must be >= %d: got %d", name, min, got)
	}

	return nil
}

func checkRate(name string, got float64) error {
	if got < 0 || got > 1 {
		return fmt.Errorf("%s must be in [0,1]: got %f", name, got)
	}

	return nil
}

func checkRange(name string, got, min, max int) error {
	if got < min || got > max {
		return fmt.Errorf("%s must be in [%d,%d]: got %d", name, min, max, got)
	}

	return nil
}

func (c *rawConfig) parsed() Config {
	return Config{
		Port:             c.Port,
		ID:               c.ID,
		IsAvailable:      c.IsAvailable,
		PingDelay:        time.Duration(c.PingDelayMs) * time.Millisecond,
		PongDelay:        time.Duration(c.PongDelayMs) * time.Millisecond,
		PingFailureRate:  c.PingFailureRate,
		PongFailureRate:  c.PongFailureRate,
		InjectStatusCode: c.InjectStatusCode,
		MaxChainDepth:    c.MaxChainDepth,
		HopTimeout:       time.Duration(c.HopTimeoutMs) * time.Millisecond,
	}
}
