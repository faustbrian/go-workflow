//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	workflow "github.com/faustbrian/go-workflow"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TestPostgreSQLDurableCompensationRecipeSurvivesRestartAndDrainsWorker is the
// durable counterpart to Example_durableCompensationRecovery. It uses only
// public workflow and PostgreSQL-adapter APIs after the test-owned database is
// provisioned.
func TestPostgreSQLDurableCompensationRecipeSurvivesRestartAndDrainsWorker(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pool := integrationPool(t, ctx)
	connection := pool.Config().ConnString()
	schema := fmt.Sprintf("recipe_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatalf("create recipe schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		cleanupPool, err := pgxpool.New(cleanupCtx, connection)
		if err != nil {
			t.Errorf("connect recipe cleanup: %v", err)
			return
		}
		defer cleanupPool.Close()
		if _, err := cleanupPool.Exec(cleanupCtx, "DROP SCHEMA "+identifier+" CASCADE"); err != nil {
			t.Errorf("drop recipe schema: %v", err)
		}
	})
	for _, migration := range mustPostgresRecipe(SchemaMigrationsFor(schema)) {
		if _, err := pool.Exec(ctx, migration.Up); err != nil {
			t.Fatalf("apply recipe migration %d: %v", migration.Version, err)
		}
	}
	store := mustPostgresRecipe(New(pool, Config{Schema: schema}))
	definition, definitions, activities, compensations := durableRecipeRegistries(t)

	startedAt := time.Now().UTC().Add(-time.Minute)
	started := mustPostgresRecipe(workflow.NewHistoryEvent(workflow.HistoryEventSpec{
		Sequence: 1, InstanceID: "order-42", Kind: workflow.EventInstanceStarted,
		OccurredAt: startedAt, Definition: definition.Reference(),
	}))
	mustRecipeCommit(t, ctx, store, mustPostgresRecipe(workflow.NewTransition(workflow.TransitionSpec{
		ID: "start-order-42", InstanceID: "order-42", Definition: definition.Reference(),
		Events: []workflow.HistoryEvent{started},
	})))
	instance := inspectRecipeInstance(t, ctx, store, definitions)
	mustRecipeCommit(t, ctx, store, mustPostgresRecipe(workflow.NewOrchestrationDecision(
		workflow.OrchestrationDecisionSpec{
			TransitionID: "schedule-reserve", WorkID: "work-reserve", Instance: instance,
			Definition: definition, DecidedAt: startedAt.Add(time.Second),
			Deadline: startedAt.Add(10 * time.Minute), IdempotencyKey: "reserve-1",
			Input: []byte("order-42"),
		},
	)).Transition())

	firstWorker := startRecipeWorker(t, ctx, "recipe-worker-1", store,
		definitions, activities, compensations)
	instance = awaitRecipeInstance(t, ctx, store, definitions, func(value workflow.Instance) bool {
		progress, ok := value.Activity("reserve")
		return ok && progress.Status() == workflow.ActivityProgressSucceeded
	})
	mustRecipeCommit(t, ctx, store, mustPostgresRecipe(workflow.NewOrchestrationDecision(
		workflow.OrchestrationDecisionSpec{
			TransitionID: "schedule-charge", WorkID: "work-charge", Instance: instance,
			Definition: definition, DecidedAt: time.Now().UTC(),
			Deadline: time.Now().UTC().Add(10 * time.Minute), IdempotencyKey: "charge-1",
			Input: []byte("order-42"),
		},
	)).Transition())
	failed := awaitRecipeInstance(t, ctx, store, definitions, func(value workflow.Instance) bool {
		progress, ok := value.Activity("charge")
		return ok && progress.Status() == workflow.ActivityProgressFailed
	})

	// Shutdown order is application-owned: stop admission, wait until Worker.Run
	// has joined every handler, then close the caller-owned store dependency.
	stopRecipeWorker(t, ctx, firstWorker)
	pool.Close()

	restartedPool := mustPostgresRecipe(pgxpool.New(ctx, connection))
	t.Cleanup(restartedPool.Close)
	if err := restartedPool.Ping(ctx); err != nil {
		t.Fatalf("ping restarted recipe pool: %v", err)
	}
	restartedStore := mustPostgresRecipe(New(restartedPool, Config{Schema: schema}))
	recovered := inspectRecipeInstance(t, ctx, restartedStore, definitions)
	if progress, ok := recovered.Activity("charge"); !ok || progress.Status() != workflow.ActivityProgressFailed {
		t.Fatalf("recovered charge progress = %#v", progress)
	}

	secondWorker := startRecipeWorker(t, ctx, "recipe-worker-2", restartedStore,
		definitions, activities, compensations)
	authorizer := durableRecipeAuthorizer{permissions: map[string]map[workflow.OperatorAction]bool{
		"operator-7": {
			workflow.OperatorCompensate:          true,
			workflow.OperatorResolveCompensation: true,
		},
	}}
	if err := authorizer.Authorize("operator-7", workflow.OperatorCompensate); err != nil {
		t.Fatalf("authorize compensation: %v", err)
	}
	mustRecipeCommit(t, ctx, restartedStore, mustPostgresRecipe(workflow.NewOperatorCompensation(
		workflow.OperatorCompensationSpec{
			CommandID: "compensate-reserve", WorkID: "work-release", Instance: recovered,
			Definition: definition, StepName: "reserve", Attempt: 1,
			IdempotencyKey: "release-1", Actor: "operator-7", Reason: "charge-failed",
			OccurredAt: time.Now().UTC(), Deadline: time.Now().UTC().Add(10 * time.Minute),
			Input: []byte("reservation-42"),
		},
	)))
	unknown := awaitRecipeInstance(t, ctx, restartedStore, definitions, func(value workflow.Instance) bool {
		progress, ok := value.Compensation("reserve")
		return ok && progress.Status() == workflow.CompensationUnknown
	})
	if err := authorizer.Authorize("operator-7", workflow.OperatorResolveCompensation); err != nil {
		t.Fatalf("authorize compensation resolution: %v", err)
	}
	mustRecipeCommit(t, ctx, restartedStore, mustPostgresRecipe(
		workflow.NewOperatorCompensationResolution(workflow.OperatorCompensationResolutionSpec{
			CommandID: "resolve-release", Instance: unknown, Definition: definition,
			StepName: "reserve", Actor: "operator-7", Reason: "manual-reconciliation",
			Code: "accepted-loss", Evidence: []byte("ticket-123"), OccurredAt: time.Now().UTC(),
		}),
	))
	resolved := inspectRecipeInstance(t, ctx, restartedStore, definitions)
	progress, ok := resolved.Compensation("reserve")
	actions := resolved.OperatorActions()
	if !ok || progress.Status() != workflow.CompensationManuallyResolved ||
		progress.Status() == workflow.CompensationSucceeded || progress.Code() != "accepted-loss" ||
		string(progress.Result()) != "ticket-123" || len(actions) != 2 {
		t.Fatalf("resolved compensation = %#v actions = %d", progress, len(resolved.OperatorActions()))
	}
	if actions[0].CommandID() != "compensate-reserve" ||
		actions[0].Action() != workflow.OperatorCompensate ||
		actions[0].Actor() != "operator-7" || actions[0].Reason() != "charge-failed" ||
		actions[1].CommandID() != "resolve-release" ||
		actions[1].Action() != workflow.OperatorResolveCompensation ||
		actions[1].Actor() != "operator-7" || actions[1].Reason() != "manual-reconciliation" {
		t.Fatalf("operator audit actions = %#v", actions)
	}
	if charge, ok := failed.Activity("charge"); !ok || charge.Status() != workflow.ActivityProgressFailed {
		t.Fatalf("pre-restart charge progress = %#v", charge)
	}

	stopRecipeWorker(t, ctx, secondWorker)
	restartedPool.Close()
}

type durableRecipeAuthorizer struct {
	permissions map[string]map[workflow.OperatorAction]bool
}

func (authorizer durableRecipeAuthorizer) Authorize(actor string, action workflow.OperatorAction) error {
	if authorizer.permissions[actor][action] {
		return nil
	}
	return errors.New("recipe operator is not authorized")
}

type durableRecipeProcessor struct {
	activity     workflow.WorkProcessor
	compensation workflow.WorkProcessor
}

func (processor durableRecipeProcessor) Process(
	ctx context.Context,
	lease workflow.WorkLease,
) (workflow.WorkDecision, error) {
	switch lease.Work().Kind() {
	case workflow.WorkActivity:
		return processor.activity.Process(ctx, lease)
	case workflow.WorkCompensation:
		return processor.compensation.Process(ctx, lease)
	default:
		return workflow.WorkDecision{}, errors.New("recipe worker received unsupported work")
	}
}

func durableRecipeRegistries(
	t *testing.T,
) (workflow.Definition, *workflow.Registry, *workflow.ActivityRegistry, *workflow.ActivityRegistry) {
	t.Helper()
	definition := mustPostgresRecipe(workflow.NewDefinition(workflow.DefinitionSpec{
		Name: "orders", Version: "compensation-v1", Mode: workflow.Orchestration,
		Steps: []workflow.StepSpec{
			{
				Name: "reserve", Kind: workflow.StepActivity, Target: "inventory.reserve",
				Timeout: time.Minute, InputLimit: 64, ResultLimit: 64,
				Retry: workflow.RetryPolicy{MaxAttempts: 1, InitialDelay: time.Second, MaxDelay: time.Second},
				Compensation: &workflow.CompensationSpec{
					Target: "inventory.release", Timeout: time.Minute, ResultLimit: 64,
					Retry: workflow.RetryPolicy{MaxAttempts: 1, InitialDelay: time.Second, MaxDelay: time.Second},
				},
			},
			{
				Name: "charge", Kind: workflow.StepActivity, Target: "payments.charge",
				Timeout: time.Minute, InputLimit: 64, ResultLimit: 64,
				Retry: workflow.RetryPolicy{MaxAttempts: 1, InitialDelay: time.Second, MaxDelay: time.Second},
			},
		},
	}))
	definitions := mustPostgresRecipe(workflow.CompileDefinitions(definition))
	reserve := mustPostgresRecipe(workflow.NewActivity("inventory.reserve",
		func(context.Context, workflow.ActivityRequest) workflow.ActivityOutcome {
			return mustPostgresRecipe(workflow.NewActivityOutcome(workflow.ActivityOutcomeSpec{
				Kind: workflow.ActivitySucceeded, Data: []byte("reservation-42"),
			}))
		}))
	charge := mustPostgresRecipe(workflow.NewActivity("payments.charge",
		func(context.Context, workflow.ActivityRequest) workflow.ActivityOutcome {
			return mustPostgresRecipe(workflow.NewActivityOutcome(workflow.ActivityOutcomeSpec{
				Kind: workflow.ActivityFailed, Code: "card-declined",
			}))
		}))
	release := mustPostgresRecipe(workflow.NewActivity("inventory.release",
		func(context.Context, workflow.ActivityRequest) workflow.ActivityOutcome {
			return mustPostgresRecipe(workflow.NewActivityOutcome(workflow.ActivityOutcomeSpec{
				Kind: workflow.ActivityUnknown, Code: "release-commit-unknown",
			}))
		}))
	return definition, definitions,
		mustPostgresRecipe(workflow.CompileActivities(reserve, charge)),
		mustPostgresRecipe(workflow.CompileActivities(release))
}

func startRecipeWorker(
	t *testing.T,
	ctx context.Context,
	owner string,
	store *Store,
	definitions *workflow.Registry,
	activities *workflow.ActivityRegistry,
	compensations *workflow.ActivityRegistry,
) *durableRecipeWorker {
	t.Helper()
	clock := workflow.SystemClock{}
	activity := mustPostgresRecipe(workflow.NewActivityWorkProcessor(workflow.ActivityWorkProcessorConfig{
		Store: store, Definitions: definitions, Activities: activities, Clock: clock,
		PageSize: 16, MaxHistoryEvents: 128,
	}))
	compensation := mustPostgresRecipe(workflow.NewCompensationWorkProcessor(
		workflow.CompensationWorkProcessorConfig{
			Store: store, Definitions: definitions, Compensations: compensations, Clock: clock,
			PageSize: 16, MaxHistoryEvents: 128,
		},
	))
	worker := mustPostgresRecipe(workflow.NewWorker(workflow.WorkerConfig{
		Store: store, Processor: durableRecipeProcessor{activity: activity, compensation: compensation},
		Clock: clock, Owner: owner, MaxConcurrent: 2, ClaimLimit: 2,
		LeaseDuration: 30 * time.Second, RenewEvery: 10 * time.Second,
		PollInterval: 5 * time.Millisecond, FinalizeTimeout: 5 * time.Second,
	}))
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- worker.Run(runCtx) }()
	run := &durableRecipeWorker{cancel: cancel, done: done}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := run.Stop(cleanupCtx); err != nil {
			t.Errorf("cleanup recipe worker: %v", err)
		}
	})
	return run
}

type durableRecipeWorker struct {
	cancel context.CancelFunc
	done   <-chan error
	once   sync.Once
	err    error
}

func (worker *durableRecipeWorker) Stop(ctx context.Context) error {
	worker.once.Do(func() {
		worker.cancel()
		select {
		case worker.err = <-worker.done:
		case <-ctx.Done():
			worker.err = ctx.Err()
		}
	})
	return worker.err
}

func stopRecipeWorker(t *testing.T, ctx context.Context, worker *durableRecipeWorker) {
	t.Helper()
	if err := worker.Stop(ctx); err != nil {
		t.Fatalf("drain recipe worker: %v", err)
	}
}

func awaitRecipeInstance(
	t *testing.T,
	ctx context.Context,
	store *Store,
	definitions *workflow.Registry,
	ready func(workflow.Instance) bool,
) workflow.Instance {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		instance := inspectRecipeInstance(t, ctx, store, definitions)
		if ready(instance) {
			return instance
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatalf("await recipe state: %v", ctx.Err())
		}
	}
}

func inspectRecipeInstance(
	t *testing.T,
	ctx context.Context,
	store *Store,
	definitions *workflow.Registry,
) workflow.Instance {
	t.Helper()
	return mustPostgresRecipe(workflow.InspectInstance(ctx, store, definitions, workflow.InstanceInspectionSpec{
		InstanceID: "order-42", PageSize: 16, MaxEvents: 128,
	}))
}

func mustRecipeCommit(t *testing.T, ctx context.Context, store *Store, transition workflow.Transition) {
	t.Helper()
	if err := store.Commit(ctx, transition); err != nil {
		t.Fatalf("commit recipe transition %q: %v", transition.ID(), err)
	}
}

func mustPostgresRecipe[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
