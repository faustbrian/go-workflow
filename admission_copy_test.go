package workflow_test

import (
	"errors"
	"testing"
	"time"

	workflow "github.com/faustbrian/go-workflow"
)

// These tiny rejected specs must not allocate defensive copies. This is an
// admission-order assertion, not a throughput or general allocation budget.
func TestConstructorsRejectBeforeDefensiveCopies(t *testing.T) {
	definition := mustDefinition(t, "orders", "1").Reference()
	now := time.Date(2036, 8, 11, 15, 0, 0, 0, time.UTC)
	input := []byte("ok")
	activity := workflow.ActivityRequestSpec{
		InstanceID: "instance-1", Definition: definition, StepName: "execute",
		Attempt: 1, MaxAttempts: 1, IdempotencyKey: "activity-key",
		StartedAt: now, Deadline: now.Add(time.Minute),
		Input: input, InputLimit: 1, ResultLimit: 1,
	}
	child := workflow.ChildStartRequestSpec{
		ParentInstanceID: "instance-1", ParentDefinition: definition,
		StepName: "child", ChildID: "child-1", ChildDefinition: definition,
		Attempt: 1, MaxAttempts: 1, IdempotencyKey: "child-key",
		StartedAt: now, Deadline: now.Add(time.Minute), Input: input, InputLimit: 1,
	}
	work, err := workflow.NewPendingWork(workflow.PendingWorkSpec{
		ID: "work-1", Kind: workflow.WorkActivity, InstanceID: "instance-1",
		Sequence: 1, AvailableAt: now, Deadline: now.Add(time.Minute), Payload: input,
	})
	if err != nil {
		t.Fatal(err)
	}
	transition := workflow.TransitionSpec{
		InstanceID: "instance-1", Definition: definition,
		Events: []workflow.HistoryEvent{{}}, Work: []workflow.PendingWork{work},
	}
	for _, test := range []struct {
		name   string
		reject func()
	}{
		{"activity input bound", func() {
			value, err := workflow.NewActivityRequest(activity)
			if !errors.Is(err, workflow.ErrInvalidActivityRequest) || value.InstanceID() != "" || value.Input() != nil {
				t.Fatal("activity rejection changed")
			}
		}},
		{"child input bound", func() {
			value, err := workflow.NewChildStartRequest(child)
			if !errors.Is(err, workflow.ErrInvalidChildStart) || value.ChildID() != "" || value.Input() != nil {
				t.Fatal("child rejection changed")
			}
		}},
		{"outcome classification", func() {
			value, err := workflow.NewActivityOutcome(workflow.ActivityOutcomeSpec{Data: input})
			if !errors.Is(err, workflow.ErrInvalidActivityOutcome) || value.Kind() != 0 || value.Data() != nil {
				t.Fatal("outcome rejection changed")
			}
		}},
		{"pending work identity", func() {
			value, err := workflow.NewPendingWork(workflow.PendingWorkSpec{Payload: input})
			if !errors.Is(err, workflow.ErrInvalidPendingWork) || value.ID() != "" || value.Payload() != nil {
				t.Fatal("work rejection changed")
			}
		}},
		{"transition identity", func() {
			value, err := workflow.NewTransition(transition)
			if !errors.Is(err, workflow.ErrInvalidTransitionPlan) || value.Valid() || value.Events() != nil || value.Work() != nil {
				t.Fatal("transition rejection changed")
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if allocations := testing.AllocsPerRun(10, test.reject); allocations != 0 {
				t.Errorf("rejected spec allocated %g times, want no defensive-copy allocations", allocations)
			}
		})
	}
	if string(input) != "ok" || string(work.Payload()) != "ok" {
		t.Fatal("rejection changed borrowed input")
	}
}
