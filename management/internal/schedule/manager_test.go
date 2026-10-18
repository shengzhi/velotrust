package schedule

import (
	"context"
	"testing"
)

type testTask struct{ name string }

func (t testTask) TaskName() string                  { return t.name }
func (testTask) Run(context.Context) (Result, error) { return Result{Summary: "ok"}, nil }
func TestScheduleManagerRegisterRejectsDuplicateTaskName(t *testing.T) {
	manager := NewScheduleManager(0)
	if err := manager.Register(testTask{name: "quality.sync"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Register(testTask{name: "quality.sync"}); err == nil {
		t.Fatal("expected duplicate task name error")
	}
}
func TestNormalizeCron(t *testing.T) {
	got, err := normalizeCron("0 */15 * * * ?")
	if err != nil || got != "0 */15 * * * *" {
		t.Fatalf("cron=%q err=%v", got, err)
	}
	if _, err := normalizeCron("*/5 * *"); err == nil {
		t.Fatal("expected invalid cron")
	}
}
