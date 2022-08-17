package archive

import (
	"context"
	"fmt"

	"github.com/cludden/concourse-go-sdk/pkg/archive/boltdb"
	"github.com/cludden/concourse-go-sdk/pkg/archive/inmem"
	"github.com/cludden/concourse-go-sdk/pkg/archive/settings"
	"github.com/go-playground/validator/v10"
)

type Config struct {
	settings.Settings `json:",inline" validate:"dive"`
	BoltDB            *boltdb.Config `json:"boltdb" validate:"omitempty"`
	Inmem             *inmem.Config  `json:"inmem" validate:"omitempty"`
}

type Archive interface {
	Close(ctx context.Context) error
	History(ctx context.Context, latest []byte) ([][]byte, error)
	Put(ctx context.Context, versions ...[]byte) error
}

func New(ctx context.Context, cfg Config) (Archive, error) {
	if err := validator.New().StructCtx(ctx, &cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %v", err)
	}

	switch {
	case cfg.BoltDB != nil:
		return boltdb.New(ctx, *cfg.BoltDB, &cfg.Settings)
	case cfg.Inmem != nil:
		return inmem.New(ctx, *cfg.Inmem, &cfg.Settings)
	default:
		return nil, fmt.Errorf("no valid provider config found")
	}
}
