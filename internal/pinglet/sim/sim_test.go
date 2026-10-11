package sim

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMachine_Validate(t *testing.T) {
	require.NoError(t, New().Validate())
}

func TestMachine_Run(t *testing.T) {
	tests := []struct {
		name        string
		wantReason  string
		rate        float64
		roll        float64
		wantOutcome Outcome
		available   bool
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
