package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func waitRecoveryReady(ctx context.Context, ping func(context.Context) error, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := ping(ctx)
		if err == nil {
			return nil
		}
		var databaseError *pgconn.PgError
		if !errors.As(err, &databaseError) || databaseError.Code != "57P03" {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func TestRecoveryReadyRetriesOnlyStartingUp(t *testing.T) {
	t.Run("immediate success", func(t *testing.T) {
		if err := waitRecoveryReady(t.Context(), func(context.Context) error { return nil }, time.Millisecond); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("transient startup", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		starting := true
		err := waitRecoveryReady(ctx, func(context.Context) error {
			if starting {
				starting = false
				return fmt.Errorf("wrapped startup: %w", &pgconn.PgError{Code: "57P03"})
			}
			return nil
		}, time.Millisecond)
		if err != nil {
			t.Fatalf("transient startup must become ready: %v", err)
		}
	})
	t.Run("persistent startup deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
		defer cancel()
		err := waitRecoveryReady(ctx, func(context.Context) error { return &pgconn.PgError{Code: "57P03"} }, time.Millisecond)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("persistent startup must stop at deadline: %v", err)
		}
	})
	t.Run("cancelled startup", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		err := waitRecoveryReady(ctx, func(context.Context) error {
			cancel()
			return &pgconn.PgError{Code: "57P03"}
		}, time.Hour)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation must interrupt startup wait: %v", err)
		}
	})
	for _, failure := range []error{&pgconn.PgError{Code: "28P01"}, errors.New("connection refused")} {
		t.Run(failure.Error(), func(t *testing.T) {
			calls := 0
			err := waitRecoveryReady(t.Context(), func(context.Context) error { calls++; return failure }, time.Hour)
			if !errors.Is(err, failure) || calls != 1 {
				t.Fatalf("non-startup failure must be returned immediately: %v, calls=%d", err, calls)
			}
		})
	}
}
