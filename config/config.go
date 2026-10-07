package config

import (
	"errors"
	"fmt"

	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
)

var (
	ErrParseConfig             = errors.New("parse config")
	ErrInvalidConfig           = errors.New("invalid config")
	ErrInvalidPingDelayMs      = errors.New("PING_DELAY_MS must be >= 0")
	ErrInvalidPongDelayMs      = errors.New("PONG_DELAY_MS must be >= 0")
	ErrInvalidRequestTimeoutMs = errors.New("REQUEST_TIMEOUT_MS must be > 0")
	ErrInvalidPingFailureRate  = errors.New("PING_FAILURE_RATE must be in [0,1]")
	ErrInvalidPongFailureRate  = errors.New("PONG_FAILURE_RATE must be in [0,1]")
	ErrInvalidInjectStatusCode = errors.New("INJECT_STATUS_CODE must be in [100,599]")
)

type Config struct {
	Port             string  `env:"PORT,required"`
	ID               string  `env:"ID,required"`
	IsAvailable      bool    `env:"IS_AVAILABLE,required"`
	PingDelayMs      int     `env:"PING_DELAY_MS" envDefault:"0"`
	PongDelayMs      int     `env:"PONG_DELAY_MS" envDefault:"0"`
	RequestTimeoutMs int     `env:"REQUEST_TIMEOUT_MS" envDefault:"5000"`
	PingFailureRate  float64 `env:"PING_FAILURE_RATE" envDefault:"0"`
	PongFailureRate  float64 `env:"PONG_FAILURE_RATE" envDefault:"0"`
	InjectStatusCode int     `env:"INJECT_STATUS_CODE" envDefault:"500"`
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	var cfg Config

	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrParseConfig, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.PingDelayMs < 0 {
		return fmt.Errorf("%w: got %d", ErrInvalidPingDelayMs, c.PingDelayMs)
	}

	if c.PongDelayMs < 0 {
		return fmt.Errorf("%w: got %d", ErrInvalidPongDelayMs, c.PongDelayMs)
	}

	if c.RequestTimeoutMs <= 0 {
		return fmt.Errorf("%w: got %d", ErrInvalidRequestTimeoutMs, c.RequestTimeoutMs)
	}

	if c.PingFailureRate < 0 || c.PingFailureRate > 1 {
		return fmt.Errorf("%w: got %f", ErrInvalidPingFailureRate, c.PingFailureRate)
	}

	if c.PongFailureRate < 0 || c.PongFailureRate > 1 {
		return fmt.Errorf("%w: got %f", ErrInvalidPongFailureRate, c.PongFailureRate)
	}

	if c.InjectStatusCode < 100 || c.InjectStatusCode > 599 {
		return fmt.Errorf("%w: got %d", ErrInvalidInjectStatusCode, c.InjectStatusCode)
	}

	return nil
}
