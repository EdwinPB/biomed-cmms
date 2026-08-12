package servicerequest

import (
	"errors"
	"testing"
)

func TestAllowedTransitions(t *testing.T) {
	allowed := []struct{ from, to Status }{
		{StatusPending, StatusAssigned},
		{StatusPending, StatusCancelled},
		{StatusAssigned, StatusInProgress},
		{StatusAssigned, StatusCancelled},
		{StatusInProgress, StatusResolved},
		{StatusInProgress, StatusCancelled},
	}

	for _, tc := range allowed {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			if !CanTransition(tc.from, tc.to) {
				t.Fatalf("CanTransition(%q, %q) = false, want true", tc.from, tc.to)
			}
			sr := &ServiceRequest{Status: tc.from}
			if err := sr.TransitionTo(tc.to); err != nil {
				t.Fatalf("TransitionTo(%q) error = %v", tc.to, err)
			}
			if sr.Status != tc.to {
				t.Errorf("status = %q, want %q", sr.Status, tc.to)
			}
		})
	}
}

func TestInvalidTransitionsRejected(t *testing.T) {
	statuses := []Status{
		StatusPending, StatusAssigned, StatusInProgress, StatusResolved, StatusCancelled,
	}

	for _, from := range statuses {
		for _, to := range statuses {
			if CanTransition(from, to) {
				continue
			}
			t.Run(string(from)+"->"+string(to), func(t *testing.T) {
				sr := &ServiceRequest{Status: from}
				err := sr.TransitionTo(to)
				if !errors.Is(err, ErrInvalidTransition) {
					t.Fatalf("TransitionTo(%q) error = %v, want ErrInvalidTransition", to, err)
				}
				if sr.Status != from {
					t.Errorf("status changed to %q, want unchanged %q", sr.Status, from)
				}
			})
		}
	}
}

func TestTerminalStatesHaveNoTransitions(t *testing.T) {
	for _, from := range []Status{StatusResolved, StatusCancelled} {
		for _, to := range []Status{StatusPending, StatusAssigned, StatusInProgress, StatusResolved, StatusCancelled} {
			if CanTransition(from, to) {
				t.Errorf("CanTransition(%q, %q) = true, terminal state should have no transitions", from, to)
			}
		}
	}
}

func TestInvalidTransitionErrorReportsAttemptedMove(t *testing.T) {
	sr := &ServiceRequest{Status: StatusPending}
	err := sr.TransitionTo(StatusResolved)
	if err == nil || err.Error() != "service request: invalid transition: pending -> resolved" {
		t.Errorf("error = %v, want attempted move in message", err)
	}
}
