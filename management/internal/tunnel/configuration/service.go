// Package configuration coordinates the current expected configuration for a
// ZTNA target. Configuration writes replace the single current database row.
package configuration

import (
	"errors"

	servicecontext "velotrust/management/internal/context"
	model "velotrust/management/internal/model/ztna"
)

type Store interface {
	GetCurrentConfig(*servicecontext.Context, string, int64) (*model.Config, error)
	SaveCurrentConfig(*servicecontext.Context, *model.Config) error
}

type Service struct {
	store Store
}

func New(store Store) (*Service, error) {
	if store == nil {
		return nil, errors.New("current configuration store is required")
	}
	return &Service{store: store}, nil
}

func (s *Service) GetCurrent(ctx *servicecontext.Context, targetType string, targetID int64) (*model.Config, error) {
	if ctx == nil {
		return nil, errors.New("service context is required")
	}
	return s.store.GetCurrentConfig(ctx, targetType, targetID)
}

// ReplaceCurrent atomically replaces the current expected config. BaseModel's
// version_no is the concurrency marker and must be zero only for first creation.
func (s *Service) ReplaceCurrent(ctx *servicecontext.Context, config *model.Config) error {
	if ctx == nil || config == nil {
		return errors.New("service context and target configuration are required")
	}
	return s.store.SaveCurrentConfig(ctx, config)
}
