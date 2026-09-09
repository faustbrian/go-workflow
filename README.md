# workflow

[![CI](https://github.com/faustbrian/go-workflow/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/faustbrian/go-workflow/actions/workflows/ci.yml)
[![CodeQL](https://img.shields.io/badge/CodeQL-required-blue)](https://github.com/faustbrian/go-workflow/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-100%25_required-blue)](CONTRIBUTING.md#verification)
[![Mutation](https://img.shields.io/badge/mutation-100%25_required-blue)](CONTRIBUTING.md#verification)
[![Documentation](https://img.shields.io/badge/docs-checked_in_CI-blue)](docs/)
[![Go Reference](https://pkg.go.dev/badge/github.com/faustbrian/go-workflow.svg)](https://pkg.go.dev/github.com/faustbrian/go-workflow)
[![Release](https://img.shields.io/github/v/release/faustbrian/go-workflow?sort=semver)](https://github.com/faustbrian/go-workflow/releases)
[![Go](https://img.shields.io/badge/go-1.26.6-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`workflow` provides explicit building blocks for durable business workflows and
sagas. It owns immutable definitions, deterministic replay, transition and
durable-work contracts, recovery semantics, and a PostgreSQL store. The
application still owns business handlers, authorization, transport,
publication and acknowledgement, process supervision, and external-effect
idempotency.

The released `v1.0.0` line is stable and actively maintained. It requires Go
1.26.6.

For ecosystem-wide design and selection guidance, see the versioned
[Golib ecosystem index](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/README.md)
and its
[Persistence and durability family](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/design-language.md#package-families-and-selection).

## Install and packages

```sh
go get github.com/faustbrian/go-workflow@v1.0.0
```

- `github.com/faustbrian/go-workflow` contains definitions, history, replay,
  transitions, durable work, processors, workers, inspection, and operator
  commands.
- `github.com/faustbrian/go-workflow/postgres` provides the pgx-backed store and
  ordered schema migrations inside the same module and release tag.

Use Workflow when a process needs durable history, external activities,
retries, compensation, timers, signals, fenced workers, and crash recovery.
Choose [State Machine](https://github.com/faustbrian/go-state-machine) for typed,
deterministic in-process state transitions without this orchestration and
durability model. Choose [Temporal](https://github.com/faustbrian/go-temporal)
for bounded period, interval, sequence, and time-of-day algebra; it is not a
workflow engine.

## Quick start

```go
package main

import (
	"log"
	"time"

	workflow "github.com/faustbrian/go-workflow"
)

func main() {
	definition, err := workflow.NewDefinition(workflow.DefinitionSpec{
		Name:    "order.fulfillment",
		Version: "1",
		Mode:    workflow.Orchestration,
		Steps: []workflow.StepSpec{{
			Name:        "reserve",
			Kind:        workflow.StepActivity,
			Target:      "inventory.reserve",
			Timeout:     time.Minute,
			InputLimit:  16 << 10,
			ResultLimit: 16 << 10,
			Retry: workflow.RetryPolicy{
				MaxAttempts:  3,
				InitialDelay: time.Second,
				MaxDelay:     time.Minute,
			},
		}},
	})
	if err != nil {
		log.Fatal(err)
	}

	if _, err := workflow.CompileDefinitions(definition); err != nil {
		log.Fatal(err)
	}
}
```

The equivalent
[`Example_durableOrchestration`](example_test.go) compiles and runs in the
documentation gate.

### Durable compensation and recovery recipe

[`Example_durableCompensationRecovery`](durable_compensation_example_test.go)
is the fast executable, non-releasable public-API composition. Its durable
counterpart,
[`TestPostgreSQLDurableCompensationRecipeSurvivesRestartAndDrainsWorker`](postgres/durable_compensation_recipe_integration_test.go),
executes the same lifecycle through the PostgreSQL store and real workers. It
proves that a known activity failure survives application store reconstruction,
then covers audited compensation admission, an unknown compensation outcome,
and explicit operator resolution that remains distinct from successful
rollback.

The application owns the definition and activity registries, handler
idempotency and reconciliation, actor authorization, process context, and
worker shutdown order. The persistence adapter owns each atomic history and due
work commit. Processors persist attempt starts before invoking handlers and
persist outcomes before work acknowledgement. On shutdown, cancel admission,
wait for `Worker.Run` to return, and close the application-owned store last.

## Contract summary

- Constructors validate before starting work. Definitions, registry entries,
  history values, transitions, and byte-backed results retain owned copies.
- A `Transition` is the atomic boundary for contiguous history and due work.
  Unknown commit outcomes must be reconciled by transition identity before a
  retry.
- `Worker.Run` owns bounded handler and renewal goroutines until it returns.
  Caller cancellation stops admission, cancels active processors, and waits
  for them to exit. Use one active `Run` call per worker.
- The PostgreSQL store starts no goroutines and does not own the supplied pool,
  schema, migrations, or caller transaction. `Stage` neither commits nor rolls
  back a caller-owned transaction.
- Stable errors support `errors.Is`; `StoreCommitError` additionally exposes
  whether a commit is known absent, known committed, or unknown. Error text is
  not a classification contract.
- Activity and compensation execution does not promise exactly-once external
  effects. Attempt starts and idempotency identities are durable, but callers
  must reconcile unknown outcomes.

## Documentation

- [Documentation index](docs/README.md)
- [Durable compensation and recovery recipe](durable_compensation_example_test.go)
- [PostgreSQL durability and shutdown proof](postgres/durable_compensation_recipe_integration_test.go)
- [Architecture and package boundaries](docs/architecture.md)
- [API and lifecycle reference](docs/reference.md)
- [Operations, recovery, capacity, and security](docs/operations.md)
- [Troubleshooting and FAQ](docs/troubleshooting.md)
- [Verification and performance evidence](docs/verification.md)
- [Compatibility](COMPATIBILITY.md), [deprecation](DEPRECATION.md), and
  [release history](CHANGELOG.md)
- [Support](SUPPORT.md) and [private security reporting](SECURITY.md)
- [Go API reference](https://pkg.go.dev/github.com/faustbrian/go-workflow)

## License

MIT. See [LICENSE](LICENSE).
