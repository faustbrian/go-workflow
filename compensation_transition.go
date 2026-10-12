package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"time"
)

const (
	// MaxCompensationDispatchBytes bounds internal durable dispatch metadata.
	MaxCompensationDispatchBytes = 1 << 10
)

var (
	// ErrInvalidCompensation classifies malformed compensation persistence or
	// dispatch requests.
	ErrInvalidCompensation = errors.New("invalid workflow compensation")
)

// CompensationDispatch is immutable durable metadata for one semantic
// compensation attempt. Lease redelivery retains the same idempotency key.
type CompensationDispatch struct {
	stepName       string
	attempt        uint32
	idempotencyKey string
}

// StepName returns the compensated activity step.
func (dispatch CompensationDispatch) StepName() string { return dispatch.stepName }

// Attempt returns the one-based semantic compensation attempt.
func (dispatch CompensationDispatch) Attempt() uint32 { return dispatch.attempt }

// IdempotencyKey returns the stable external side-effect identity.
func (dispatch CompensationDispatch) IdempotencyKey() string { return dispatch.idempotencyKey }

type compensationDispatchDocument struct {
	StepName       string `json:"step_name"`
	Attempt        uint32 `json:"attempt"`
	IdempotencyKey string `json:"idempotency_key"`
}

// DecodeCompensationDispatch validates byte-bounded durable dispatch metadata without external
// I/O. It rejects unknown fields and trailing JSON values using encoding/json;
// that decoder still accepts duplicate keys and case-insensitive field matches.
// The result carries identities, not proof of authorization or a current lease.
func DecodeCompensationDispatch(payload []byte) (CompensationDispatch, error) {
	if len(payload) == 0 {
		return CompensationDispatch{}, ErrInvalidCompensation
	}
	if len(payload) > MaxCompensationDispatchBytes {
		return CompensationDispatch{}, ErrInvalidCompensation
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var document compensationDispatchDocument
	if err := decoder.Decode(&document); err != nil {
		return CompensationDispatch{}, ErrInvalidCompensation
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return CompensationDispatch{}, ErrInvalidCompensation
	}
	dispatch := CompensationDispatch{
		stepName: document.StepName, attempt: document.Attempt, idempotencyKey: document.IdempotencyKey,
	}
	if !stableName.MatchString(dispatch.stepName) || dispatch.attempt == 0 ||
		!instanceIDPattern.MatchString(dispatch.idempotencyKey) {
		return CompensationDispatch{}, ErrInvalidCompensation
	}
	return dispatch, nil
}

// CompensationScheduleSpec supplies atomic history and durable dispatch for
// the first attempt of one explicit compensation.
type CompensationScheduleSpec struct {
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
	// Attempt must be 1 for first compensation admission.
	Attempt uint32
	// IdempotencyKey identifies this semantic external attempt, not the lease claim;
	// the effect owner must honor it.
	IdempotencyKey string
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

// NewCompensationSchedule constructs an atomic history-and-work plan for an
// explicit successful activity compensation. It neither commits nor executes it;
// callers select compensation order and commit before admitting external effects.
func NewCompensationSchedule(spec CompensationScheduleSpec) (Transition, error) {
	step, ok := definitionActivityStep(spec.Definition, spec.StepName)
	if !ok {
		return Transition{}, ErrInvalidCompensation
	}
	if step.Compensation == nil {
		return Transition{}, ErrInvalidCompensation
	}
	if spec.Instance.status != StatusRunning && spec.Instance.status != StatusCancelling {
		return Transition{}, ErrInvalidCompensation
	}
	if spec.Definition.Reference() != spec.Instance.definition {
		return Transition{}, ErrInvalidCompensation
	}
	activity, activityExists := spec.Instance.Activity(spec.StepName)
	if !activityExists || activity.status != ActivityProgressSucceeded {
		return Transition{}, ErrInvalidCompensation
	}
	if _, exists := spec.Instance.Compensation(spec.StepName); exists {
		return Transition{}, ErrInvalidCompensation
	}
	if spec.Attempt != 1 {
		return Transition{}, ErrInvalidCompensation
	}
	if len(spec.Input) > int(step.InputLimit) {
		return Transition{}, ErrInvalidCompensation
	}
	if !instanceIDPattern.MatchString(spec.IdempotencyKey) {
		return Transition{}, ErrInvalidCompensation
	}
	if spec.ScheduledAt.Before(spec.Instance.updatedAt) {
		return Transition{}, ErrInvalidCompensation
	}
	document := compensationDispatchDocument{
		StepName: spec.StepName, Attempt: spec.Attempt, IdempotencyKey: spec.IdempotencyKey,
	}
	payload, _ := json.Marshal(document)
	event, err := NewHistoryEvent(HistoryEventSpec{
		Sequence: spec.Instance.sequence + 1, InstanceID: spec.Instance.id,
		Kind: EventCompensationScheduled, OccurredAt: spec.ScheduledAt,
		StepName: spec.StepName, Data: spec.Input,
	})
	if err != nil {
		return Transition{}, ErrInvalidCompensation
	}
	work, err := NewPendingWork(PendingWorkSpec{
		ID: spec.WorkID, Kind: WorkCompensation, InstanceID: spec.Instance.id, Sequence: event.Sequence(),
		AvailableAt: spec.ScheduledAt, Deadline: spec.Deadline, Payload: payload,
		TenantID: spec.TenantID, CorrelationID: spec.CorrelationID,
	})
	if err != nil {
		return Transition{}, ErrInvalidCompensation
	}
	transition, err := NewTransition(TransitionSpec{
		ID: spec.TransitionID, InstanceID: spec.Instance.id, ExpectedSequence: spec.Instance.sequence,
		Definition: spec.Instance.definition, Events: []HistoryEvent{event}, Work: []PendingWork{work},
	})
	if err != nil {
		return Transition{}, ErrInvalidCompensation
	}
	return transition, nil
}

// CompensationAttemptStartSpec supplies the persisted decision required before
// executing one leased compensating side effect.
type CompensationAttemptStartSpec struct {
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

// NewCompensationAttemptStart creates the attempt-start transition that must
// commit before external compensation begins.
func NewCompensationAttemptStart(spec CompensationAttemptStartSpec) (Transition, error) {
	if !spec.Lease.Valid() {
		return Transition{}, ErrInvalidCompensation
	}
	if spec.Lease.Work().Kind() != WorkCompensation {
		return Transition{}, ErrInvalidCompensation
	}
	work := spec.Lease.Work()
	dispatch, err := DecodeCompensationDispatch(work.Payload())
	if err != nil || spec.Instance.sequence < work.Sequence() {
		return Transition{}, ErrInvalidCompensation
	}
	if work.InstanceID() != spec.Instance.id {
		return Transition{}, ErrInvalidCompensation
	}
	step, ok := definitionActivityStep(spec.Definition, dispatch.StepName())
	if !ok {
		return Transition{}, ErrInvalidCompensation
	}
	if step.Compensation == nil {
		return Transition{}, ErrInvalidCompensation
	}
	if spec.Definition.Reference() != spec.Instance.definition {
		return Transition{}, ErrInvalidCompensation
	}
	if spec.Instance.status != StatusRunning && spec.Instance.status != StatusCancelling {
		return Transition{}, ErrInvalidCompensation
	}
	progress, exists := spec.Instance.Compensation(dispatch.StepName())
	if !exists || (progress.status != CompensationReady && progress.status != CompensationRetryWaiting) {
		return Transition{}, ErrInvalidCompensation
	}
	if dispatch.Attempt() != progress.attempt+1 {
		return Transition{}, ErrInvalidCompensation
	}
	if dispatch.Attempt() > step.Compensation.Retry.MaxAttempts {
		return Transition{}, ErrInvalidCompensation
	}
	startedAt := canonicalTime(spec.StartedAt)
	dueAt := canonicalTime(startedAt.Add(step.Compensation.Timeout))
	if startedAt.Before(work.AvailableAt()) {
		return Transition{}, ErrInvalidCompensation
	}
	if !startedAt.Before(work.Deadline()) {
		return Transition{}, ErrInvalidCompensation
	}
	if dueAt.After(work.Deadline()) {
		return Transition{}, ErrInvalidCompensation
	}
	if startedAt.Before(spec.Lease.ClaimedAt()) || !startedAt.Before(spec.Lease.ExpiresAt()) {
		return Transition{}, ErrInvalidCompensation
	}
	if startedAt.Before(spec.Instance.updatedAt) {
		return Transition{}, ErrInvalidCompensation
	}
	if progress.status == CompensationRetryWaiting && startedAt.Before(progress.dueAt) {
		return Transition{}, ErrInvalidCompensation
	}
	event, err := NewHistoryEvent(HistoryEventSpec{
		Sequence: spec.Instance.sequence + 1, InstanceID: work.InstanceID(),
		Kind: EventCompensationAttemptStarted, OccurredAt: startedAt,
		StepName: dispatch.StepName(), Attempt: dispatch.Attempt(),
		IdempotencyKey: dispatch.IdempotencyKey(), DueAt: dueAt,
	})
	if err != nil {
		return Transition{}, ErrInvalidCompensation
	}
	transition, err := NewTransition(TransitionSpec{
		ID: spec.TransitionID, InstanceID: work.InstanceID(), ExpectedSequence: spec.Instance.sequence,
		Definition: spec.Instance.definition, Events: []HistoryEvent{event},
	})
	if err != nil {
		return Transition{}, ErrInvalidCompensation
	}
	return transition, nil
}

// CompensationAttemptOutcomeSpec supplies one known or unknown persisted
// compensation result.
type CompensationAttemptOutcomeSpec struct {
	// TransitionID is the caller-selected store-wide identity for exact replay of
	// this complete plan.
	TransitionID string
	// Instance is the replayed state whose sequence and pinned definition this plan must match.
	Instance Instance
	// Definition supplies the exact registered behavior pinned by the instance.
	Definition Definition
	// StepName selects a stable name in the pinned definition.
	StepName string
	// Attempt is the one-based semantic attempt, distinct from durable lease redelivery.
	Attempt uint32
	// OccurredAt supplies the persisted decision time; constructors remove monotonic
	// clock data and use UTC.
	OccurredAt time.Time
	// Outcome is the validated known or unknown result to persist for this running attempt.
	Outcome ActivityOutcome
}

// NewCompensationAttemptOutcome records an explicit bounded compensation
// outcome without translating failure into successful rollback.
func NewCompensationAttemptOutcome(spec CompensationAttemptOutcomeSpec) (Transition, error) {
	step, ok := definitionActivityStep(spec.Definition, spec.StepName)
	if !ok {
		return Transition{}, ErrInvalidCompensation
	}
	if step.Compensation == nil {
		return Transition{}, ErrInvalidCompensation
	}
	progress, exists := spec.Instance.Compensation(spec.StepName)
	if !exists {
		return Transition{}, ErrInvalidCompensation
	}
	if spec.Definition.Reference() != spec.Instance.definition {
		return Transition{}, ErrInvalidCompensation
	}
	if progress.status != CompensationRunning || progress.attempt != spec.Attempt {
		return Transition{}, ErrInvalidCompensation
	}
	if !spec.Outcome.valid() {
		return Transition{}, ErrInvalidCompensation
	}
	if len(spec.Outcome.data) > int(step.Compensation.ResultLimit) {
		return Transition{}, ErrInvalidCompensation
	}
	if spec.OccurredAt.Before(spec.Instance.updatedAt) {
		return Transition{}, ErrInvalidCompensation
	}
	event, err := NewHistoryEvent(HistoryEventSpec{
		Sequence: spec.Instance.sequence + 1, InstanceID: spec.Instance.id,
		Kind: compensationOutcomeEventKind(spec.Outcome.kind), OccurredAt: spec.OccurredAt,
		StepName: spec.StepName, Attempt: spec.Attempt, Code: spec.Outcome.code,
		Retryable: spec.Outcome.retryable, Data: spec.Outcome.data,
	})
	if err != nil {
		return Transition{}, ErrInvalidCompensation
	}
	transition, err := NewTransition(TransitionSpec{
		ID: spec.TransitionID, InstanceID: spec.Instance.id,
		ExpectedSequence: spec.Instance.sequence, Definition: spec.Instance.definition,
		Events: []HistoryEvent{event},
	})
	if err != nil {
		return Transition{}, ErrInvalidCompensation
	}
	return transition, nil
}

// CompensationRetrySpec supplies one persisted retry decision and the next
// semantic compensation attempt.
type CompensationRetrySpec struct {
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
	// IdempotencyKey identifies this semantic external attempt, not the lease claim;
	// the effect owner must honor it.
	IdempotencyKey string
	// ScheduledAt supplies the persisted decision time, no earlier than the replayed state.
	ScheduledAt time.Time
	// Deadline bounds durable work admission; external code must separately honor cancellation.
	Deadline time.Time
	// TenantID is optional bounded routing metadata, not an authentication or isolation claim.
	TenantID string
	// CorrelationID is optional bounded correlation data, not an unbounded metric label.
	CorrelationID string
}

// NewCompensationRetry atomically records independently retryable
// compensation admission and its durable work.
func NewCompensationRetry(spec CompensationRetrySpec) (Transition, error) {
	step, ok := definitionActivityStep(spec.Definition, spec.StepName)
	if !ok {
		return Transition{}, ErrInvalidCompensation
	}
	if step.Compensation == nil {
		return Transition{}, ErrInvalidCompensation
	}
	progress, exists := spec.Instance.Compensation(spec.StepName)
	if !exists {
		return Transition{}, ErrInvalidCompensation
	}
	if spec.Definition.Reference() != spec.Instance.definition {
		return Transition{}, ErrInvalidCompensation
	}
	if spec.Instance.status != StatusRunning && spec.Instance.status != StatusCancelling {
		return Transition{}, ErrInvalidCompensation
	}
	if progress.status != CompensationFailed || !progress.retryable ||
		progress.attempt >= step.Compensation.Retry.MaxAttempts {
		return Transition{}, ErrInvalidCompensation
	}
	if !instanceIDPattern.MatchString(spec.IdempotencyKey) || spec.ScheduledAt.Before(spec.Instance.updatedAt) {
		return Transition{}, ErrInvalidCompensation
	}
	dueAt := canonicalTime(spec.ScheduledAt.Add(retryDelay(step.Compensation.Retry, progress.attempt)))
	event, err := NewHistoryEvent(HistoryEventSpec{
		Sequence: spec.Instance.sequence + 1, InstanceID: spec.Instance.id,
		Kind: EventCompensationRetryScheduled, OccurredAt: spec.ScheduledAt,
		StepName: spec.StepName, Attempt: progress.attempt, DueAt: dueAt,
	})
	if err != nil {
		return Transition{}, ErrInvalidCompensation
	}
	payload := encodeCompensationDispatch(spec.StepName, progress.attempt+1, spec.IdempotencyKey)
	work, err := NewPendingWork(PendingWorkSpec{
		ID: spec.WorkID, Kind: WorkCompensation, InstanceID: spec.Instance.id,
		Sequence: event.Sequence(), AvailableAt: dueAt, Deadline: spec.Deadline,
		Payload: payload, TenantID: spec.TenantID, CorrelationID: spec.CorrelationID,
	})
	if err != nil {
		return Transition{}, ErrInvalidCompensation
	}
	transition, err := NewTransition(TransitionSpec{
		ID: spec.TransitionID, InstanceID: spec.Instance.id,
		ExpectedSequence: spec.Instance.sequence, Definition: spec.Instance.definition,
		Events: []HistoryEvent{event}, Work: []PendingWork{work},
	})
	if err != nil {
		return Transition{}, ErrInvalidCompensation
	}
	return transition, nil
}

func encodeCompensationDispatch(stepName string, attempt uint32, idempotencyKey string) []byte {
	payload, _ := json.Marshal(compensationDispatchDocument{
		StepName: stepName, Attempt: attempt, IdempotencyKey: idempotencyKey,
	})
	return payload
}

func compensationOutcomeEventKind(kind ActivityOutcomeKind) EventKind {
	switch kind {
	case ActivitySucceeded:
		return EventCompensationAttemptSucceeded
	case ActivityFailed:
		return EventCompensationAttemptFailed
	case ActivityUnknown:
		return EventCompensationAttemptUnknown
	default:
		return 0
	}
}
