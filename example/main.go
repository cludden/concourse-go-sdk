package main

import (
	"context"
	"fmt"

	concourse "github.com/cludden/concourse-go-sdk"
)

func main() {
	concourse.Main(&Resource{})
}

// =============================================================================

type (
	InParams struct {
		Shallow bool `json:"shallow"`
	}

	OutParams struct {
		Force bool `json:"bool"`
	}

	Source struct {
		Color string `json:"color"`
	}

	Version struct {
		Ref string `json:"ref"`
	}
)

func (s *Source) Validate(context.Context) error {
	switch s.Color {
	case "blue", "green":
		return nil
	default:
		return fmt.Errorf("color must be one of blue, green: got %s", s.Color)
	}
}

func (v *Version) Validate(context.Context) error {
	if v.Ref == "" {
		return fmt.Errorf("ref is required")
	}
	return nil
}

// =============================================================================

type Resource struct{}

func (r *Resource) Initialize(ctx context.Context, source *Source) (err error) {
	return nil
}

func (r *Resource) Check(ctx context.Context, source *Source, v *Version) ([]Version, error) {
	if v == nil {
		return []Version{}, nil
	}
	return []Version{*v}, nil
}

func (r *Resource) In(ctx context.Context, source *Source, v *Version, path string, p *InParams) (*Version, []concourse.Metadata, error) {
	return &Version{Ref: "foo"}, []concourse.Metadata{{Name: "bar", Value: "baz"}}, nil
}

func (r *Resource) Out(ctx context.Context, source *Source, path string, p *OutParams) (*Version, []concourse.Metadata, error) {
	return &Version{Ref: "foo"}, []concourse.Metadata{{Name: "bar", Value: "baz"}}, nil
}
