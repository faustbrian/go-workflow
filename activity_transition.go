package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"time"
)

const (
	// MaxActivityDispatchBytes bounds internal durable dispatch metadata.
	MaxActivityDispatchBytes = 1 << 10
)

var (
	// ErrInvalidActivityTransition classifies malformed or state-incoherent
	// durable activity progression.
	ErrInvalidActivityTransition = errors.New("invalid workflow activity transition")
)

// ActivityDispatch is immutable durable metadata for one semantic attempt.
// Redelivery of one work record retains this identity.
type ActivityDispatch struct {
	stepName       string
	attempt        uint32
	idempotencyKey string
}

// StepName returns the definition activity step.
func (dispatch ActivityDispatch) StepName() string { return dispatch.stepName }

// Attempt returns the one-based semantic activity attempt.
func (dispatch ActivityDispatch) Attempt() uint32 { return dispatch.attempt }

// IdempotencyKey returns the stable external side-effect identity.
func (dispatch ActivityDispatch) IdempotencyKey() string { return dispatch.idempotencyKey }

type activityDispatchDocument struct {
	StepName       string `json:"step_name"`
	Attempt        uint32 `json:"attempt"`
	IdempotencyKey string `json:"idempotency_key"`
}

// DecodeActivityDispatch validates byte-bounded durable dispatch metadata without external
// I/O. It rejects unknown fields and trailing JSON values using encoding/json;
// that decoder still accepts duplicate keys and case-insensitive field matches.
// The result carries identities, not proof of authorization or a current lease.
func DecodeActivityDispatch(payload []byte) (ActivityDispatch, error) {
	if len(payload) == 0 {
		return ActivityDispatch{}, ErrInvalidActivityTransition
	}
	if len(payload) > MaxActivityDispatchBytes {
		return ActivityDispatch{}, ErrInvalidActivityTransition
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var document activityDispatchDocument
	if err := decoder.Decode(&document); err != nil {
		return ActivityDispatch{}, ErrInvalidActivityTransition
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ActivityDispatch{}, ErrInvalidActivityTransition
	}
	dispatch := ActivityDispatch{
		stepName: document.StepName, attempt: document.Attempt, idempotencyKey: document.IdempotencyKey,
	}
	if !stableName.MatchString(dispatch.stepName) {
		return ActivityDispatch{}, ErrInvalidActivityTransition
	}
	if dispatch.attempt == 0 {
		return ActivityDispatch{}, ErrInvalidActivityTransition
	}
	if !instanceIDPattern.MatchString(dispatch.idempotencyKey) {
		return ActivityDispatch{}, ErrInvalidActivityTransition
	}
	return dispatch, nil
}

// ActivityScheduleSpec supplies atomic history and first-attempt due work.
type ActivityScheduleSpec struct {
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
	// Attempt must be 1 for first admission.
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

// NewActivitySchedule records bounded activity input before dispatching work.
func NewActivitySchedule(spec ActivityScheduleSpec) (Transition, error) {
	step, ok := definitionActivityStep(spec.Definition, spec.StepName)
	if !ok {
		return Transition{}, ErrInvalidActivityTransition
	}
	if spec.Attempt != 1 {
		return Transition{}, ErrInvalidActivityTransition
	}
	if len(spec.Input) > int(step.InputLimit) {
		return Transition{}, ErrInvalidActivityTransition
	}
	if !instanceIDPattern.MatchString(spec.IdempotencyKey) {
		return Transition{}, ErrInvalidActivityTransition
	}
	if spec.Instance.status != StatusRunning {
		return Transition{}, ErrInvalidActivityTransition
	}
	if spec.Definition.Reference() != spec.Instance.definition {
		return Transition{}, ErrInvalidActivityTransition
	}
	if _, exists := spec.Instance.Activity(spec.StepName); exists {
		return Transition{}, ErrInvalidActivityTransition
	}
	if spec.ScheduledAt.Before(spec.Instance.updatedAt) {
		return Transition{}, ErrInvalidActivityTransition
	}
	payload := encodeActivityDispatch(spec.StepName, spec.Attempt, spec.IdempotencyKey)
	event, err := NewHistoryEvent(HistoryEventSpec{
		Sequence: spec.Instance.sequence + 1, InstanceID: spec.Instance.id,
		Kind: EventActivityScheduled, OccurredAt: spec.ScheduledAt,
		StepName: spec.StepName, Data: spec.Input,
	})
	if err != nil {
		return Transition{}, ErrInvalidActivityTransition
	}
	work, err := NewPendingWork(PendingWorkSpec{
		ID: spec.WorkID, Kind: WorkActivity, InstanceID: spec.Instance.id, Sequence: event.Sequence(),
		AvailableAt: spec.ScheduledAt, Deadline: spec.Deadline, Payload: payload,
		TenantID: spec.TenantID, CorrelationID: spec.CorrelationID,
	})
	if err != nil {
		return Transition{}, ErrInvalidActivityTransition
	}
	transition, err := NewTransition(TransitionSpec{
		ID: spec.TransitionID, InstanceID: spec.Instance.id, ExpectedSequence: spec.Instance.sequence,
		Definition: spec.Instance.definition, Events: []HistoryEvent{event}, Work: []PendingWork{work},
	})
	if err != nil {
		return Transition{}, ErrInvalidActivityTransition
	}
	return transition, nil
}

// ActivityAttemptStartSpec supplies the persisted decision required before an
// activity side effect begins.
type ActivityAttemptStartSpec struct {
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

// NewActivityAttemptStart creates the attempt-start transition that must
// commit before external activity execution.
func NewActivityAttemptStart(spec ActivityAttemptStartSpec) (Transition, error) {
	if !spec.Lease.Valid() {
		return Transition{}, ErrInvalidActivityTransition
	}
	if spec.Lease.Work().Kind() != WorkActivity {
		return Transition{}, ErrInvalidActivityTransition
	}
	work := spec.Lease.Work()
	dispatch, err := DecodeActivityDispatch(work.Payload())
	if err != nil {
		return Transition{}, ErrInvalidActivityTransition
	}
	if spec.Instance.sequence < work.Sequence() {
		return Transition{}, ErrInvalidActivityTransition
	}
	if work.InstanceID() != spec.Instance.id {
		return Transition{}, ErrInvalidActivityTransition
	}
	step, ok := definitionActivityStep(spec.Definition, dispatch.StepName())
	if !ok {
		return Transition{}, ErrInvalidActivityTransition
	}
	if dispatch.Attempt() > step.Retry.MaxAttempts {
		return Transition{}, ErrInvalidActivityTransition
	}
	progress, exists := spec.Instance.Activity(dispatch.StepName())
	if !exists {
		return Transition{}, ErrInvalidActivityTransition
	}
	if spec.Definition.Reference() != spec.Instance.definition {
		return Transition{}, ErrInvalidActivityTransition
	}
	if spec.Instance.status != StatusRunning {
		return Transition{}, ErrInvalidActivityTransition
	}
	if progress.status != ActivityProgressReady && progress.status != ActivityProgressRetryWaiting {
		return Transition{}, ErrInvalidActivityTransition
	}
	if dispatch.Attempt() != progress.attempt+1 {
		return Transition{}, ErrInvalidActivityTransition
	}
	startedAt := canonicalTime(spec.StartedAt)
	dueAt := canonicalTime(startedAt.Add(step.Timeout))
	if startedAt.Before(work.AvailableAt()) {
		return Transition{}, ErrInvalidActivityTransition
	}
	if !startedAt.Before(work.Deadline()) {
		return Transition{}, ErrInvalidActivityTransition
	}
	if dueAt.After(work.Deadline()) {
		return Transition{}, ErrInvalidActivityTransition
	}
	if startedAt.Before(spec.Lease.ClaimedAt()) {
		return Transition{}, ErrInvalidActivityTransition
	}
	if !startedAt.Before(spec.Lease.ExpiresAt()) {
		return Transition{}, ErrInvalidActivityTransition
	}
	if startedAt.Before(spec.Instance.updatedAt) {
		return Transition{}, ErrInvalidActivityTransition
	}
	if progress.status == ActivityProgressRetryWaiting && startedAt.Before(progress.dueAt) {
		return Transition{}, ErrInvalidActivityTransition
	}
	event, err := NewHistoryEvent(HistoryEventSpec{
		Sequence: spec.Instance.sequence + 1, InstanceID: work.InstanceID(),
		Kind: EventActivityAttemptStarted, OccurredAt: startedAt,
		StepName: dispatch.StepName(), Attempt: dispatch.Attempt(),
		IdempotencyKey: dispatch.IdempotencyKey(), DueAt: dueAt,
	})
	if err != nil {
		return Transition{}, ErrInvalidActivityTransition
	}
	transition, err := NewTransition(TransitionSpec{
		ID: spec.TransitionID, InstanceID: work.InstanceID(), ExpectedSequence: spec.Instance.sequence,
		Definition: spec.Instance.definition, Events: []HistoryEvent{event},
	})
	if err != nil {
		return Transition{}, ErrInvalidActivityTransition
	}
	return transition, nil
}

// ActivityAttemptOutcomeSpec supplies one known or unknown persisted result.
type ActivityAttemptOutcomeSpec struct {
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

// NewActivityAttemptOutcome records an explicit bounded activity outcome.
func NewActivityAttemptOutcome(spec ActivityAttemptOutcomeSpec) (Transition, error) {
	step, ok := definitionActivityStep(spec.Definition, spec.StepName)
	progress, exists := spec.Instance.Activity(spec.StepName)
	if !ok {
		return Transition{}, ErrInvalidActivityTransition
	}
	if !exists {
		return Transition{}, ErrInvalidActivityTransition
	}
	if spec.Definition.Reference() != spec.Instance.definition {
		return Transition{}, ErrInvalidActivityTransition
	}
	if progress.status != ActivityProgressRunning {
		return Transition{}, ErrInvalidActivityTransition
	}
	if progress.attempt != spec.Attempt {
		return Transition{}, ErrInvalidActivityTransition
	}
	if !spec.Outcome.valid() {
		return Transition{}, ErrInvalidActivityTransition
	}
	if len(spec.Outcome.data) > int(step.ResultLimit) {
		return Transition{}, ErrInvalidActivityTransition
	}
	if spec.OccurredAt.Before(spec.Instance.updatedAt) {
		return Transition{}, ErrInvalidActivityTransition
	}
	event, err := NewHistoryEvent(HistoryEventSpec{
		Sequence: spec.Instance.sequence + 1, InstanceID: spec.Instance.id,
		Kind: activityOutcomeEventKind(spec.Outcome.kind), OccurredAt: spec.OccurredAt,
		StepName: spec.StepName, Attempt: spec.Attempt, Code: spec.Outcome.code,
		Retryable: spec.Outcome.retryable, Data: spec.Outcome.data,
	})
	if err != nil {
		return Transition{}, ErrInvalidActivityTransition
	}
	transition, err := NewTransition(TransitionSpec{
		ID: spec.TransitionID, InstanceID: spec.Instance.id,
		ExpectedSequence: spec.Instance.sequence, Definition: spec.Instance.definition,
		Events: []HistoryEvent{event},
	})
	if err != nil {
		return Transition{}, ErrInvalidActivityTransition
	}
	return transition, nil
}

// ActivityRetrySpec supplies one persisted retry decision and next-attempt work.
type ActivityRetrySpec struct {
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

// NewActivityRetry atomically records deterministic retry admission and work.
func NewActivityRetry(spec ActivityRetrySpec) (Transition, error) {
	step, ok := definitionActivityStep(spec.Definition, spec.StepName)
	progress, exists := spec.Instance.Activity(spec.StepName)
	if !ok {
		return Transition{}, ErrInvalidActivityTransition
	}
	if !exists {
		return Transition{}, ErrInvalidActivityTransition
	}
	if spec.Definition.Reference() != spec.Instance.definition {
		return Transition{}, ErrInvalidActivityTransition
	}
	if progress.status != ActivityProgressFailed {
		return Transition{}, ErrInvalidActivityTransition
	}
	if !progress.retryable {
		return Transition{}, ErrInvalidActivityTransition
	}
	if progress.attempt >= step.Retry.MaxAttempts {
		return Transition{}, ErrInvalidActivityTransition
	}
	if !instanceIDPattern.MatchString(spec.IdempotencyKey) {
		return Transition{}, ErrInvalidActivityTransition
	}
	if spec.ScheduledAt.Before(spec.Instance.updatedAt) {
		return Transition{}, ErrInvalidActivityTransition
	}
	dueAt := canonicalTime(spec.ScheduledAt.Add(retryDelay(step.Retry, progress.attempt)))
	event, err := NewHistoryEvent(HistoryEventSpec{
		Sequence: spec.Instance.sequence + 1, InstanceID: spec.Instance.id,
		Kind: EventActivityRetryScheduled, OccurredAt: spec.ScheduledAt,
		StepName: spec.StepName, Attempt: progress.attempt, DueAt: dueAt,
	})
	if err != nil {
		return Transition{}, ErrInvalidActivityTransition
	}
	payload := encodeActivityDispatch(spec.StepName, progress.attempt+1, spec.IdempotencyKey)
	work, err := NewPendingWork(PendingWorkSpec{
		ID: spec.WorkID, Kind: WorkActivity, InstanceID: spec.Instance.id,
		Sequence: event.Sequence(), AvailableAt: dueAt, Deadline: spec.Deadline,
		Payload: payload, TenantID: spec.TenantID, CorrelationID: spec.CorrelationID,
	})
	if err != nil {
		return Transition{}, ErrInvalidActivityTransition
	}
	transition, err := NewTransition(TransitionSpec{
		ID: spec.TransitionID, InstanceID: spec.Instance.id,
		ExpectedSequence: spec.Instance.sequence, Definition: spec.Instance.definition,
		Events: []HistoryEvent{event}, Work: []PendingWork{work},
	})
	if err != nil {
		return Transition{}, ErrInvalidActivityTransition
	}
	return transition, nil
}

func encodeActivityDispatch(stepName string, attempt uint32, idempotencyKey string) []byte {
	payload, _ := json.Marshal(activityDispatchDocument{
		StepName: stepName, Attempt: attempt, IdempotencyKey: idempotencyKey,
	})
	return payload
}

func activityOutcomeEventKind(kind ActivityOutcomeKind) EventKind {
	switch kind {
	case ActivitySucceeded:
		return EventActivityAttemptSucceeded
	case ActivityFailed:
		return EventActivityAttemptFailed
	case ActivityUnknown:
		return EventActivityAttemptUnknown
	default:
		return 0
	}
}
