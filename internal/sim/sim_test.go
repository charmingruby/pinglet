package sim

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMachine_Validate(t *testing.T) {
	require.NoError(t, New().Validate())
}

func TestMachine_Run(t *testing.T) {
	tests := []struct {
		name        string
		available   bool
		rate        float64
		roll        float64
		wantOutcome Outcome
		wantReason  string
	}{
		{
			name:        "available with zero failure rate proceeds",
			available:   true,
			rate:        0,
			wantOutcome: Proceed,
		},
		{
			name:        "unavailable marks unavailable",
			available:   false,
			rate:        0,
			wantOutcome: Unavailable,
			wantReason:  ErrUnavailable.Error(),
		},
		{
			name:        "roll below rate injects failure",
			available:   true,
			rate:        1,
			roll:        0.5,
			wantOutcome: Injected,
			wantReason:  ErrInjected.Error(),
		},
		{
			name:        "roll above rate proceeds",
			available:   true,
			rate:        0.5,
			roll:        0.9,
			wantOutcome: Proceed,
		},
		{
			name:        "roll equal to rate proceeds",
			available:   true,
			rate:        0.5,
			roll:        0.5,
			wantOutcome: Proceed,
		},
		{
			name:        "unavailable wins over failure rate",
			available:   false,
			rate:        1,
			roll:        0,
			wantOutcome: Unavailable,
			wantReason:  ErrUnavailable.Error(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			machine := New()
			require.NoError(t, machine.Validate())

			data := &Data{
				Input: Input{
					FailureRate: tc.rate,
					IsAvailable: tc.available,
					Roll:        func() float64 { return tc.roll },
				},
			}

			_, err := machine.Run(context.Background(), data)
			require.NoError(t, err)

			assert.Equal(t, tc.wantOutcome, data.Outcome)
			assert.Equal(t, tc.wantReason, data.Reason)
		})
	}
}

func TestApplyDelay(t *testing.T) {
	tests := []struct {
		name      string
		delay     time.Duration
		want      bool
		minElapse time.Duration
	}{
		{
			name:      "zero delay returns immediately",
			delay:     0,
			want:      true,
			minElapse: 0,
		},
		{
			name:      "negative delay returns immediately",
			delay:     -time.Second,
			want:      true,
			minElapse: 0,
		},
		{
			name:      "positive delay actually waits",
			delay:     100 * time.Millisecond,
			want:      true,
			minElapse: 100 * time.Millisecond,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			assert.Equal(t, tc.want, ApplyDelay(context.Background(), tc.delay))
			assert.GreaterOrEqual(t, time.Since(start), tc.minElapse)
		})
	}
}

func TestApplyDelay_CanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	assert.False(t, ApplyDelay(ctx, 10*time.Second))
	assert.Less(t, time.Since(start), 2*time.Second)
}
