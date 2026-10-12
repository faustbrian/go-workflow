package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"time"
)

const (
	// MaxChildDispatchBytes bounds internal version-pinned child metadata.
	MaxChildDispatchBytes = 1 << 10
)

var (
	// ErrInvalidChildTransition classifies malformed or incoherent child plans.
	ErrInvalidChildTransition = errors.New("invalid workflow child transition")
)

// ChildDispatch is immutable durable metadata for starting one pinned child.
type ChildDispatch struct {
	stepName       string
	childID        string
	definition     DefinitionReference
	attempt        uint32
	idempotencyKey string
}

// StepName returns the parent definition step.
func (dispatch ChildDispatch) StepName() string { return dispatch.stepName }

// ChildID returns the stable child instance identity.
func (dispatch ChildDispatch) ChildID() string { return dispatch.childID }

// Definition returns the exact child behavior identity.
func (dispatch ChildDispatch) Definition() DefinitionReference { return dispatch.definition }

// Attempt returns the one-based semantic child-start attempt.
func (dispatch ChildDispatch) Attempt() uint32 { return dispatch.attempt }

// IdempotencyKey returns the stable key for this semantic attempt.
func (dispatch ChildDispatch) IdempotencyKey() string { return dispatch.idempotencyKey }

type childDispatchDocument struct {
	StepName              string `json:"step_name"`
	ChildID               string `json:"child_id"`
	DefinitionName        string `json:"definition_name"`
	DefinitionVersion     string `json:"definition_version"`
	DefinitionFingerprint string `json:"definition_fingerprint"`
	Attempt               uint32 `json:"attempt"`
	IdempotencyKey        string `json:"idempotency_key"`
}

// DecodeChildDispatch validates byte-bounded durable dispatch metadata without external
// I/O. It rejects unknown fields and trailing JSON values using encoding/json;
// that decoder still accepts duplicate keys and case-insensitive field matches.
// The result carries identities, not proof of authorization or a current lease.
func DecodeChildDispatch(payload []byte) (ChildDispatch, error) {
	if len(payload) > MaxChildDispatchBytes {
		return ChildDispatch{}, ErrInvalidChildTransition
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var document childDispatchDocument
	if err := decoder.Decode(&document); err != nil {
		return ChildDispatch{}, ErrInvalidChildTransition
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ChildDispatch{}, ErrInvalidChildTransition
	}
	reference, err := NewDefinitionReference(document.DefinitionName, document.DefinitionVersion, document.DefinitionFingerprint)
	dispatch := ChildDispatch{
		stepName: document.StepName, childID: document.ChildID, definition: reference,
		attempt: document.Attempt, idempotencyKey: document.IdempotencyKey,
	}
	if err != nil || !stableName.MatchString(dispatch.stepName) || !instanceIDPattern.MatchString(dispatch.childID) ||
		dispatch.attempt == 0 || !instanceIDPattern.MatchString(dispatch.idempotencyKey) {
		return ChildDispatch{}, ErrInvalidChildTransition
	}
	return dispatch, nil
}

// ChildScheduleSpec supplies atomic parent history and durable child work.
type ChildScheduleSpec struct {
	// TransitionID is the caller-selected store-wide identity for exact replay of
	// this complete plan.
	TransitionID string
	// WorkID identifies the durable work record; callers must avoid conflicting reuse.
	WorkID string
	// ChildID is the stable child identity that the caller-owned creator must deduplicate.
	ChildID string
	// Instance is the replayed state whose sequence and pinned definition this plan must match.
	Instance Instance
	// Definition supplies the exact registered behavior pinned by the instance.
	Definition Definition
	// StepName selects a stable name in the pinned definition.
	StepName string
	// ScheduledAt supplies the persisted decision time, no earlier than the replayed state.
	ScheduledAt time.Time
	// Deadline bounds durable work admission; external code must separately honor cancellation.
	Deadline time.Time
	// Input is opaque application data copied into accepted immutable values; no
	// redaction or encryption is applied.
	Input []byte
	// TenantID is optional bounded routing metadata, not an authentication or isolation claim.
	TenantID string
	// CorrelationID is optional bounded correlation data, not an unbounded metric label.
	CorrelationID string
}

// NewChildSchedule records the pinned child identity and bounded input before
// any child-start adapter may act.
func NewChildSchedule(spec ChildScheduleSpec) (Transition, error) {
	step, ok := definitionStep(spec.Definition, spec.StepName, StepChild)
	if !ok || spec.Instance.status != StatusRunning || spec.Definition.Reference() != spec.Instance.definition ||
		len(spec.Input) > int(step.InputLimit) || !instanceIDPattern.MatchString(spec.ChildID) ||
		spec.ScheduledAt.Before(spec.Instance.updatedAt) {
		return Transition{}, ErrInvalidChildTransition
	}
	if _, exists := spec.Instance.Child(spec.StepName); exists {
		return Transition{}, ErrInvalidChildTransition
	}
	event, err := NewHistoryEvent(HistoryEventSpec{
		Sequence: spec.Instance.sequence + 1, InstanceID: spec.Instance.id, Kind: EventChildScheduled,
		OccurredAt: spec.ScheduledAt, Definition: step.ChildDefinition, SuccessorID: spec.ChildID,
		StepName: spec.StepName, Data: spec.Input,
	})
	if err != nil {
		return Transition{}, ErrInvalidChildTransition
	}
	payload := encodeChildDispatch(spec.StepName, spec.ChildID, step.ChildDefinition, 1, spec.ChildID)
	work, err := NewPendingWork(PendingWorkSpec{
		ID: spec.WorkID, Kind: WorkChild, InstanceID: spec.Instance.id, Sequence: event.Sequence(),
		AvailableAt: spec.ScheduledAt, Deadline: spec.Deadline, Payload: payload,
		TenantID: spec.TenantID, CorrelationID: spec.CorrelationID,
	})
	if err != nil {
		return Transition{}, ErrInvalidChildTransition
	}
	transition, err := NewTransition(TransitionSpec{
		ID: spec.TransitionID, InstanceID: spec.Instance.id, ExpectedSequence: spec.Instance.sequence,
		Definition: spec.Instance.definition, Events: []HistoryEvent{event}, Work: []PendingWork{work},
	})
	if err != nil {
		return Transition{}, ErrInvalidChildTransition
	}
	return transition, nil
}

// ChildOutcomeSpec supplies one known terminal child result observed by the
// parent. FailureCode selects failure; an empty code selects success.
type ChildOutcomeSpec struct {
	// TransitionID is the caller-selected store-wide identity for exact replay of
	// this complete plan.
	TransitionID string
	// Instance is the replayed state whose sequence and pinned definition this plan must match.
	Instance Instance
	// Definition supplies the exact registered behavior pinned by the instance.
	Definition Definition
	// StepName selects a stable name in the pinned definition.
	StepName string
	// ChildID is the stable child identity that the caller-owned creator must deduplicate.
	ChildID string
	// CompletedAt is the observation time, no earlier than replayed state.
	CompletedAt time.Time
	// Result is opaque application outcome data copied into accepted immutable
	// values; no redaction is applied.
	Result []byte
	// FailureCode selects known failure when nonempty; an empty code selects success.
	FailureCode string
}

// NewChildOutcome records a known child terminal result before parent
// orchestration advances.
func NewChildOutcome(spec ChildOutcomeSpec) (Transition, error) {
	step, ok := definitionStep(spec.Definition, spec.StepName, StepChild)
	progress, exists := spec.Instance.Child(spec.StepName)
	if !ok || !exists || !childTerminalOutcomeAllowed(progress.Status()) ||
		progress.ChildID() != spec.ChildID ||
		spec.Instance.status != StatusRunning || spec.Definition.Reference() != spec.Instance.definition ||
		len(spec.Result) > int(step.ResultLimit) || spec.CompletedAt.Before(spec.Instance.updatedAt) {
		return Transition{}, ErrInvalidChildTransition
	}
	kind := EventChildCompleted
	if spec.FailureCode != "" {
		if !stableName.MatchString(spec.FailureCode) {
			return Transition{}, ErrInvalidChildTransition
		}
		kind = EventChildFailed
	}
	event, err := NewHistoryEvent(HistoryEventSpec{
		Sequence: spec.Instance.sequence + 1, InstanceID: spec.Instance.id, Kind: kind,
		OccurredAt: spec.CompletedAt, SuccessorID: spec.ChildID, StepName: spec.StepName,
		Code: spec.FailureCode, Data: spec.Result,
	})
	if err != nil {
		return Transition{}, ErrInvalidChildTransition
	}
	transition, err := NewTransition(TransitionSpec{
		ID: spec.TransitionID, InstanceID: spec.Instance.id, ExpectedSequence: spec.Instance.sequence,
		Definition: spec.Instance.definition, Events: []HistoryEvent{event},
	})
	if err != nil {
		return Transition{}, ErrInvalidChildTransition
	}
	return transition, nil
}

func childTerminalOutcomeAllowed(status ChildProgressStatus) bool {
	return status == ChildScheduled || status == ChildStartRunning ||
		status == ChildActive || status == ChildStartUnknownStatus
}

// ChildStartAttemptSpec supplies the durable pre-creation boundary.
type ChildStartAttemptSpec struct {
	// TransitionID is the caller-selected store-wide identity for exact replay of
	// this complete plan.
	TransitionID string
	// Lease supplies work and claimed ownership metadata; validation does not query
	// the current store fence.
	Lease WorkLease
	// Instance is the replayed state whose sequence and pinned definition this plan must match.
	Instance Instance
	// Definition supplies the exact registered behavior pinned by the instance.
	Definition Definition
	// StartedAt supplies the persisted attempt start used to derive its deadline.
	StartedAt time.Time
}

// NewChildStartAttempt constructs pre-creation history from validated lease
// metadata. It neither checks the current database fence nor invokes the creator;
// the caller must commit admission before starting an external effect.
func NewChildStartAttempt(spec ChildStartAttemptSpec) (Transition, error) {
	if !spec.Lease.Valid() {
		return Transition{}, ErrInvalidChildTransition
	}
	work := spec.Lease.Work()
	dispatch, _ := DecodeChildDispatch(work.Payload())
	step, _ := definitionStep(spec.Definition, dispatch.StepName(), StepChild)
	progress, _ := spec.Instance.Child(dispatch.StepName())
	startedAt := canonicalTime(spec.StartedAt)
	dueAt := canonicalTime(startedAt.Add(step.Timeout))
	if work.Kind() != WorkChild ||
		work.InstanceID() != spec.Instance.id || spec.Instance.status != StatusRunning ||
		spec.Definition.Reference() != spec.Instance.definition || spec.Instance.sequence < work.Sequence() ||
		progress.ChildID() != dispatch.ChildID() || progress.Definition() != dispatch.Definition() ||
		(progress.Status() != ChildScheduled && progress.Status() != ChildStartRetryWaiting) ||
		dispatch.Attempt() != progress.Attempt()+1 || dispatch.Attempt() > step.Retry.MaxAttempts ||
		startedAt.Before(work.AvailableAt()) || dueAt.After(work.Deadline()) ||
		startedAt.Before(spec.Lease.ClaimedAt()) ||
		!startedAt.Before(spec.Lease.ExpiresAt()) || startedAt.Before(spec.Instance.updatedAt) ||
		(progress.Status() == ChildStartRetryWaiting && startedAt.Before(progress.DueAt())) {
		return Transition{}, ErrInvalidChildTransition
	}
	event, err := NewHistoryEvent(HistoryEventSpec{
		Sequence: spec.Instance.sequence + 1, InstanceID: spec.Instance.id,
		Kind: EventChildStartAttempted, OccurredAt: startedAt,
		SuccessorID: dispatch.ChildID(), StepName: dispatch.StepName(),
		Attempt: dispatch.Attempt(), IdempotencyKey: dispatch.IdempotencyKey(), DueAt: dueAt,
	})
	if err != nil {
		return Transition{}, ErrInvalidChildTransition
	}
	transition, err := NewTransition(TransitionSpec{
		ID: spec.TransitionID, InstanceID: spec.Instance.id, ExpectedSequence: spec.Instance.sequence,
		Definition: spec.Instance.definition, Events: []HistoryEvent{event},
	})
	if err != nil {
		return Transition{}, ErrInvalidChildTransition
	}
	return transition, nil
}

// ChildStartAttemptOutcomeSpec supplies one explicit creation result.
type ChildStartAttemptOutcomeSpec struct {
	// TransitionID is the caller-selected store-wide identity for exact replay of
	// this complete plan.
	TransitionID string
	// Instance is the replayed state whose sequence and pinned definition this plan must match.
	Instance Instance
	// Definition supplies the exact registered behavior pinned by the instance.
	Definition Definition
	// StepName selects a stable name in the pinned definition.
	StepName string
	// ChildID is the stable child identity that the caller-owned creator must deduplicate.
	ChildID string
	// Attempt is the one-based semantic attempt, distinct from durable lease redelivery.
	Attempt uint32
	// OccurredAt supplies the persisted decision time; constructors remove monotonic
	// clock data and use UTC.
	OccurredAt time.Time
	// Outcome is the validated known or unknown result to persist for this running attempt.
	Outcome ChildStartOutcome
}

// NewChildStartAttemptOutcome persists a known or uncertain creation result.
func NewChildStartAttemptOutcome(spec ChildStartAttemptOutcomeSpec) (Transition, error) {
	progress, _ := spec.Instance.Child(spec.StepName)
	if spec.Instance.status != StatusRunning ||
		spec.Definition.Reference() != spec.Instance.definition ||
		progress.Status() != ChildStartRunning || progress.ChildID() != spec.ChildID ||
		progress.Attempt() != spec.Attempt || !spec.Outcome.valid() ||
		spec.OccurredAt.Before(spec.Instance.updatedAt) {
		return Transition{}, ErrInvalidChildTransition
	}
	kind := EventChildStarted
	switch spec.Outcome.Kind() {
	case ChildStartFailed:
		kind = EventChildStartFailed
	case ChildStartUnknown:
		kind = EventChildStartUnknown
	}
	event, err := NewHistoryEvent(HistoryEventSpec{
		Sequence: spec.Instance.sequence + 1, InstanceID: spec.Instance.id, Kind: kind,
		OccurredAt: spec.OccurredAt, SuccessorID: spec.ChildID, StepName: spec.StepName,
		Attempt: spec.Attempt, Code: spec.Outcome.Code(), Retryable: spec.Outcome.Retryable(),
	})
	if err != nil {
		return Transition{}, ErrInvalidChildTransition
	}
	transition, err := NewTransition(TransitionSpec{
		ID: spec.TransitionID, InstanceID: spec.Instance.id, ExpectedSequence: spec.Instance.sequence,
		Definition: spec.Instance.definition, Events: []HistoryEvent{event},
	})
	if err != nil {
		return Transition{}, ErrInvalidChildTransition
	}
	return transition, nil
}

// ChildStartRetrySpec supplies one deterministic retry decision and due work.
type ChildStartRetrySpec struct {
	// TransitionID is the caller-selected store-wide identity for exact replay of
	// this complete plan.
	TransitionID string
	// WorkID identifies the durable work record; callers must avoid conflicting reuse.
	WorkID string
	// Instance is the replayed state whose sequence and pinned definition this plan must match.
	Instance Instance
	// Definition supplies the exact registered behavior pinned by the instance.
	Definition Definition
	// StepName selects a stable name in the pinned definition.
	StepName string
	// ScheduledAt supplies the persisted decision time, no earlier than the replayed state.
	ScheduledAt time.Time
	// Deadline bounds durable work admission; external code must separately honor cancellation.
	Deadline time.Time
	// TenantID is optional bounded routing metadata, not an authentication or isolation claim.
	TenantID string
	// CorrelationID is optional bounded correlation data, not an unbounded metric label.
	CorrelationID string
}

// NewChildStartRetry atomically records retry admission and its next work.
func NewChildStartRetry(spec ChildStartRetrySpec) (Transition, error) {
	step, _ := definitionStep(spec.Definition, spec.StepName, StepChild)
	progress, _ := spec.Instance.Child(spec.StepName)
	if spec.Instance.status != StatusRunning ||
		spec.Definition.Reference() != spec.Instance.definition ||
		progress.Status() != ChildStartFailedStatus || !progress.Retryable() ||
		progress.Attempt() >= step.Retry.MaxAttempts || spec.ScheduledAt.Before(spec.Instance.updatedAt) {
		return Transition{}, ErrInvalidChildTransition
	}
	dueAt := canonicalTime(spec.ScheduledAt.Add(retryDelay(step.Retry, progress.Attempt())))
	event, err := NewHistoryEvent(HistoryEventSpec{
		Sequence: spec.Instance.sequence + 1, InstanceID: spec.Instance.id,
		Kind: EventChildStartRetryScheduled, OccurredAt: spec.ScheduledAt,
		SuccessorID: progress.ChildID(), StepName: spec.StepName,
		Attempt: progress.Attempt(), DueAt: dueAt,
	})
	if err != nil {
		return Transition{}, ErrInvalidChildTransition
	}
	idempotencyKey := processorTransitionID(spec.WorkID, "child-start-key")
	payload := encodeChildDispatch(
		spec.StepName, progress.ChildID(), progress.Definition(), progress.Attempt()+1, idempotencyKey,
	)
	work, err := NewPendingWork(PendingWorkSpec{
		ID: spec.WorkID, Kind: WorkChild, InstanceID: spec.Instance.id, Sequence: event.Sequence(),
		AvailableAt: dueAt, Deadline: spec.Deadline, Payload: payload,
		TenantID: spec.TenantID, CorrelationID: spec.CorrelationID,
	})
	if err != nil {
		return Transition{}, ErrInvalidChildTransition
	}
	transition, err := NewTransition(TransitionSpec{
		ID: spec.TransitionID, InstanceID: spec.Instance.id, ExpectedSequence: spec.Instance.sequence,
		Definition: spec.Instance.definition, Events: []HistoryEvent{event}, Work: []PendingWork{work},
	})
	if err != nil {
		return Transition{}, ErrInvalidChildTransition
	}
	return transition, nil
}

func encodeChildDispatch(
	stepName, childID string,
	definition DefinitionReference,
	attempt uint32,
	idempotencyKey string,
) []byte {
	payload, _ := json.Marshal(childDispatchDocument{
		StepName: stepName, ChildID: childID, DefinitionName: definition.Name(),
		DefinitionVersion: definition.Version(), DefinitionFingerprint: definition.Fingerprint(),
		Attempt: attempt, IdempotencyKey: idempotencyKey,
	})
	return payload
}
