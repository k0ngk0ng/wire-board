package production

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sync"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type Registry struct {
	Modules []gameapi.Module

	close     func(context.Context) error
	closeOnce sync.Once
	closeErr  error
}

func Open(ctx context.Context, files fs.FS) (*Registry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	modules, err := Modules()
	if err != nil {
		return nil, err
	}
	if files == nil {
		return nil, errors.New("initialize pal: missing rules filesystem")
	}
	bundle, err := pal.OpenProductionBundle(ctx, files)
	if err != nil {
		return nil, fmt.Errorf("initialize pal: %w", err)
	}
	ready := false
	defer func() {
		if !ready {
			bundle.Close(context.Background())
		}
	}()
	module := bundle.Module
	if module.Controls == nil {
		return nil, errors.New("initialize pal: missing controller definition")
	}
	if err := module.Validate(); err != nil {
		return nil, fmt.Errorf("initialize pal: %w", err)
	}
	modules = append(modules, module)
	seen := map[string]bool{}
	for _, item := range modules {
		if seen[item.ID] {
			return nil, fmt.Errorf("duplicate production game %s", item.ID)
		}
		seen[item.ID] = true
	}
	ready = true
	return &Registry{Modules: modules, close: bundle.Close}, nil
}

func (registry *Registry) Close(ctx context.Context) error {
	if registry == nil || registry.close == nil {
		return nil
	}
	registry.closeOnce.Do(func() {
		registry.closeErr = registry.close(ctx)
	})
	return registry.closeErr
}
