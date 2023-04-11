package sdk

import (
	"context"
	"errors"
)

// BaseResource provides an embeddable Resource implementation with noops for optional methods
type BaseResource[Source any, Version any, GetParams any, PutParams any] struct{}

func (r *BaseResource[Source, Version, GetParams, PutParams]) Archive(ctx context.Context, s *Source) (Archive, error) {
	return nil, nil
}

func (r *BaseResource[Source, Version, GetParams, PutParams]) Close(ctx context.Context) error {
	return nil
}

func (r *BaseResource[Source, Version, GetParams, PutParams]) Initialize(ctx context.Context, s *Source) error {
	return nil
}

func (r *BaseResource[Source, Version, GetParams, PutParams]) Check(ctx context.Context, s *Source, v *Version) ([]Version, error) {
	return nil, errors.New("not implemented")
}

func (r *BaseResource[Source, Version, GetParams, PutParams]) In(ctx context.Context, s *Source, v *Version, path string, p *GetParams) ([]Metadata, error) {
	return nil, errors.New("not implemented")
}

func (r *BaseResource[Source, Version, GetParams, PutParams]) Out(ctx context.Context, s *Source, path string, p *GetParams) (Version, []Metadata, error) {
	var v Version
	return v, nil, errors.New("not implemented")
}
