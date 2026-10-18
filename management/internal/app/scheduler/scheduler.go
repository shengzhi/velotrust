// Package scheduler contains the standalone scheduler process composition.
package scheduler

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"velotrust/management/internal/config"
	db "velotrust/management/internal/repository"
	"velotrust/management/internal/schedule"
	"velotrust/pkg/ioc"
)

// Serve starts the scheduler. Applications register concrete schedule.Task
// implementations with the manager before calling Start.
func Serve(tasks ...schedule.Task) error {
	conf, err := ioc.GetObject[*config.Config]()
	if err != nil {
		return err
	}
	if err := db.Initialize(conf.Database.Driver, conf.Database.DSN, conf.Database.ShowSQL); err != nil {
		return err
	}
	defer db.Close()
	manager := schedule.NewScheduleManager(0)
	for _, task := range tasks {
		if err := manager.Register(task); err != nil {
			return err
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	manager.Start(ctx, 0)
	return nil
}
