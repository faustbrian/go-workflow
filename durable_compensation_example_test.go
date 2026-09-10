package workflow_test

import (
	"context"
	"fmt"
	"time"

	workflow "github.com/faustbrian/go-workflow"
)

// Example_durableCompensationRecovery is a non-releasable reference recipe.
// The in-memory recipe store stands in for an application-owned durable store;
// a service uses the PostgreSQL adapter or another TransitionStore with the
// same atomic history-and-work contract.
func Example_durableCompensationRecovery() {
	now := time.Now().UTC()
	definition := mustRecipe(workflow.NewDefinition(workflow.DefinitionSpec{
		Name: "orders", Version: "compensation-v1", Mode: workflow.Orchestration,
		Steps: []workflow.StepSpec{
			{
				Name: "reserve", Kind: workflow.StepActivity, Target: "inventory.reserve",
				Timeout: time.Minute, InputLimit: 64, ResultLimit: 64,
				Retry: workflow.RetryPolicy{
					MaxAttempts: 1, InitialDelay: time.Second, MaxDelay: time.Second,
				},
				Compensation: &workflow.CompensationSpec{
					Target: "inventory.release", Timeout: time.Minute, ResultLimit: 64,
					Retry: workflow.RetryPolicy{
						MaxAttempts: 1, InitialDelay: time.Second, MaxDelay: time.Second,
					},
				},
			},
			{
				Name: "charge", Kind: workflow.StepActivity, Target: "payments.charge",
				Timeout: time.Minute, InputLimit: 64, ResultLimit: 64,
				Retry: workflow.RetryPolicy{
					MaxAttempts: 1, InitialDelay: time.Second, MaxDelay: time.Second,
				},
			},
		},
	}))
	definitions := mustRecipe(workflow.CompileDefinitions(definition))

	reserve := mustRecipe(workflow.NewActivity(
		"inventory.reserve",
		func(context.Context, workflow.ActivityRequest) workflow.ActivityOutcome {
			return mustRecipe(workflow.NewActivityOutcome(workflow.ActivityOutcomeSpec{
				Kind: workflow.ActivitySucceeded, Data: []byte("reservation-42"),
			}))
		},
	))
	charge := mustRecipe(workflow.NewActivity(
		"payments.charge",
		func(context.Context, workflow.ActivityRequest) workflow.ActivityOutcome {
			return mustRecipe(workflow.NewActivityOutcome(workflow.ActivityOutcomeSpec{
				Kind: workflow.ActivityFailed, Code: "card-declined",
			}))
		},
	))
	release := mustRecipe(workflow.NewActivity(
		"inventory.release",
		func(context.Context, workflow.ActivityRequest) workflow.ActivityOutcome {
			return mustRecipe(workflow.NewActivityOutcome(workflow.ActivityOutcomeSpec{
				Kind: workflow.ActivityUnknown, Code: "release-commit-unknown",
			}))
		},
	))
	activities := mustRecipe(workflow.CompileActivities(reserve, charge))
	compensations := mustRecipe(workflow.CompileActivities(release))

	// The process supervisor owns this context. In a service it cancels worker
	// admission, waits for Worker.Run to return, and only then closes the store.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := newRecipeStore(definitions)
	started := mustRecipe(workflow.NewHistoryEvent(workflow.HistoryEventSpec{
		Sequence: 1, InstanceID: "order-42", Kind: workflow.EventInstanceStarted,
		OccurredAt: now, Definition: definition.Reference(),
	}))
	mustRecipeOK(store.Commit(ctx, mustRecipe(workflow.NewTransition(workflow.TransitionSpec{
		ID: "start-order-42", InstanceID: "order-42", Definition: definition.Reference(),
		Events: []workflow.HistoryEvent{started},
	}))))

	reserveDecision := mustRecipe(workflow.NewOrchestrationDecision(workflow.OrchestrationDecisionSpec{
		TransitionID: "schedule-reserve", WorkID: "work-reserve", Instance: store.instance(),
		Definition: definition, DecidedAt: now.Add(time.Second), Deadline: now.Add(time.Hour),
		IdempotencyKey: "reserve-1", Input: []byte("order-42"),
	}))
	mustRecipeOK(store.Commit(ctx, reserveDecision.Transition()))
	reserveLease := store.lease("work-reserve", now.Add(2*time.Second))
	reserveProcessor := mustRecipe(workflow.NewActivityWorkProcessor(workflow.ActivityWorkProcessorConfig{
		Store: store, Definitions: definitions, Activities: activities,
		Clock: recipeClock{now: now.Add(2 * time.Second)}, PageSize: 16, MaxHistoryEvents: 64,
	}))
	mustRecipeComplete(reserveProcessor.Process(ctx, reserveLease))
	store.complete("work-reserve")
	reserveProgress, _ := store.instance().Activity("reserve")
	if reserveProgress.Status() != workflow.ActivityProgressSucceeded {
		panic("reserve activity did not reach known success")
	}

	chargeDecision := mustRecipe(workflow.NewOrchestrationDecision(workflow.OrchestrationDecisionSpec{
		TransitionID: "schedule-charge", WorkID: "work-charge", Instance: store.instance(),
		Definition: definition, DecidedAt: now.Add(3 * time.Second), Deadline: now.Add(time.Hour),
		IdempotencyKey: "charge-1", Input: []byte("order-42"),
	}))
	mustRecipeOK(store.Commit(ctx, chargeDecision.Transition()))
	chargeLease := store.lease("work-charge", now.Add(4*time.Second))
	chargeProcessor := mustRecipe(workflow.NewActivityWorkProcessor(workflow.ActivityWorkProcessorConfig{
		Store: store, Definitions: definitions, Activities: activities,
		Clock: recipeClock{now: now.Add(4 * time.Second)}, PageSize: 16, MaxHistoryEvents: 64,
	}))
	mustRecipeComplete(chargeProcessor.Process(ctx, chargeLease))
	store.complete("work-charge")
	failedCharge, _ := store.instance().Activity("charge")

	// The application authorizes the actor before constructing the command.
	// Commit atomically records its audit event and compensation due work.
	compensate := mustRecipe(workflow.NewOperatorCompensation(workflow.OperatorCompensationSpec{
		CommandID: "compensate-reserve", WorkID: "work-release", Instance: store.instance(),
		Definition: definition, StepName: "reserve", Attempt: 1,
		IdempotencyKey: "release-1", Actor: "operator-7", Reason: "charge-failed",
		OccurredAt: now.Add(5 * time.Second), Deadline: now.Add(time.Hour),
		Input: []byte("reservation-42"),
	}))
	mustRecipeOK(store.Commit(ctx, compensate))
	releaseLease := store.lease("work-release", now.Add(6*time.Second))
	releaseProcessor := mustRecipe(workflow.NewCompensationWorkProcessor(
		workflow.CompensationWorkProcessorConfig{
			Store: store, Definitions: definitions, Compensations: compensations,
			Clock: recipeClock{now: now.Add(6 * time.Second)}, PageSize: 16, MaxHistoryEvents: 64,
		},
	))
	mustRecipeComplete(releaseProcessor.Process(ctx, releaseLease))
	store.complete("work-release")
	unknownRelease, _ := store.instance().Compensation("reserve")

	// Reconciliation happens in the external idempotency system. When it cannot
	// prove success or absence, an authorized operator records a durable manual
	// disposition; manual resolution is deliberately not successful rollback.
	resolution := mustRecipe(workflow.NewOperatorCompensationResolution(
		workflow.OperatorCompensationResolutionSpec{
			CommandID: "resolve-release", Instance: store.instance(), Definition: definition,
			StepName: "reserve", Actor: "operator-7", Reason: "manual-reconciliation",
			Code: "accepted-loss", Evidence: []byte("ticket-123"),
			OccurredAt: now.Add(7 * time.Second),
		},
	))
	mustRecipeOK(store.Commit(ctx, resolution))
	resolvedRelease, _ := store.instance().Compensation("reserve")

	fmt.Println("charge known failure:", failedCharge.Status() == workflow.ActivityProgressFailed)
	fmt.Println("compensation unknown:", unknownRelease.Status() == workflow.CompensationUnknown)
	fmt.Println("manual resolution:", resolvedRelease.Status() == workflow.CompensationManuallyResolved)
	fmt.Println("successful rollback:", resolvedRelease.Status() == workflow.CompensationSucceeded)
	fmt.Println("audited operator actions:", len(store.instance().OperatorActions()))
	// Output:
	// charge known failure: true
	// compensation unknown: true
	// manual resolution: true
	// successful rollback: false
	// audited operator actions: 2
}

// recipeStore is deliberately not a reusable store. It makes this recipe
// executable while preserving the public TransitionStore ownership contract.
type recipeStore struct {
	definitions *workflow.Registry
	history     []workflow.HistoryEvent
	work        map[string]workflow.PendingWork
	transitions map[string]string
}

func newRecipeStore(definitions *workflow.Registry) *recipeStore {
	return &recipeStore{
		definitions: definitions,
		work:        make(map[string]workflow.PendingWork),
		transitions: make(map[string]string),
	}
}

// Commit owns the atomic persistence boundary. It validates the complete next
// history before publishing either history or due work to readers.
func (store *recipeStore) Commit(ctx context.Context, transition workflow.Transition) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if fingerprint, exists := store.transitions[transition.ID()]; exists {
		if fingerprint == transition.Fingerprint() {
			return nil
		}
		return workflow.ErrDuplicateTransition
	}
	if transition.ExpectedSequence() != uint64(len(store.history)) {
		return workflow.ErrStoreConflict
	}
	nextHistory := append(append([]workflow.HistoryEvent(nil), store.history...), transition.Events()...)
	if _, err := workflow.Replay(store.definitions, nextHistory); err != nil {
		return err
	}
	nextWork := make(map[string]workflow.PendingWork, len(store.work)+len(transition.Work()))
	for id, work := range store.work {
		nextWork[id] = work
	}
	for _, work := range transition.Work() {
		nextWork[work.ID()] = work
	}
	store.history = nextHistory
	store.work = nextWork
	store.transitions[transition.ID()] = transition.Fingerprint()
	return nil
}

func (store *recipeStore) History(ctx context.Context, query workflow.HistoryQuery) (workflow.HistoryPage, error) {
	if err := ctx.Err(); err != nil {
		return workflow.HistoryPage{}, err
	}
	if len(store.history) == 0 || query.InstanceID() != store.history[0].InstanceID() {
		return workflow.HistoryPage{}, workflow.ErrStoreNotFound
	}
	start := int(query.AfterSequence())
	if start > len(store.history) {
		return workflow.HistoryPage{}, workflow.ErrStoreNotFound
	}
	end := min(start+int(query.Limit()), len(store.history))
	return workflow.NewHistoryPage(query, store.history[start:end], end < len(store.history))
}

func (store *recipeStore) ReconcileTransition(
	ctx context.Context,
	reconciliation workflow.TransitionReconciliation,
) (workflow.TransitionReconciliationOutcome, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	fingerprint, exists := store.transitions[reconciliation.TransitionID()]
	if !exists {
		return workflow.TransitionMissing, nil
	}
	if fingerprint != reconciliation.Fingerprint() {
		return workflow.TransitionConflicting, nil
	}
	return workflow.TransitionCommitted, nil
}

func (store *recipeStore) instance() workflow.Instance {
	return mustRecipe(workflow.Replay(store.definitions, store.history))
}

func (store *recipeStore) lease(workID string, claimedAt time.Time) workflow.WorkLease {
	work, exists := store.work[workID]
	if !exists {
		panic("recipe work was not committed")
	}
	return mustRecipe(workflow.NewWorkLease(workflow.WorkLeaseSpec{
		Work: work, Owner: "recipe-worker", Token: 1, Attempt: 1,
		ClaimedAt: claimedAt, ExpiresAt: claimedAt.Add(5 * time.Minute),
	}))
}

func (store *recipeStore) complete(workID string) { delete(store.work, workID) }

type recipeClock struct{ now time.Time }

func (clock recipeClock) Now() time.Time { return clock.now }

func (recipeClock) NewTimer(duration time.Duration) workflow.ClockTimer {
	return workflow.SystemClock{}.NewTimer(duration)
}

func mustRecipe[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

func mustRecipeOK(err error) {
	if err != nil {
		panic(err)
	}
}

func mustRecipeComplete(decision workflow.WorkDecision, err error) {
	if err != nil {
		panic(err)
	}
	if decision.Kind() != workflow.WorkComplete {
		panic("recipe work did not complete")
	}
}
