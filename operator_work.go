package workflow

import (
	"encoding/json"
	"time"
)

// OperatorActivityRetrySpec supplies one caller-authorized audited activity
// retry command. The command records audit history and retry due work in one
// optimistic transition.
type OperatorActivityRetrySpec struct {
	// CommandID is the caller-selected store-wide identity binding audit data and
	// the complete command.
	CommandID string
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
	// Actor is a bounded principal identifier already authenticated and authorized by the caller.
	Actor string
	// Reason is a bounded stable audit code; it must not contain confidential
	// free-form diagnostics.
	Reason string
	// OccurredAt supplies the persisted decision time; constructors remove monotonic
	// clock data and use UTC.
	OccurredAt time.Time
	// Deadline bounds durable work admission; external code must separately honor cancellation.
	Deadline time.Time
	// TenantID is optional bounded routing metadata, not an authentication or isolation claim.
	TenantID string
	// CorrelationID is optional bounded correlation data, not an unbounded metric label.
	CorrelationID string
}

// NewOperatorActivityRetry atomically records caller-supplied authorization
// audit data before admitting a definition-policy activity retry.
func NewOperatorActivityRetry(spec OperatorActivityRetrySpec) (Transition, error) {
	audit, commandInstance, err := newOperatorWorkAudit(
		spec.CommandID, spec.Instance, OperatorRetryActivity, spec.Actor, spec.Reason, spec.OccurredAt,
	)
	if err != nil {
		return Transition{}, ErrInvalidOperatorCommand
	}
	retry, err := NewActivityRetry(ActivityRetrySpec{
		TransitionID: spec.CommandID, WorkID: spec.WorkID, Instance: commandInstance,
		Definition: spec.Definition, StepName: spec.StepName, IdempotencyKey: spec.IdempotencyKey,
		ScheduledAt: spec.OccurredAt, Deadline: spec.Deadline,
		TenantID: spec.TenantID, CorrelationID: spec.CorrelationID,
	})
	if err != nil {
		return Transition{}, ErrInvalidOperatorCommand
	}
	return newOperatorWorkTransition(spec.CommandID, spec.Instance, audit, retry)
}

// OperatorCompensationSpec supplies one caller-authorized audited explicit
// compensation command.
type OperatorCompensationSpec struct {
	// CommandID is the caller-selected store-wide identity binding audit data and
	// the complete command.
	CommandID string
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
	// Actor is a bounded principal identifier already authenticated and authorized by the caller.
	Actor string
	// Reason is a bounded stable audit code; it must not contain confidential
	// free-form diagnostics.
	Reason string
	// OccurredAt supplies the persisted decision time; constructors remove monotonic
	// clock data and use UTC.
	OccurredAt time.Time
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

// NewOperatorCompensation atomically records audit data before scheduling the
// requested compensation and its first durable work item.
func NewOperatorCompensation(spec OperatorCompensationSpec) (Transition, error) {
	audit, commandInstance, err := newOperatorWorkAudit(
		spec.CommandID, spec.Instance, OperatorCompensate, spec.Actor, spec.Reason, spec.OccurredAt,
	)
	if err != nil {
		return Transition{}, ErrInvalidOperatorCommand
	}
	compensation, err := NewCompensationSchedule(CompensationScheduleSpec{
		TransitionID: spec.CommandID, WorkID: spec.WorkID, Instance: commandInstance,
		Definition: spec.Definition, StepName: spec.StepName, Attempt: spec.Attempt,
		IdempotencyKey: spec.IdempotencyKey, ScheduledAt: spec.OccurredAt,
		Deadline: spec.Deadline, Input: spec.Input, TenantID: spec.TenantID,
		CorrelationID: spec.CorrelationID,
	})
	if err != nil {
		return Transition{}, ErrInvalidOperatorCommand
	}
	return newOperatorWorkTransition(spec.CommandID, spec.Instance, audit, compensation)
}

// OperatorCompensationResolutionSpec supplies caller-authorized audit data and
// bounded manual reconciliation evidence for one failed or unknown
// compensation.
type OperatorCompensationResolutionSpec struct {
	// CommandID is the caller-selected store-wide identity binding audit data and
	// the complete command.
	CommandID string
	// Instance is the replayed state whose sequence and pinned definition this plan must match.
	Instance Instance
	// Definition supplies the exact registered behavior pinned by the instance.
	Definition Definition
	// StepName selects a stable name in the pinned definition.
	StepName string
	// Actor is a bounded principal identifier already authenticated and authorized by the caller.
	Actor string
	// Reason is a bounded stable audit code; it must not contain confidential
	// free-form diagnostics.
	Reason string
	// Code is a bounded stable classification, not automatically redacted application data.
	Code string
	// Evidence is copied opaque manual-reconciliation data bounded by the
	// compensation ResultLimit; it is not redacted.
	Evidence []byte
	// OccurredAt supplies the persisted decision time; constructors remove monotonic
	// clock data and use UTC.
	OccurredAt time.Time
}

// NewOperatorCompensationResolution records manual reconciliation as its own
// durable outcome. It never emits CompensationSucceeded.
func NewOperatorCompensationResolution(spec OperatorCompensationResolutionSpec) (Transition, error) {
	audit, commandInstance, err := newOperatorWorkAudit(
		spec.CommandID, spec.Instance, OperatorResolveCompensation,
		spec.Actor, spec.Reason, spec.OccurredAt,
	)
	if err != nil {
		return Transition{}, ErrInvalidOperatorCommand
	}
	if spec.Definition.Reference() != spec.Instance.definition {
		return Transition{}, ErrInvalidOperatorCommand
	}
	step, ok := definitionActivityStep(spec.Definition, spec.StepName)
	if !ok || step.Compensation == nil {
		return Transition{}, ErrInvalidOperatorCommand
	}
	progress, exists := spec.Instance.Compensation(spec.StepName)
	if !exists {
		return Transition{}, ErrInvalidOperatorCommand
	}
	if progress.Status() != CompensationFailed && progress.Status() != CompensationUnknown {
		return Transition{}, ErrInvalidOperatorCommand
	}
	if !stableName.MatchString(spec.Code) || len(spec.Evidence) > int(step.Compensation.ResultLimit) {
		return Transition{}, ErrInvalidOperatorCommand
	}
	resolved, _ := NewHistoryEvent(HistoryEventSpec{
		Sequence: commandInstance.sequence + 1, InstanceID: commandInstance.id,
		Kind: EventCompensationManuallyResolved, OccurredAt: spec.OccurredAt,
		StepName: spec.StepName, Code: spec.Code, Data: spec.Evidence,
	})
	action, _ := NewTransition(TransitionSpec{
		ID: spec.CommandID, InstanceID: commandInstance.id,
		ExpectedSequence: commandInstance.sequence, Definition: commandInstance.definition,
		Events: []HistoryEvent{resolved},
	})
	return newOperatorWorkTransition(spec.CommandID, spec.Instance, audit, action)
}

// OperatorApprovalSpec supplies caller-authorized audit identity and bounded
// approval evidence for one human-approval step.
type OperatorApprovalSpec struct {
	// CommandID is the caller-selected store-wide identity binding audit data and
	// the complete command.
	CommandID string
	// Instance is the replayed state whose sequence and pinned definition this plan must match.
	Instance Instance
	// Definition supplies the exact registered behavior pinned by the instance.
	Definition Definition
	// StepName selects a stable name in the pinned definition.
	StepName string
	// Actor is a bounded principal identifier already authenticated and authorized by the caller.
	Actor string
	// Reason is a bounded stable audit code; it must not contain confidential
	// free-form diagnostics.
	Reason string
	// OccurredAt supplies the persisted decision time; constructors remove monotonic
	// clock data and use UTC.
	OccurredAt time.Time
	// Payload is opaque input copied into the accepted plan; the caller owns its
	// privacy and interpretation.
	Payload []byte
}

// NewOperatorApproval atomically records audit identity before accepting one
// approval decision. Replay treats the approval as persisted step progress.
func NewOperatorApproval(spec OperatorApprovalSpec) (Transition, error) {
	audit, commandInstance, err := newOperatorWorkAudit(
		spec.CommandID, spec.Instance, OperatorApprove, spec.Actor, spec.Reason, spec.OccurredAt,
	)
	if err != nil {
		return Transition{}, ErrInvalidOperatorCommand
	}
	if spec.Definition.Reference() != spec.Instance.definition {
		return Transition{}, ErrInvalidOperatorCommand
	}
	step, ok := definitionStep(spec.Definition, spec.StepName, StepApproval)
	if !ok || len(spec.Payload) > int(step.InputLimit) {
		return Transition{}, ErrInvalidOperatorCommand
	}
	accepted, _ := NewHistoryEvent(HistoryEventSpec{
		Sequence: commandInstance.sequence + 1, InstanceID: commandInstance.id,
		Kind: EventSignalReceived, OccurredAt: spec.OccurredAt, StepName: spec.StepName,
		IdempotencyKey: spec.CommandID, Data: spec.Payload,
	})
	action, _ := NewTransition(TransitionSpec{
		ID: spec.CommandID, InstanceID: commandInstance.id,
		ExpectedSequence: commandInstance.sequence, Definition: commandInstance.definition,
		Events: []HistoryEvent{accepted},
	})
	return newOperatorWorkTransition(spec.CommandID, spec.Instance, audit, action)
}

func newOperatorWorkAudit(
	commandID string,
	instance Instance,
	action OperatorAction,
	actor string,
	reason string,
	occurredAt time.Time,
) (HistoryEvent, Instance, error) {
	if !instanceIDPattern.MatchString(commandID) {
		return HistoryEvent{}, Instance{}, ErrInvalidOperatorCommand
	}
	if occurredAt.IsZero() || occurredAt.Before(instance.updatedAt) {
		return HistoryEvent{}, Instance{}, ErrInvalidOperatorCommand
	}
	if instance.sequence > ^uint64(0)-2 {
		return HistoryEvent{}, Instance{}, ErrInvalidOperatorCommand
	}
	if !instanceIDPattern.MatchString(instance.id) || !instance.definition.valid() {
		return HistoryEvent{}, Instance{}, ErrInvalidOperatorCommand
	}
	document := operatorAuditDocument{Action: action, Actor: actor, Reason: reason}
	auditData, _ := json.Marshal(document)
	if _, err := decodeOperatorAudit(auditData); err != nil {
		return HistoryEvent{}, Instance{}, ErrInvalidOperatorCommand
	}
	occurredAt = canonicalTime(occurredAt)
	audit, _ := NewHistoryEvent(HistoryEventSpec{
		Sequence: instance.sequence + 1, InstanceID: instance.id,
		Kind: EventOperatorCommandRecorded, OccurredAt: occurredAt,
		IdempotencyKey: commandID, Data: auditData,
	})
	commandInstance := instance
	commandInstance.sequence++
	commandInstance.updatedAt = occurredAt
	return audit, commandInstance, nil
}

func newOperatorWorkTransition(
	commandID string,
	instance Instance,
	audit HistoryEvent,
	action Transition,
) (Transition, error) {
	events := append([]HistoryEvent{audit}, action.Events()...)
	transition, err := NewTransition(TransitionSpec{
		ID: commandID, InstanceID: instance.id, ExpectedSequence: instance.sequence,
		Definition: instance.definition, Events: events, Work: action.Work(),
	})
	if err != nil {
		return Transition{}, ErrInvalidOperatorCommand
	}
	return transition, nil
}
