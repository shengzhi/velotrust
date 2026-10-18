package schedule

import "context"

type Task interface {
	TaskName() string
	Run(context.Context) (Result, error)
}

type Result struct{ Summary string }
