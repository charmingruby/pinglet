package config

import (
	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
)

type Config struct {
	Port        string `env:"PORT,required"`
	ID          string `env:"ID,required"`
	IsAvailable bool   `env:"IS_AVAILABLE,required"`
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	var cfg Config

	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
