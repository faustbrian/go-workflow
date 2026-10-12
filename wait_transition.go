package workflow

import (
	"errors"
	"time"
)

var (
	// ErrInvalidWait classifies malformed timer or signal persistence plans.
	ErrInvalidWait = errors.New("invalid workflow wait")
)

// TimerScheduleSpec supplies one atomic durable timer admission plan.
type TimerScheduleSpec struct {
	// TransitionID is the caller-selected store-wide identity for exact replay of
	// this complete plan.
	TransitionID string
	// WorkID identifies the durable work record; callers must avoid conflicting reuse.
	WorkID string
	// InstanceID identifies the durable instance; syntax validation does not authorize access.
	InstanceID string
	// ExpectedSequence is the optimistic history cursor enforced by the committing store.
	ExpectedSequence uint64
	// Definition supplies the exact registered behavior pinned by the instance.
	Definition Definition
	// StepName selects a stable name in the pinned definition.
	StepName string
	// ScheduledAt is the nonzero decision time from which the step timeout derives due work.
	ScheduledAt time.Time
	// Deadline bounds durable work admission; external code must separately honor cancellation.
	Deadline time.Time
	// TenantID is optional bounded routing metadata, not an authentication or isolation claim.
	TenantID string
	// CorrelationID is optional bounded correlation data, not an unbounded metric label.
	CorrelationID string
}

// NewTimerSchedule creates history and due work that must commit atomically.
func NewTimerSchedule(spec TimerScheduleSpec) (Transition, error) {
	step, ok := definitionStep(spec.Definition, spec.StepName, StepTimer)
	if !ok {
		return Transition{}, ErrInvalidWait
	}
	if spec.ScheduledAt.IsZero() {
		return Transition{}, ErrInvalidWait
	}
	scheduledAt := canonicalTime(spec.ScheduledAt)
	dueAt := canonicalTime(scheduledAt.Add(step.Timeout))
	event, err := NewHistoryEvent(HistoryEventSpec{
		Sequence: spec.ExpectedSequence + 1, InstanceID: spec.InstanceID,
		Kind: EventTimerScheduled, OccurredAt: scheduledAt, StepName: spec.StepName, DueAt: dueAt,
	})
	if err != nil {
		return Transition{}, ErrInvalidWait
	}
	work, err := NewPendingWork(PendingWorkSpec{
		ID: spec.WorkID, Kind: WorkTimer, InstanceID: spec.InstanceID, Sequence: event.Sequence(),
		AvailableAt: dueAt, Deadline: spec.Deadline, Payload: []byte(spec.StepName),
		TenantID: spec.TenantID, CorrelationID: spec.CorrelationID,
	})
	if err != nil {
		return Transition{}, ErrInvalidWait
	}
	transition, err := NewTransition(TransitionSpec{
		ID: spec.TransitionID, InstanceID: spec.InstanceID,
		ExpectedSequence: spec.ExpectedSequence, Definition: spec.Definition.Reference(),
		Events: []HistoryEvent{event}, Work: []PendingWork{work},
	})
	if err != nil {
		return Transition{}, ErrInvalidWait
	}
	return transition, nil
}

// TimerFireSpec supplies a due observation from validated timer-work metadata.
// Its transition must commit before the work lease completes; constructors do
// not query the current ownership fence.
type TimerFireSpec struct {
	// TransitionID is the caller-selected store-wide identity for exact replay of
	// this complete plan.
	TransitionID string
	// Lease supplies work and claimed ownership metadata; validation does not query
	// the current store fence.
	Lease WorkLease
	// ExpectedSequence is the optimistic history cursor enforced by the committing store.
	ExpectedSequence uint64
	// Definition supplies the exact registered behavior pinned by the instance.
	Definition Definition
	// FiredAt is no earlier than work availability and strictly before its deadline;
	// no live store fence is queried.
	FiredAt time.Time
}

// NewTimerFire creates the durable timer-fired decision represented by a
// current timer lease.
func NewTimerFire(spec TimerFireSpec) (Transition, error) {
	if !spec.Lease.Valid() || spec.Lease.Work().Kind() != WorkTimer {
		return Transition{}, ErrInvalidWait
	}
	work := spec.Lease.Work()
	stepName := string(work.Payload())
	if _, ok := definitionStep(spec.Definition, stepName, StepTimer); !ok ||
		spec.ExpectedSequence < work.Sequence() {
		return Transition{}, ErrInvalidWait
	}
	firedAt := canonicalTime(spec.FiredAt)
	if firedAt.Before(work.AvailableAt()) || !firedAt.Before(work.Deadline()) {
		return Transition{}, ErrInvalidWait
	}
	event, err := NewHistoryEvent(HistoryEventSpec{
		Sequence: spec.ExpectedSequence + 1, InstanceID: work.InstanceID(),
		Kind: EventTimerFired, OccurredAt: firedAt, StepName: stepName,
	})
	if err != nil {
		return Transition{}, ErrInvalidWait
	}
	transition, err := NewTransition(TransitionSpec{
		ID: spec.TransitionID, InstanceID: work.InstanceID(), ExpectedSequence: spec.ExpectedSequence,
		Definition: spec.Definition.Reference(), Events: []HistoryEvent{event},
	})
	if err != nil {
		return Transition{}, ErrInvalidWait
	}
	return transition, nil
}

// SignalAcceptanceSpec supplies one inbound deduplicated signal transition.
type SignalAcceptanceSpec struct {
	// InstanceID identifies the durable instance; syntax validation does not authorize access.
	InstanceID string
	// ExpectedSequence is the optimistic history cursor enforced by the committing store.
	ExpectedSequence uint64
	// Definition supplies the exact registered behavior pinned by the instance.
	Definition Definition
	// StepName selects a stable name in the pinned definition.
	StepName string
	// SignalID becomes the transition identity as well as the inbound deduplication key.
	SignalID string
	// ReceivedAt supplies the durable acceptance time; transport acknowledgement
	// remains caller-owned.
	ReceivedAt time.Time
	// Payload is opaque input copied into the accepted plan; the caller owns its
	// privacy and interpretation.
	Payload []byte
}

// NewSignalAcceptance creates the transition that must commit before an
// inbound transport acknowledges the signal.
func NewSignalAcceptance(spec SignalAcceptanceSpec) (Transition, error) {
	step, ok := definitionStep(spec.Definition, spec.StepName, StepSignal)
	if !ok || len(spec.Payload) > int(step.InputLimit) {
		return Transition{}, ErrInvalidWait
	}
	event, err := NewHistoryEvent(HistoryEventSpec{
		Sequence: spec.ExpectedSequence + 1, InstanceID: spec.InstanceID,
		Kind: EventSignalReceived, OccurredAt: spec.ReceivedAt, StepName: spec.StepName,
		IdempotencyKey: spec.SignalID, Data: spec.Payload,
	})
	if err != nil {
		return Transition{}, ErrInvalidWait
	}
	transition, err := NewTransition(TransitionSpec{
		ID: spec.SignalID, InstanceID: spec.InstanceID,
		ExpectedSequence: spec.ExpectedSequence, Definition: spec.Definition.Reference(),
		Events: []HistoryEvent{event},
	})
	if err != nil {
		return Transition{}, ErrInvalidWait
	}
	return transition, nil
}
