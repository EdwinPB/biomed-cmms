package servicerequest

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

var testActor = uuid.MustParse("33333333-3333-3333-3333-333333333333")

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
			if _, err := sr.TransitionTo(tc.to, testActor); err != nil {
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
				_, err := sr.TransitionTo(to, testActor)
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
	_, err := sr.TransitionTo(StatusResolved, testActor)
	if err == nil || err.Error() != "service request: invalid transition: pending -> resolved" {
		t.Errorf("error = %v, want attempted move in message", err)
	}
}

func TestTransitionProducesEvent(t *testing.T) {
	tenantID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	id := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sr := &ServiceRequest{ID: id, TenantID: tenantID, Status: StatusPending}

	event, err := sr.TransitionTo(StatusAssigned, testActor)
	if err != nil {
		t.Fatalf("TransitionTo() error = %v", err)
	}
	if event.TenantID != tenantID {
		t.Errorf("event tenant = %v, want %v", event.TenantID, tenantID)
	}
	if event.RequestID != id {
		t.Errorf("event request id = %v, want %v", event.RequestID, id)
	}
	if event.ActorID != testActor {
		t.Errorf("event actor = %v, want %v", event.ActorID, testActor)
	}
	if event.FromStatus != StatusPending {
		t.Errorf("event from = %q, want %q", event.FromStatus, StatusPending)
	}
	if event.ToStatus != StatusAssigned {
		t.Errorf("event to = %q, want %q", event.ToStatus, StatusAssigned)
	}
}

func TestFailedTransitionProducesNoEvent(t *testing.T) {
	sr := &ServiceRequest{Status: StatusPending}
	event, err := sr.TransitionTo(StatusResolved, testActor)
	if err == nil {
		t.Fatal("TransitionTo() error = nil, want ErrInvalidTransition")
	}
	if event != (RequestEvent{}) {
		t.Errorf("event = %+v, want zero value", event)
	}
}
