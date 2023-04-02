package testutil

import (
	"context"

	sdk "github.com/cludden/concourse-go-sdk"
	"github.com/cludden/concourse-go-sdk/pkg/archive"
)

type (
	GetParams struct {
		Baz string `json:"baz"`
	}

	PutParams struct {
		Bar string `json:"bar"`
	}

	Source struct {
		Archive *archive.Config `json:"archive,omitempty"`
	}

	Version struct {
		Qux string `json:"qux"`
	}
)

type Resource interface {
	Check(context.Context, *Source, *Version) ([]Version, error)
	Close(context.Context) error
	In(context.Context, *Source, *Version, string, *GetParams) ([]sdk.Metadata, error)
	Out(context.Context, *Source, string, *PutParams) (Version, []sdk.Metadata, error)
	Archive(context.Context, *Source) (sdk.Archive, error)
	Initialize(context.Context, *Source) error
}
