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

type Archivable interface {
	Archive(context.Context, *Source) (archive.Archive, error)
}

type Resource interface {
	Check(context.Context, *Source, *Version) ([]Version, error)
	In(context.Context, *Source, *Version, string, *GetParams) (*Version, []sdk.Metadata, error)
	Out(context.Context, *Source, string, *PutParams) (*Version, []sdk.Metadata, error)
}

type ResourceInit interface {
	Resource
	Initialize(context.Context, *Source) error
}

type ResourceArchive interface {
	Resource
	Archivable
}

type ResourceInitArchive interface {
	ResourceInit
	Archivable
}
