package database

import (
	"context"
	"errors"
	"testing"

	"xorm.io/xorm"
)

func TestWithTransactionRollsBackOnCallbackError(t *testing.T) {
	db, err := NewDatabase("sqlite", "file:db-transaction-rollback?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Engine().Exec("CREATE TABLE transaction_test (value TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("stop transaction")
	err = db.WithTransaction(context.Background(), func(session *xorm.Session) error {
		if _, err := session.Exec("INSERT INTO transaction_test (value) VALUES (?)", "rolled back"); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected callback error, got %v", err)
	}
	var result struct {
		Count int64 `xorm:"'count'"`
	}
	if _, err := db.Engine().SQL("SELECT COUNT(*) AS count FROM transaction_test").Get(&result); err != nil {
		t.Fatal(err)
	}
	if result.Count != 0 {
		t.Fatalf("transaction callback error left %d committed rows", result.Count)
	}
	if err := db.WithTransaction(context.Background(), func(session *xorm.Session) error {
		_, err := session.Exec("INSERT INTO transaction_test (value) VALUES (?)", "committed")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Engine().SQL("SELECT COUNT(*) AS count FROM transaction_test").Get(&result); err != nil {
		t.Fatal(err)
	}
	if result.Count != 1 {
		t.Fatalf("successful transaction committed %d rows, want 1", result.Count)
	}
}
