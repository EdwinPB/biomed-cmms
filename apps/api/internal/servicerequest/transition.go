package servicerequest

import (
	"errors"
	"fmt"
)

// ErrInvalidTransition is returned when a service request tries to move to a
// status that is not allowed from its current status.
var ErrInvalidTransition = errors.New("service request: invalid transition")

// allowedTransitions lists every legal status move. Terminal states
// (resolved, cancelled) have no outgoing transitions.
var allowedTransitions = map[Status]map[Status]bool{
	StatusPending: {
		StatusAssigned:  true,
		StatusCancelled: true,
	},
	StatusAssigned: {
		StatusInProgress: true,
		StatusCancelled:  true,
	},
	StatusInProgress: {
		StatusResolved:  true,
		StatusCancelled: true,
	},
}

// CanTransition reports whether moving from one status to another is allowed
// by the domain rules.
func CanTransition(from, to Status) bool {
	m := allowedTransitions[from]
	return m != nil && m[to]
}

// TransitionTo moves the request to the next status. It returns
// ErrInvalidTransition (wrapped with the attempted move) when the transition
// is not allowed and leaves the status unchanged.
func (sr *ServiceRequest) TransitionTo(next Status) error {
	if !CanTransition(sr.Status, next) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, sr.Status, next)
	}
	sr.Status = next
	return nil
}
