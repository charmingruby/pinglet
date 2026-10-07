package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
)

var (
	ErrParseConfig             = errors.New("parse config")
	ErrInvalidConfig           = errors.New("invalid config")
	ErrInvalidPingDelayMs      = errors.New("PING_DELAY_MS must be >= 0")
	ErrInvalidPongDelayMs      = errors.New("PONG_DELAY_MS must be >= 0")
	ErrInvalidPingFailureRate  = errors.New("PING_FAILURE_RATE must be in [0,1]")
	ErrInvalidPongFailureRate  = errors.New("PONG_FAILURE_RATE must be in [0,1]")
	ErrInvalidInjectStatusCode = errors.New("INJECT_STATUS_CODE must be in [100,599]")
)

type rawConfig struct {
	Port             string  `env:"PORT,required"`
	ID               string  `env:"ID,required"`
	IsAvailable      bool    `env:"IS_AVAILABLE,required"`
	PingDelayMs      int     `env:"PING_DELAY_MS" envDefault:"0"`
	PongDelayMs      int     `env:"PONG_DELAY_MS" envDefault:"0"`
	PingFailureRate  float64 `env:"PING_FAILURE_RATE" envDefault:"0"`
	PongFailureRate  float64 `env:"PONG_FAILURE_RATE" envDefault:"0"`
	InjectStatusCode int     `env:"INJECT_STATUS_CODE" envDefault:"500"`
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

	parsed := raw.parse()

	return &parsed, nil
}

func (c *rawConfig) validate() error {
	if c.PingDelayMs < 0 {
		return fmt.Errorf("%w: got %d", ErrInvalidPingDelayMs, c.PingDelayMs)
	}

	if c.PongDelayMs < 0 {
		return fmt.Errorf("%w: got %d", ErrInvalidPongDelayMs, c.PongDelayMs)
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

func (c *rawConfig) parse() Config {
	return Config{
		Port:             c.Port,
		ID:               c.ID,
		IsAvailable:      c.IsAvailable,
		PingDelay:        time.Duration(c.PingDelayMs) * time.Millisecond,
		PongDelay:        time.Duration(c.PongDelayMs) * time.Millisecond,
		PingFailureRate:  c.PingFailureRate,
		PongFailureRate:  c.PongFailureRate,
		InjectStatusCode: c.InjectStatusCode,
	}
}
