package database

import (
	"context"
	"errors"
	"testing"

	"xorm.io/xorm"
)

type saveLifecycleModel struct {
	BaseModel `xorm:"extends"`
	ID        int64  `xorm:"pk autoincr 'id'"`
	Name      string `xorm:"'name'"`
	UpdatedBy string `xorm:"'updated_by'"`
}

func (saveLifecycleModel) TableName() string { return "save_lifecycle_test" }

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

func TestSaveUsesModelLifecycleAndJoinsTransaction(t *testing.T) {
	db, err := NewDatabase("sqlite", "file:db-save-lifecycle?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Engine().Exec("CREATE TABLE save_lifecycle_test (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, version_no INTEGER NOT NULL, created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, updated_by TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}

	model := &saveLifecycleModel{Name: "first", UpdatedBy: "operator"}
	model.CreateNew("operator")
	if _, err := db.Save(model); err != nil {
		t.Fatalf("insert with Save: %v", err)
	}
	if model.ID == 0 || model.VersionNo != 1 {
		t.Fatalf("Save did not populate generated identity and initial version: %+v", model)
	}

	var stale saveLifecycleModel
	found, err := db.Engine().ID(model.ID).Get(&stale)
	if err != nil || !found {
		t.Fatalf("load model for optimistic update: found=%v err=%v", found, err)
	}
	model.Name = "updated"
	model.UpdatedBy = "operator-2"
	model.Update("operator-2")
	model.Cols("name")
	if err := db.WithSaveTransaction(context.Background(), func(tx *DBTransaction) error {
		if _, err := tx.Save(model); err != nil {
			return err
		}
		rollback := &saveLifecycleModel{Name: "rolled back", UpdatedBy: "operator"}
		rollback.CreateNew("operator")
		if _, err := tx.Save(rollback); err != nil {
			return err
		}
		return errors.New("rollback save transaction")
	}); err == nil {
		t.Fatal("expected callback error to roll back model Save operations")
	}
	var persisted saveLifecycleModel
	if found, err := db.Engine().NewSession().ID(model.ID).Get(&persisted); err != nil || !found {
		t.Fatalf("reload model after rollback: found=%v err=%v", found, err)
	}
	if persisted.Name != "first" || persisted.VersionNo != 1 {
		t.Fatalf("Save transaction did not roll back update: %+v", persisted)
	}

	model.Name = "updated"
	model.Update("operator-2")
	model.Cols("name")
	if _, err := db.Save(model); err != nil {
		t.Fatalf("update with Save: %v", err)
	}
	if model.State() != ModelState_None {
		t.Fatalf("successful Save did not clear the pending model state: %v", model.State())
	}
	stale.Name = "stale update"
	stale.Update("operator-2")
	if _, err := db.Save(&stale); err == nil {
		t.Fatal("expected stale model Save to fail optimistic version check")
	}
	if found, err := db.Engine().NewSession().SQL("SELECT * FROM save_lifecycle_test WHERE id=?", model.ID).Get(&persisted); err != nil || !found {
		t.Fatalf("reload model after stale update: found=%v err=%v", found, err)
	}
	if persisted.Name != "updated" || persisted.VersionNo != 2 {
		t.Fatalf("stale Save changed persisted model: %+v", persisted)
	}
}
