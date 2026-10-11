package sim

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/charmingruby/fsm/fsm"
)

var (
	ErrUnavailable = errors.New("failure injected: unavailable")
	ErrInjected    = errors.New("failure injected: failure rate")
)

const (
	stateCheckAvailability fsm.State = "check_availability"
	stateEvaluateFailure   fsm.State = "evaluate_failure"
	stateUnavailable       fsm.State = "unavailable"
	stateInjected          fsm.State = "injected"
	stateDone              fsm.State = "done"
	stateFailed            fsm.State = "failed"
)

type Outcome int

const (
	Proceed Outcome = iota
	Unavailable
	Injected
)

type Input struct {
	Roll        func() float64
	FailureRate float64
	IsAvailable bool
}

type Data struct {
	Reason  string
	Input   Input
	Outcome Outcome
}

func New(opts ...fsm.Option[Data]) *fsm.FSM[Data] {
	f := fsm.New[Data](stateCheckAvailability, opts...)

	f.
		On(stateCheckAvailability, checkAvailability).
		OnFail(stateCheckAvailability, stateUnavailable).
		On(stateUnavailable, markUnavailable).
		On(stateEvaluateFailure, evaluateFailure).
		OnFail(stateEvaluateFailure, stateInjected).
		On(stateInjected, markInjected).
		Terminal(stateDone, stateFailed)

	return f
}

func checkAvailability(_ context.Context, d *Data) (fsm.State, error) {
	if !d.Input.IsAvailable {
		return fsm.EmptyState, ErrUnavailable
	}

	return stateEvaluateFailure, nil
}

func markUnavailable(_ context.Context, d *Data) (fsm.State, error) {
	d.Outcome = Unavailable
	d.Reason = ErrUnavailable.Error()

	return stateFailed, nil
}

func evaluateFailure(_ context.Context, d *Data) (fsm.State, error) {
	rate := d.Input.FailureRate
	if rate <= 0 {
		return stateDone, nil
	}

	roll := rand.Float64()
	if d.Input.Roll != nil {
		roll = d.Input.Roll()
	}

	if roll < rate {
		return fsm.EmptyState, ErrInjected
	}

	return stateDone, nil
}

func markInjected(_ context.Context, d *Data) (fsm.State, error) {
	d.Outcome = Injected
	d.Reason = ErrInjected.Error()

	return stateFailed, nil
}

func Check(ctx context.Context, m *fsm.FSM[Data], delay time.Duration, in Input) (reason string, aborted bool) {
	if !applyDelay(ctx, delay) {
		return "", true
	}

	d := &Data{Input: in}
	if _, err := m.Run(ctx, d); err != nil {
		if ctx.Err() != nil {
			return "", true
		}

		return err.Error(), false
	}

	if d.Outcome != Proceed {
		return d.Reason, false
	}

	return "", false
}

func applyDelay(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
