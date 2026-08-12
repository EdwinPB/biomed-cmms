package rfp

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testRFP() *RFP {
	return &RFP{
		ID:               uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		TenantID:         uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		ServiceRequestID: uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		Title:            "MRI replacement",
		Description:      "Procure a replacement MRI scanner.",
		Status:           StatusDraft,
		DueAt:            futureTime(),
		CreatedBy:        uuid.MustParse("44444444-4444-4444-4444-444444444444"),
	}
}

func futureTime() *time.Time {
	t := time.Now().Add(24 * time.Hour)
	return &t
}

func pastTime() *time.Time {
	t := time.Now().Add(-24 * time.Hour)
	return &t
}

func TestAllowedTransitions(t *testing.T) {
	allowed := []struct{ from, to Status }{
		{StatusDraft, StatusPublished},
		{StatusDraft, StatusCancelled},
		{StatusPublished, StatusClosed},
		{StatusPublished, StatusCancelled},
	}

	for _, tc := range allowed {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			if !CanTransition(tc.from, tc.to) {
				t.Fatalf("CanTransition(%q, %q) = false, want true", tc.from, tc.to)
			}
			r := testRFP()
			r.Status = tc.from
			if err := r.TransitionTo(tc.to); err != nil {
				t.Fatalf("TransitionTo(%q) error = %v", tc.to, err)
			}
			if r.Status != tc.to {
				t.Errorf("status = %q, want %q", r.Status, tc.to)
			}
		})
	}
}

func TestInvalidTransitionsRejected(t *testing.T) {
	tests := []struct{ from, to Status }{
		{StatusDraft, StatusClosed},
		{StatusPublished, StatusDraft},
		{StatusClosed, StatusDraft},
		{StatusClosed, StatusPublished},
		{StatusClosed, StatusCancelled},
		{StatusCancelled, StatusDraft},
		{StatusCancelled, StatusPublished},
		{StatusCancelled, StatusClosed},
		{StatusCancelled, StatusCancelled},
		{StatusDraft, StatusDraft},
		{StatusPublished, StatusPublished},
		{StatusClosed, StatusClosed},
	}

	for _, tc := range tests {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			r := testRFP()
			r.Status = tc.from
			err := r.TransitionTo(tc.to)
			if !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("TransitionTo(%q) error = %v, want ErrInvalidTransition", tc.to, err)
			}
			if r.Status != tc.from {
				t.Errorf("status changed to %q, want unchanged %q", r.Status, tc.from)
			}
		})
	}
}

func TestTerminalStatesHaveNoTransitions(t *testing.T) {
	for _, from := range []Status{StatusClosed, StatusCancelled} {
		for _, to := range []Status{StatusDraft, StatusPublished, StatusClosed, StatusCancelled} {
			if CanTransition(from, to) {
				t.Errorf("CanTransition(%q, %q) = true, terminal state should have no transitions", from, to)
			}
		}
	}
}

func TestInvalidTransitionErrorReportsAttemptedMove(t *testing.T) {
	r := testRFP()
	r.Status = StatusDraft
	err := r.TransitionTo(StatusClosed)
	if err == nil || err.Error() != "rfp: invalid transition: draft -> closed" {
		t.Errorf("error = %v, want attempted move in message", err)
	}
}

func TestPublishMissingDueAtRejected(t *testing.T) {
	r := testRFP()
	r.DueAt = nil
	if err := r.TransitionTo(StatusPublished); !errors.Is(err, ErrPublishDueAtRequired) {
		t.Errorf("error = %v, want ErrPublishDueAtRequired", err)
	}
	if r.Status != StatusDraft {
		t.Errorf("status = %q, want unchanged draft", r.Status)
	}
}

func TestPublishPastDueAtRejected(t *testing.T) {
	r := testRFP()
	r.DueAt = pastTime()
	if err := r.TransitionTo(StatusPublished); !errors.Is(err, ErrPublishDueAtInPast) {
		t.Errorf("error = %v, want ErrPublishDueAtInPast", err)
	}
	if r.Status != StatusDraft {
		t.Errorf("status = %q, want unchanged draft", r.Status)
	}
}

func TestPublishEmptyTitleRejected(t *testing.T) {
	r := testRFP()
	r.Title = ""
	if err := r.TransitionTo(StatusPublished); !errors.Is(err, ErrPublishTitleRequired) {
		t.Errorf("error = %v, want ErrPublishTitleRequired", err)
	}
	if r.Status != StatusDraft {
		t.Errorf("status = %q, want unchanged draft", r.Status)
	}
}

func TestPublishEmptyDescriptionRejected(t *testing.T) {
	r := testRFP()
	r.Description = ""
	if err := r.TransitionTo(StatusPublished); !errors.Is(err, ErrPublishDescriptionRequired) {
		t.Errorf("error = %v, want ErrPublishDescriptionRequired", err)
	}
	if r.Status != StatusDraft {
		t.Errorf("status = %q, want unchanged draft", r.Status)
	}
}

func TestPublishMultipleMissingFieldsJoined(t *testing.T) {
	r := testRFP()
	r.Title = ""
	r.Description = ""
	r.DueAt = nil
	err := r.TransitionTo(StatusPublished)
	if err == nil {
		t.Fatal("TransitionTo() error = nil, want joined publish errors")
	}
	for _, want := range []error{ErrPublishTitleRequired, ErrPublishDescriptionRequired, ErrPublishDueAtRequired} {
		if !errors.Is(err, want) {
			t.Errorf("error = %v, want %v in join", err, want)
		}
	}
	if r.Status != StatusDraft {
		t.Errorf("status = %q, want unchanged draft", r.Status)
	}
}

func TestValidPublishSucceeds(t *testing.T) {
	r := testRFP()
	if err := r.TransitionTo(StatusPublished); err != nil {
		t.Fatalf("TransitionTo(published) error = %v", err)
	}
	if r.Status != StatusPublished {
		t.Errorf("status = %q, want published", r.Status)
	}
}

func TestNonPublishTransitionsDoNotRequireDueAt(t *testing.T) {
	tests := []struct{ from, to Status }{
		{StatusDraft, StatusCancelled},
		{StatusPublished, StatusClosed},
		{StatusPublished, StatusCancelled},
	}
	for _, tc := range tests {
		t.Run(string(tc.from)+"->"+string(tc.to), func(t *testing.T) {
			r := testRFP()
			r.Status = tc.from
			r.DueAt = nil
			if err := r.TransitionTo(tc.to); err != nil {
				t.Fatalf("TransitionTo(%q) error = %v, want success without due_at", tc.to, err)
			}
			if r.Status != tc.to {
				t.Errorf("status = %q, want %q", r.Status, tc.to)
			}
		})
	}
}
