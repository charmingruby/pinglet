package sim_test

import (
	"testing"

	"github.com/charmingruby/pinglet/internal/pinglet/sim"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMachine_Validate(t *testing.T) {
	require.NoError(t, sim.New().Validate())
}

func TestMachine_Run(t *testing.T) {
	tests := []struct {
		name        string
		wantReason  string
		rate        float64
		roll        float64
		wantOutcome sim.Outcome
		available   bool
	}{
		{
			name:        "available with zero failure rate proceeds",
			available:   true,
			rate:        0,
			wantOutcome: sim.Proceed,
		},
		{
			name:        "unavailable marks unavailable",
			available:   false,
			rate:        0,
			wantOutcome: sim.Unavailable,
			wantReason:  sim.ErrUnavailable.Error(),
		},
		{
			name:        "roll below rate injects failure",
			available:   true,
			rate:        1,
			roll:        0.5,
			wantOutcome: sim.Injected,
			wantReason:  sim.ErrInjected.Error(),
		},
		{
			name:        "roll above rate proceeds",
			available:   true,
			rate:        0.5,
			roll:        0.9,
			wantOutcome: sim.Proceed,
		},
		{
			name:        "roll equal to rate proceeds",
			available:   true,
			rate:        0.5,
			roll:        0.5,
			wantOutcome: sim.Proceed,
		},
		{
			name:        "unavailable wins over failure rate",
			available:   false,
			rate:        1,
			roll:        0,
			wantOutcome: sim.Unavailable,
			wantReason:  sim.ErrUnavailable.Error(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			machine := sim.New()
			require.NoError(t, machine.Validate())

			data := &sim.Data{
				Input: sim.Input{
					FailureRate: tc.rate,
					IsAvailable: tc.available,
					Roll:        func() float64 { return tc.roll },
				},
			}

			_, err := machine.Run(t.Context(), data)
			require.NoError(t, err)

			assert.Equal(t, tc.wantOutcome, data.Outcome)
			assert.Equal(t, tc.wantReason, data.Reason)
		})
	}
}
