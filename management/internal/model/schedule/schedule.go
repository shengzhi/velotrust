package schedule

import (
	"time"

	"velotrust/shared/repository/database"
)

type ScheduledTask struct {
	database.DeletableModel `xorm:"extends"`
	ID                      string     `gorm:"column:id;primaryKey;size:36" xorm:"pk 'id'" json:"id"`
	TaskCode                string     `gorm:"column:task_code;size:128" json:"task_code"`
	TaskName                string     `gorm:"column:task_name;size:128" json:"task_name"`
	CronExpression          string     `gorm:"column:cron_expression;size:128" json:"cron_expression"`
	HandlerParams           string     `gorm:"column:handler_params" json:"handler_params,omitempty"`
	Remark                  string     `gorm:"column:remark;size:500" json:"remark,omitempty"`
	Status                  string     `gorm:"column:status;size:16" json:"status"`
	LastRunAt               *time.Time `gorm:"column:last_run_at" json:"last_run_at,omitempty"`
	NextRunAt               *time.Time `gorm:"column:next_run_at" json:"next_run_at,omitempty"`
}

func (ScheduledTask) TableName() string  { return "t_sch_task" }
func (m *ScheduledTask) PKValues() []any { return []any{m.ID} }

type ScheduledTaskExecution struct {
	database.BaseModel `xorm:"extends"`
	ID                 string     `gorm:"column:id;primaryKey;size:36" xorm:"pk 'id'" json:"id"`
	TaskID             string     `gorm:"column:task_id;size:36" json:"task_id"`
	ScheduledAt        time.Time  `gorm:"column:scheduled_at" json:"scheduled_at"`
	StartedAt          time.Time  `gorm:"column:started_at" json:"started_at"`
	FinishedAt         *time.Time `gorm:"column:finished_at" json:"finished_at,omitempty"`
	Status             string     `gorm:"column:status;size:16" json:"status"`
	DurationMS         *int64     `gorm:"column:duration_ms" json:"duration_ms,omitempty"`
	ErrorMessage       string     `gorm:"column:error_message" json:"error_message,omitempty"`
	ResultSummary      string     `gorm:"column:result_summary" json:"result_summary,omitempty"`
}

func (ScheduledTaskExecution) TableName() string  { return "t_sch_task_execution" }
func (m *ScheduledTaskExecution) PKValues() []any { return []any{m.ID} }

var (
	_ database.IModel = (*ScheduledTask)(nil)
	_ database.IModel = (*ScheduledTaskExecution)(nil)
)
