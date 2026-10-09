//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	workflow "github.com/faustbrian/go-workflow/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgreSQLProtocolModesPreserveWorkflowInstants(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	basePool := integrationPool(t, ctx)
	for _, test := range []struct {
		name string
		mode pgx.QueryExecMode
	}{
		{name: "binary", mode: pgx.QueryExecModeCacheStatement},
		{name: "text", mode: pgx.QueryExecModeSimpleProtocol},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := basePool.Config()
			config.ConnConfig.DefaultQueryExecMode = test.mode
			config.ConnConfig.RuntimeParams["timezone"] = "Asia/Tokyo"
			config.AfterConnect = func(_ context.Context, connection *pgx.Conn) error {
				connection.TypeMap().RegisterType(&pgtype.Type{
					Name: "timestamptz", OID: pgtype.TimestamptzOID,
					Codec: &pgtype.TimestamptzCodec{ScanLocation: time.FixedZone("client", -5*60*60)},
				})
				return nil
			}
			pool, err := pgxpool.NewWithConfig(ctx, config)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(pool.Close)
			var zone string
			if err := pool.QueryRow(ctx, "SHOW timezone").Scan(&zone); err != nil || zone != "Asia/Tokyo" {
				t.Fatalf("session timezone = %q, %v", zone, err)
			}
			schema := "workflow_timestamp_" + test.name
			if _, err := pool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
				t.Fatal(err)
			}
			migration, err := SchemaMigrationFor(schema)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, migration.Up); err != nil {
				t.Fatal(err)
			}
			store, err := New(pool, Config{Schema: schema})
			if err != nil {
				t.Fatal(err)
			}
			created := mustCreateTransition(t)
			for _, transition := range []workflow.Transition{created, mustAttemptTransition(t, created.Definition())} {
				if err := store.Commit(ctx, transition); err != nil {
					t.Fatal(err)
				}
			}
			page, err := store.History(ctx, mustHistoryQuery(t, 0, 10))
			if err != nil || len(page.Events()) != 3 {
				t.Fatalf("history = %#v, %v", page, err)
			}
			occurred := time.Date(2026, 8, 9, 12, 3, 0, 0, time.UTC)
			event := page.Events()[2]
			if event.OccurredAt() != occurred || event.DueAt() != occurred.Add(time.Minute) ||
				!page.Events()[0].DueAt().IsZero() {
				t.Fatalf("history instants = %v, %v; nullable due = %v", event.OccurredAt(), event.DueAt(), page.Events()[0].DueAt())
			}
			var epochMicros int64
			query := "SELECT (EXTRACT(EPOCH FROM occurred_at)*1000000)::bigint FROM " +
				pgx.Identifier{schema, "workflow_history"}.Sanitize() + " WHERE sequence = 3"
			if err := pool.QueryRow(ctx, query).Scan(&epochMicros); err != nil || epochMicros != occurred.UnixMicro() {
				t.Fatalf("server epoch microseconds = %d, %v", epochMicros, err)
			}
			due := time.Date(2026, 8, 9, 12, 0, 1, 0, time.UTC)
			claim := func(now time.Time, wantCount int, wantToken uint64) []workflow.WorkLease {
				t.Helper()
				request, err := workflow.NewWorkClaimRequest(workflow.WorkClaimRequestSpec{
					Owner: "timestamp-worker", Now: now, LeaseDuration: 30 * time.Second, Limit: 10,
				})
				if err != nil {
					t.Fatal(err)
				}
				leases, err := store.Claim(ctx, request)
				if err != nil || len(leases) != wantCount {
					t.Fatalf("claim at %v = %#v, %v", now, leases, err)
				}
				if wantCount != 0 && leases[0].Token() != wantToken {
					t.Fatalf("lease token = %d, want %d", leases[0].Token(), wantToken)
				}
				return leases
			}
			claim(due.Add(-time.Microsecond), 0, 0)
			leases := claim(due, 1, 1)
			if leases[0].Work().AvailableAt() != due || leases[0].ExpiresAt() != due.Add(30*time.Second) {
				t.Fatalf("work/lease instants = %v, %v", leases[0].Work().AvailableAt(), leases[0].ExpiresAt())
			}
			claim(due.Add(30*time.Second-time.Microsecond), 0, 0)
			claim(due.Add(30*time.Second), 1, 2)
		})
	}
}
