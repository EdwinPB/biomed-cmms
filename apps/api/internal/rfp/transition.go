package rfp

import (
	"errors"
	"fmt"
	"time"
)

// ErrInvalidTransition is returned when an RFP tries to move to a status that
// is not allowed from its current status.
var ErrInvalidTransition = errors.New("rfp: invalid transition")

// Publishing preconditions. These are domain rules, enforced here rather than
// in PostgreSQL or HTTP.
var (
	ErrPublishTitleRequired       = errors.New("rfp: publish requires a title")
	ErrPublishDescriptionRequired = errors.New("rfp: publish requires a description")
	ErrPublishDueAtRequired       = errors.New("rfp: publish requires a due date")
	ErrPublishDueAtInPast         = errors.New("rfp: publish due date must be in the future")
)

// allowedTransitions lists every legal status move. Closed and cancelled are
// terminal states with no outgoing transitions.
var allowedTransitions = map[Status]map[Status]bool{
	StatusDraft: {
		StatusPublished: true,
		StatusCancelled: true,
	},
	StatusPublished: {
		StatusClosed:    true,
		StatusCancelled: true,
	},
}

// CanTransition reports whether moving from one status to another is allowed
// by the domain rules.
func CanTransition(from, to Status) bool {
	m := allowedTransitions[from]
	return m != nil && m[to]
}

// TransitionTo moves the RFP to the next status. It returns
// ErrInvalidTransition (wrapped with the attempted move) when the transition
// is not allowed and leaves the status unchanged. Publishing additionally
// requires a title, a description, and a due date in the future; a publish
// failing these preconditions returns the matching explicit error and leaves
// the status unchanged.
func (r *RFP) TransitionTo(next Status) error {
	from := r.Status
	if !CanTransition(from, next) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, from, next)
	}
	if next == StatusPublished {
		if err := validatePublish(*r); err != nil {
			return err
		}
	}
	r.Status = next
	return nil
}

func validatePublish(r RFP) error {
	var errs []error
	if r.Title == "" {
		errs = append(errs, ErrPublishTitleRequired)
	}
	if r.Description == "" {
		errs = append(errs, ErrPublishDescriptionRequired)
	}
	if r.DueAt == nil {
		errs = append(errs, ErrPublishDueAtRequired)
	} else if !r.DueAt.After(time.Now()) {
		errs = append(errs, ErrPublishDueAtInPast)
	}
	return errors.Join(errs...)
}
