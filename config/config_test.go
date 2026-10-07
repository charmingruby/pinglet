package config

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("PORT", "4000")
	t.Setenv("ID", "node-a")
	t.Setenv("IS_AVAILABLE", "true")
}

func TestLoad_Valid(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		check func(t *testing.T, cfg *Config)
	}{
		{
			name: "defaults when only required vars are set",
			env:  map[string]string{},
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "4000", cfg.Port)
				assert.Equal(t, "node-a", cfg.ID)
				assert.True(t, cfg.IsAvailable)
				assert.Equal(t, time.Duration(0), cfg.PingDelay)
				assert.Equal(t, time.Duration(0), cfg.PongDelay)
				assert.Equal(t, 0.0, cfg.PingFailureRate)
				assert.Equal(t, 0.0, cfg.PongFailureRate)
				assert.Equal(t, 500, cfg.InjectStatusCode)
			},
		},
		{
			name: "delays are parsed from ms to duration",
			env: map[string]string{
				"PING_DELAY_MS": "250",
				"PONG_DELAY_MS": "1500",
			},
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 250*time.Millisecond, cfg.PingDelay)
				assert.Equal(t, 1500*time.Millisecond, cfg.PongDelay)
			},
		},
		{
			name: "failure rates and inject code are passed through",
			env: map[string]string{
				"PING_FAILURE_RATE":  "0.5",
				"PONG_FAILURE_RATE":  "0.25",
				"INJECT_STATUS_CODE": "503",
				"IS_AVAILABLE":       "false",
			},
			check: func(t *testing.T, cfg *Config) {
				assert.Equal(t, 0.5, cfg.PingFailureRate)
				assert.Equal(t, 0.25, cfg.PongFailureRate)
				assert.Equal(t, 503, cfg.InjectStatusCode)
				assert.False(t, cfg.IsAvailable)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setRequiredEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			cfg, err := Load()
			require.NoError(t, err)
			require.NotNil(t, cfg)
			tc.check(t, cfg)
		})
	}
}

func TestLoad_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr error
	}{
		{
			name:    "negative ping delay",
			env:     map[string]string{"PING_DELAY_MS": "-1"},
			wantErr: ErrInvalidPingDelayMs,
		},
		{
			name:    "negative pong delay",
			env:     map[string]string{"PONG_DELAY_MS": "-5"},
			wantErr: ErrInvalidPongDelayMs,
		},
		{
			name:    "ping failure rate above one",
			env:     map[string]string{"PING_FAILURE_RATE": "1.5"},
			wantErr: ErrInvalidPingFailureRate,
		},
		{
			name:    "pong failure rate below zero",
			env:     map[string]string{"PONG_FAILURE_RATE": "-0.1"},
			wantErr: ErrInvalidPongFailureRate,
		},
		{
			name:    "inject status code below 100",
			env:     map[string]string{"INJECT_STATUS_CODE": "99"},
			wantErr: ErrInvalidInjectStatusCode,
		},
		{
			name:    "inject status code above 599",
			env:     map[string]string{"INJECT_STATUS_CODE": "600"},
			wantErr: ErrInvalidInjectStatusCode,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setRequiredEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			cfg, err := Load()
			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.True(t, errors.Is(err, ErrInvalidConfig), "expected ErrInvalidConfig, got %v", err)
			assert.True(t, errors.Is(err, tc.wantErr), "expected %v, got %v", tc.wantErr, err)
		})
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	setRequiredEnv(t)
	require.NoError(t, os.Unsetenv("PORT"))

	cfg, err := Load()
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.True(t, errors.Is(err, ErrParseConfig), "expected ErrParseConfig, got %v", err)
}
