package schedule

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	model "velotrust/management/internal/model/schedule"
	db "velotrust/management/internal/repository"
	"velotrust/shared/constants"
)

type scheduledEntry struct {
	cronEntry      cron.EntryID
	version        int
	cron, taskCode string
}

// ScheduleManager owns task implementations and reconciles persisted schedules
// with cron. It is independent of the application process that starts it.
type ScheduleManager struct {
	cron    *cron.Cron
	tasks   map[string]Task
	entries map[string]scheduledEntry
	running map[string]bool
	mu      sync.Mutex
	timeout time.Duration
}

func NewScheduleManager(timeout time.Duration) *ScheduleManager {
	if timeout <= 0 {
		timeout = constants.DefaultScheduleTaskTimeout
	}
	return &ScheduleManager{cron: cron.New(cron.WithSeconds()), tasks: map[string]Task{}, entries: map[string]scheduledEntry{}, running: map[string]bool{}, timeout: timeout}
}

func (m *ScheduleManager) Register(task Task) error {
	if task == nil || strings.TrimSpace(task.TaskName()) == "" {
		return fmt.Errorf("scheduled task name is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.tasks[task.TaskName()]; exists {
		return fmt.Errorf("scheduled task %q already registered", task.TaskName())
	}
	m.tasks[task.TaskName()] = task
	return nil
}

func (m *ScheduleManager) Start(ctx context.Context, syncEvery time.Duration) {
	if syncEvery <= 0 {
		syncEvery = constants.DefaultScheduleSyncInterval
	}
	m.cron.Start()
	m.Sync(ctx)
	ticker := time.NewTicker(syncEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			m.Stop()
			return
		case <-ticker.C:
			m.Sync(ctx)
		}
	}
}
func (m *ScheduleManager) Stop() { stopped := m.cron.Stop(); <-stopped.Done() }

func (m *ScheduleManager) Sync(_ context.Context) {
	var definitions []model.ScheduledTask
	if err := db.Engine().Where("is_del=?", false).Find(&definitions); err != nil {
		slog.Error("load scheduled tasks", "error", err)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := make(map[string]struct{}, len(definitions))
	for i := range definitions {
		definition := &definitions[i]
		seen[definition.ID] = struct{}{}
		entry, exists := m.entries[definition.ID]
		implementation := m.tasks[definition.TaskCode]
		if definition.Status != "RUNNING" || implementation == nil {
			if exists {
				m.cron.Remove(entry.cronEntry)
				delete(m.entries, definition.ID)
			}
			continue
		}
		expr, err := normalizeCron(definition.CronExpression)
		if err != nil {
			if exists {
				m.cron.Remove(entry.cronEntry)
				delete(m.entries, definition.ID)
			}
			slog.Error("invalid scheduled task cron", "task_id", definition.ID, "error", err)
			continue
		}
		if exists && entry.version == definition.VersionNo && entry.cron == expr && entry.taskCode == definition.TaskCode {
			continue
		}
		if exists {
			m.cron.Remove(entry.cronEntry)
		}
		id, err := m.cron.AddFunc(expr, func() { m.run(definition.ID) })
		if err != nil {
			slog.Error("register scheduled task", "task_id", definition.ID, "error", err)
			continue
		}
		m.entries[definition.ID] = scheduledEntry{cronEntry: id, version: definition.VersionNo, cron: expr, taskCode: definition.TaskCode}
		m.saveNextRun(definition, m.cron.Entry(id).Next)
	}
	for id, entry := range m.entries {
		if _, ok := seen[id]; !ok {
			m.cron.Remove(entry.cronEntry)
			delete(m.entries, id)
		}
	}
}

func (m *ScheduleManager) run(taskID string) {
	m.mu.Lock()
	if m.running[taskID] {
		m.mu.Unlock()
		return
	}
	m.running[taskID] = true
	m.mu.Unlock()
	defer func() { m.mu.Lock(); delete(m.running, taskID); m.mu.Unlock() }()
	var definition model.ScheduledTask
	found, err := db.Engine().Where("id=? AND is_del=? AND status=?", taskID, false, "RUNNING").Get(&definition)
	if err != nil || !found {
		return
	}
	m.mu.Lock()
	implementation := m.tasks[definition.TaskCode]
	m.mu.Unlock()
	if implementation == nil {
		return
	}
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	result, err := implementation.Run(ctx)
	finished := time.Now()
	status, message := "SUCCESS", ""
	if ctx.Err() == context.DeadlineExceeded {
		status, message = "TIMEOUT", "task execution timed out"
	} else if err != nil {
		status, message = "FAILED", redact(err.Error())
	}
	duration := finished.Sub(started).Milliseconds()
	execution := &model.ScheduledTaskExecution{ID: newID(), TaskID: definition.ID, ScheduledAt: started, StartedAt: started, FinishedAt: &finished, Status: status, DurationMS: &duration, ErrorMessage: message, ResultSummary: redact(result.Summary)}
	execution.CreateNew("system")
	if _, err := db.Save(execution); err != nil {
		slog.Error("save task execution", "error", err)
	}
	m.mu.Lock()
	entry, ok := m.entries[definition.ID]
	next := time.Time{}
	if ok {
		next = m.cron.Entry(entry.cronEntry).Next
	}
	m.mu.Unlock()
	definition.LastRunAt = &started
	if !next.IsZero() {
		definition.NextRunAt = &next
	}
	definition.Update("system")
	definition.Cols("last_run_at", "next_run_at")
	if _, err := db.Save(&definition); err != nil {
		slog.Error("update task runtime state", "error", err)
	}
}

func (m *ScheduleManager) saveNextRun(task *model.ScheduledTask, next time.Time) {
	if next.IsZero() {
		return
	}
	task.NextRunAt = &next
	task.Update("system")
	task.Cols("next_run_at")
	if _, err := db.Save(task); err != nil {
		slog.Error("update next task run", "error", err)
	}
}

func normalizeCron(value string) (string, error) {
	fields := strings.Fields(value)
	if len(fields) != 6 {
		return "", fmt.Errorf("cron must contain six fields")
	}
	if fields[3] == "?" {
		fields[3] = "*"
	}
	if fields[5] == "?" {
		fields[5] = "*"
	}
	normalized := strings.Join(fields, " ")
	_, err := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow).Parse(normalized)
	return normalized, err
}
func redact(value string) string {
	if len(value) > 2000 {
		return value[:2000] + "…"
	}
	return value
}
func newID() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes[0:4]) + "-" + hex.EncodeToString(bytes[4:6]) + "-" + hex.EncodeToString(bytes[6:8]) + "-" + hex.EncodeToString(bytes[8:10]) + "-" + hex.EncodeToString(bytes[10:])
}
