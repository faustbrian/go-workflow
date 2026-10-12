// Package workflow provides explicit durable workflow and saga primitives.
//
// Definitions pin names, application versions and SHA-256 fingerprints. Replay
// reconstructs validated history without calling handlers, clocks or migration
// callbacks. Plans returned by constructors are not durable until a store commits
// them; the store owns optimistic sequencing and exact-content idempotency.
//
// Activity, child and compensation processors persist attempt admission before
// external calls. Redelivered running attempts become unknown rather than being
// silently repeated. Business owners must deduplicate external effects and
// reconcile uncertainty; durable work is not exactly-once external execution.
//
// Accepted immutable values own mutable data and expose copying accessors. Registries,
// workers and processors borrow stores, callbacks, clocks and hooks, whose
// concurrency safety and cancellation cooperation remain caller obligations.
// A worker joins processors before returning and cannot forcibly stop trusted
// code. Applications own supervision, transport acknowledgements, authorization,
// retention, encryption and redaction; tenant identifiers are only metadata.
// Nil-interface checks do not detect typed-nil collaborator implementations;
// callers must supply valid concrete dependencies.
//
// See the repository architecture, operations and API reference for transaction
// composition, recovery and version-matched PostgreSQL integration.
package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

const (
	// MaxPayloadBytes bounds persisted workflow inputs and results.
	MaxPayloadBytes = 16 << 20
	// MaxFanOut bounds one definition's parallel admission request.
	MaxFanOut = 10_000
)

var stableName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]{0,127}$`)

var (
	// ErrInvalidDefinition classifies malformed or unsafe definitions.
	ErrInvalidDefinition = errors.New("invalid workflow definition")
	// ErrDuplicateDefinition classifies duplicate immutable definition keys.
	ErrDuplicateDefinition = errors.New("duplicate workflow definition")
	// ErrDefinitionNotFound classifies an unavailable pinned definition.
	ErrDefinitionNotFound = errors.New("workflow definition not found")
	// ErrInvalidMigration classifies incomplete or incoherent migrations.
	ErrInvalidMigration = errors.New("invalid workflow migration")
	// ErrDuplicateMigration classifies duplicate explicit migration edges.
	ErrDuplicateMigration = errors.New("duplicate workflow migration")
	// ErrMigrationNotFound classifies an unavailable explicit migration edge.
	ErrMigrationNotFound = errors.New("workflow migration not found")
)

// ExecutionMode distinguishes central orchestration from explicit external
// choreography. Choreography does not imply or install a global event bus.
type ExecutionMode uint8

const (
	// Orchestration advances through decisions made by one workflow definition.
	Orchestration ExecutionMode = 1
	// Choreography advances through explicitly supplied durable external events.
	Choreography ExecutionMode = 2
)

// StepKind identifies one durable definition step.
type StepKind uint8

const (
	// StepActivity requests an idempotent external activity.
	StepActivity StepKind = 1
	// StepSignal waits for a named external signal.
	StepSignal StepKind = 2
	// StepTimer waits for durable time to become due.
	StepTimer StepKind = 3
	// StepChild starts or observes a version-pinned child workflow.
	StepChild StepKind = 4
	// StepParallel admits a bounded set of branches.
	StepParallel StepKind = 5
	// StepJoin waits for an explicitly modeled branch set.
	StepJoin StepKind = 6
	// StepRace selects the first persisted winning branch.
	StepRace StepKind = 7
	// StepApproval waits for an authorized operator or application decision.
	StepApproval StepKind = 8
)

// RetryPolicy is a bounded retry contract. Its zero value is invalid.
type RetryPolicy struct {
	// MaxAttempts bounds semantic attempts, including the first.
	MaxAttempts uint32
	// InitialDelay is the positive first retry delay; later delays double up to MaxDelay.
	InitialDelay time.Duration
	// MaxDelay caps retry backoff and must be at least InitialDelay.
	MaxDelay time.Duration
}

// CompensationSpec defines an independently retryable compensating activity.
// A failed compensation remains a failed compensation; it is never translated
// into a successful rollback.
type CompensationSpec struct {
	// Target names the explicitly registered compensating activity.
	Target string
	// Timeout is the positive duration of each compensation attempt.
	Timeout time.Duration
	// ResultLimit is a positive outcome byte allowance no greater than MaxPayloadBytes.
	ResultLimit uint32
	// Retry is independent of the forward activity policy.
	Retry RetryPolicy
}

// StepSpec is definition input. NewDefinition validates and deep-copies it.
// Target names an activity, signal, child definition, or approval policy as
// selected by Kind. ChildDefinition is required only for StepChild and pins
// the exact child behavior fingerprint.
type StepSpec struct {
	// Name is the unique bounded ASCII step identity within this definition.
	Name string
	// Kind selects which step-specific constraints apply.
	Kind StepKind
	// Target names the activity, signal or approval target; child steps require
	// it to match ChildDefinition.Name().
	Target string
	// ChildDefinition pins the exact child name, version and fingerprint; it is
	// invalid on other kinds.
	ChildDefinition DefinitionReference
	// Branches is an ordered set of distinct step references for parallel, join or race controls.
	Branches []string
	// Timeout is positive for activity, child, timer, signal and approval kinds;
	// waits do not themselves create a supervisor.
	Timeout time.Duration
	// InputLimit is positive and at most MaxPayloadBytes for activity, child,
	// signal and approval kinds; other kinds do not validate this field.
	InputLimit uint32
	// ResultLimit is positive and at most MaxPayloadBytes for activity and child
	// kinds; other kinds do not validate this field.
	ResultLimit uint32
	// Retry supplies validated activity or child semantic-attempt policy.
	Retry RetryPolicy
	// FanOutLimit bounds this control step from 1 through MaxFanOut.
	FanOutLimit uint32
	// Compensation optionally declares an activity compensation; it does not
	// automatically schedule rollback.
	Compensation *CompensationSpec
}

// DefinitionSpec supplies one stable immutable workflow definition version.
type DefinitionSpec struct {
	// Name is the bounded ASCII application definition identity.
	Name string
	// Version is the caller-assigned immutable behavior version, not the Go module version.
	Version string
	// Mode selects explicit orchestration or choreography.
	Mode ExecutionMode
	// Deprecated marks deployment policy metadata; constructors and replay do not
	// refuse new starts.
	Deprecated bool
	// Steps supplies ordered behavior; NewDefinition owns copies of steps, branches
	// and compensation specs.
	Steps []StepSpec
}

// Definition is one validated immutable workflow behavior version.
// Its zero value is invalid.
type Definition struct {
	spec        DefinitionSpec
	fingerprint string
}

// NewDefinition validates, owns, and fingerprints one immutable definition.
// It copies steps and nested specs before validation; MaxFanOut is an accepted
// value bound, not a bound on allocation before rejection. Fingerprints bind the
// owned Go JSON representation, including order and metadata, not semantic
// equivalence, authentication or an external canonical-JSON standard.
func NewDefinition(spec DefinitionSpec) (Definition, error) {
	owned := cloneDefinitionSpec(spec)
	if err := validateDefinition(owned); err != nil {
		return Definition{}, err
	}

	// DefinitionSpec contains only JSON-total scalar, slice, and struct fields.
	encoded, _ := json.Marshal(owned)
	digest := sha256.Sum256(encoded)

	return Definition{
		spec:        owned,
		fingerprint: hex.EncodeToString(digest[:]),
	}, nil
}

// Name returns the stable definition name.
func (definition Definition) Name() string { return definition.spec.Name }

// Version returns the immutable behavior version.
func (definition Definition) Version() string { return definition.spec.Version }

// Mode returns the definition execution model.
func (definition Definition) Mode() ExecutionMode { return definition.spec.Mode }

// Deprecated reports whether new instance creation should be refused by a
// caller. Existing pinned instances remain resolvable.
func (definition Definition) Deprecated() bool { return definition.spec.Deprecated }

// Steps returns a deep copy of the ordered definition steps.
func (definition Definition) Steps() []StepSpec {
	return cloneSteps(definition.spec.Steps)
}

// Fingerprint returns the deterministic behavior digest used to detect silent
// reinterpretation of an immutable name and version.
func (definition Definition) Fingerprint() string { return definition.fingerprint }

// Reference returns the exact immutable behavior identity that instances must
// persist with their history.
func (definition Definition) Reference() DefinitionReference {
	return DefinitionReference{
		name:        definition.Name(),
		version:     definition.Version(),
		fingerprint: definition.Fingerprint(),
	}
}

func cloneDefinitionSpec(spec DefinitionSpec) DefinitionSpec {
	spec.Steps = cloneSteps(spec.Steps)
	return spec
}

func cloneSteps(steps []StepSpec) []StepSpec {
	if steps == nil {
		return nil
	}
	cloned := make([]StepSpec, len(steps))
	copy(cloned, steps)
	for index := range cloned {
		cloned[index].Branches = append([]string(nil), steps[index].Branches...)
		if steps[index].Compensation != nil {
			compensation := *steps[index].Compensation
			cloned[index].Compensation = &compensation
		}
	}
	return cloned
}

func validateDefinition(spec DefinitionSpec) error {
	if !stableName.MatchString(spec.Name) {
		return invalidDefinition("name")
	}
	if !stableName.MatchString(spec.Version) {
		return invalidDefinition("version")
	}
	if spec.Mode != Orchestration && spec.Mode != Choreography {
		return invalidDefinition("mode")
	}
	if len(spec.Steps) == 0 || len(spec.Steps) > MaxFanOut {
		return invalidDefinition("steps")
	}

	names := make(map[string]StepKind, len(spec.Steps))
	for _, step := range spec.Steps {
		if !stableName.MatchString(step.Name) {
			return invalidDefinition("step.name")
		}
		if _, exists := names[step.Name]; exists {
			return invalidDefinition("step.name")
		}
		names[step.Name] = step.Kind
		if err := validateStep(step); err != nil {
			return err
		}
	}
	for _, step := range spec.Steps {
		for _, branch := range step.Branches {
			kind, exists := names[branch]
			if !exists {
				return invalidDefinition("step.branch")
			}
			if branch == step.Name {
				return invalidDefinition("step.branch")
			}
			switch kind {
			case StepParallel, StepJoin, StepRace:
				return invalidDefinition("step.branch")
			}
		}
	}
	if spec.Mode == Orchestration {
		return validateOrchestrationControlFlow(spec.Steps, names)
	}

	return nil
}

func validateOrchestrationControlFlow(steps []StepSpec, kinds map[string]StepKind) error {
	owners := make(map[string]string)
	branchCounts := make(map[string]int)
	joined := make(map[string]struct{})
	for _, step := range steps {
		switch step.Kind {
		case StepParallel:
			for _, branch := range step.Branches {
				if kinds[branch] != StepActivity {
					return invalidDefinition("step.parallel_branch")
				}
				if _, exists := owners[branch]; exists {
					return invalidDefinition("step.branch_owner")
				}
				owners[branch] = step.Name
			}
			branchCounts[step.Name] = len(step.Branches)
		case StepRace:
			for _, branch := range step.Branches {
				if kinds[branch] != StepSignal && kinds[branch] != StepApproval {
					return invalidDefinition("step.race_branch")
				}
				if _, exists := owners[branch]; exists {
					return invalidDefinition("step.branch_owner")
				}
				owners[branch] = step.Name
			}
		case StepJoin:
			owner, exists := owners[step.Branches[0]]
			if !exists {
				return invalidDefinition("step.join")
			}
			if branchCounts[owner] != len(step.Branches) {
				return invalidDefinition("step.join")
			}
			if _, exists := joined[owner]; exists {
				return invalidDefinition("step.join")
			}
			for _, branch := range step.Branches[1:] {
				if owners[branch] != owner {
					return invalidDefinition("step.join")
				}
			}
			joined[owner] = struct{}{}
		}
	}
	return nil
}

func validateStep(step StepSpec) error {
	switch step.Kind {
	case StepActivity:
		if !stableName.MatchString(step.Target) || step.Timeout <= 0 ||
			!validPayloadLimit(step.InputLimit) || !validPayloadLimit(step.ResultLimit) ||
			!validRetry(step.Retry) {
			return invalidDefinition("step.activity")
		}
	case StepChild:
		if !stableName.MatchString(step.Target) {
			return invalidDefinition("step.child")
		}
		if !step.ChildDefinition.valid() {
			return invalidDefinition("step.child")
		}
		if step.ChildDefinition.Name() != step.Target || step.Timeout <= 0 ||
			!validPayloadLimit(step.InputLimit) || !validPayloadLimit(step.ResultLimit) ||
			!validRetry(step.Retry) {
			return invalidDefinition("step.child")
		}
	case StepSignal, StepApproval:
		if !stableName.MatchString(step.Target) || step.Timeout <= 0 ||
			!validPayloadLimit(step.InputLimit) {
			return invalidDefinition("step.wait")
		}
	case StepTimer:
		if step.Timeout <= 0 {
			return invalidDefinition("step.timer")
		}
	case StepParallel, StepJoin, StepRace:
		if step.FanOutLimit == 0 || step.FanOutLimit > MaxFanOut || len(step.Branches) == 0 ||
			len(step.Branches) > int(step.FanOutLimit) || !validBranchNames(step.Branches) {
			return invalidDefinition("step.fan_out")
		}
	default:
		return invalidDefinition("step.kind")
	}
	if step.Kind != StepParallel && step.Kind != StepJoin && step.Kind != StepRace && len(step.Branches) != 0 {
		return invalidDefinition("step.branches")
	}
	if step.Kind != StepChild && step.ChildDefinition != (DefinitionReference{}) {
		return invalidDefinition("step.child_definition")
	}

	if step.Compensation != nil {
		compensation := step.Compensation
		if step.Kind != StepActivity || !stableName.MatchString(compensation.Target) ||
			compensation.Timeout <= 0 || !validPayloadLimit(compensation.ResultLimit) ||
			!validRetry(compensation.Retry) {
			return invalidDefinition("step.compensation")
		}
	}

	return nil
}

func validBranchNames(branches []string) bool {
	seen := make(map[string]struct{}, len(branches))
	for _, branch := range branches {
		if !stableName.MatchString(branch) {
			return false
		}
		if _, exists := seen[branch]; exists {
			return false
		}
		seen[branch] = struct{}{}
	}
	return true
}

func validPayloadLimit(limit uint32) bool {
	return limit > 0 && limit <= MaxPayloadBytes
}

func validRetry(policy RetryPolicy) bool {
	return policy.MaxAttempts > 0 && policy.InitialDelay > 0 &&
		policy.MaxDelay >= policy.InitialDelay
}

func invalidDefinition(field string) error {
	return fmt.Errorf("%w: %s", ErrInvalidDefinition, field)
}
