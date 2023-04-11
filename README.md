# concourse-go-sdk
[![Go Reference](https://pkg.go.dev/badge/github.com/cludden/concourse-go-sdk.svg)](https://pkg.go.dev/github.com/cludden/concourse-go-sdk)  

a minimal SDK for implementing an idiomatic [Concourse](https://concourse-ci.org) [custom resource type](https://concourse-ci.org/implementing-resource-types.html) in go.



## Getting Started
1. Define 4 [required types](#required-types) as structs with appropriate json struct tags
2. Provide a [resource implementation](#resource-implementation) that leverages the types defined in step 1
3. Define a `func main() {}` that invokes this sdk's `Main` function with your resource and type definitions
4. Build a `check`, `in`, and `out` binary using [goreleaser](example/.goreleaser.yaml) or directly passing the appropriate linker flags to configure the build variable (e.g. `-ldflags="-X 'github.com/cludden/concourse-go-sdk.Operation={check,in,out}'"`)

*Example*
```go
package main

import (
	"context"
	"errors"

	sdk "github.com/cludden/concourse-go-sdk"
)

// 3. Invoke the Main function provided by this sdk
func main() {
	sdk.Main[Source, Version, GetParams, PutParams](&Resource{})
}

// 1. Define required type definitions
type (
	GetParams struct{}

	PutParams struct{}

	Source struct{}

	Version struct{}
)

// 2. Define resource type and corresponding methods
type Resource struct{
    // embed BaseResource to inherit noop implementations of all optional methods
    sdk.BaseResource[Source, Version, GetParams, PutParams]
}

// Check checks for new versions
func (r *Resource) Check(ctx context.Context, s *Source, v *Version) ([]Version, error) {
	return nil, errors.New("not implemented")
}

// In retrieves the specified version and writes it to the filesystem
func (r *Resource) In(ctx context.Context, s *Source, v *Version, dir string, p *GetParams) ([]sdk.Metadata, error) {
	return nil, errors.New("not implemented")
}

// Out creates a new version
func (r *Resource) Out(ctx context.Context, s *Source, dir string, p *PutParams) (*Version, []sdk.Metadata, error) {
	return nil, nil, errors.New("not implemented")
}
```



## Required Types
The various resource methods leverage a combination of 4 required types ([Source](#source), [Version](#version), [GetParams](#getparams), [PutParams](#putparams)), which should be implemented as [Go struct types](https://gobyexample.com/structs) with appropriate [struct tags](https://gobyexample.com/json) defined for accurate JSON decoding. Note that the names of these types are not important, but their position in the various method signatures *is*.

### `Source`
an arbitrary JSON object which specifies the runtime configuration of the resource, including any credentials. This is passed verbatim from the [resource configuration](https://concourse-ci.org/resources.html#schema.resource.source). For the `git` resource, this would include the repo URI, the branch, and the private key, if necessary.

*Example*
```go
// Source describes the available configuration for a git resource
type Source struct {
    URI           string   `json:"uri"`
    Branch        string   `json:"branch"`
    PrivateKey    string   `json:"private_key"`
    Paths         []string `json:"paths"`
    IgnorePaths   []string `json:"ignore_paths"`
    DisableCISkip bool     `json:"disable_ci_skip"`
}
```

### `Version`
a JSON object with string fields, used to uniquely identify an instance of the resource. For `git` this would be the commit's SHA.

*Example*
```go
// Version describes the attributes that uniquely identify a git resource version
type Version struct {
    Ref string `json:"ref"`
}
```

### `GetParams`
an arbitrary JSON object passed along verbatim from [get step params](https://concourse-ci.org/get-step.html#schema.get.params) on a get step.

*Example*
```go
// GetParams describes the available parameters for a git resource get step
type GetParams struct {
    Depth      int      `json:"depth"`
    FetchTags  bool     `json:"fetch_tags"`
    Submodules []string `json:"submodules"`
}
```

### `PutParams`
an arbitrary JSON object passed along verbatim from [put step params](https://concourse-ci.org/put-step.html#schema.put.params) on a put step.

*Example*
```go
// PutParams describes the available parameters for a git resource put step
type PutParams struct {
    Repository string `json:"repository"`
    Rebase     bool   `json:"rebase"`
    Merge      bool   `json:"merge"`
    Tag        string `json:"tag"`
    Force      bool   `json:"force"`
}
```

### Validation
Any of the above types can optionally choose to implement the `Validatable` interface shown below, in which case the sdk will perform runtime validation prior to invoking action methods.

```go
type Validatable interface {
    Validate(context.Context) error
}
```



## Resource Implementation
A `Resource` can be any struct that satisfies the following interface utilizing the [required types](#required-types) documented above. This package provides a `BaseResource` type that provides an embeddable `Resource` implementation with noops for all methods, allowing consumers to only provide implementations for desired functionality.

```go
// Resource describes a Concourse custom resource implementation
type Resource[Source any, Version any, GetParams any, PutParams any] interface {
    // Archive intializes an Archive implementation for persisting resource
    // version history outside of Concourse
    Archive(context.Context, *Source) (Archive, error)

    // Check checks for new versions
    Check(context.Context, *Source, *Version) ([]Version, error)

    // Close is called after any Check/In/Out operation
    Close(context.Context) error

    // In fetches the specified version and writes it to the filesystem
    In(context.Context, *Source, *Version, string, *GetParams) ([]Metadata, error)

    // Initialize is called prior to any Check/In/Out operation and provides
    // an opportunity to perform common resource initialization logic
    Initialize(context.Context, *Source) error

    // Out creates a new resource version
    Out(context.Context, *Source, string, *PutParams) (Version, []Metadata, error)
}
```

*Example*
```go
type (
	GetParams struct{}

	PutParams struct{}

	Source struct{}

	Version struct{
        Ref string `json:"ref"`
    }
)

type MyResource struct {
	sdk.BaseResource[Source, Version, GetParams, PutParams]
}

func (r *MyResource) Out(ctx context.Context, source *Source, path string, p *PutParams) (Version, []sdk.Metadata, error) {
	return Version{Ref: "foo"}, []sdk.Metadata{{Name: "bar", Value: "baz"}}, nil
}

```



## Archiving
In certain situations, Concourse can reset a particular resource's version history (e.g. when the source parameters change). Often times, this is undesirable. This sdk supports archiving resource version history as a workaround. To enable this functionality, a resource should implement an [Archive](#archive) method that initializes and returns a valid archive:

```go
type Archive interface {
    // Close should handle any graceful termination steps (e.g. closing open connections or file handles, persisting local data to a remote store, etc)
	Close(ctx context.Context) error
    // History returns an ordered list of json serialized versions
	History(ctx context.Context) ([][]byte, error)
    // Put appends an ordered list of versions to a resource's history, making sure to avoid duplicates
	Put(ctx context.Context, versions ...[]byte) error
}
```

This sdk also provides the following out-of-the-box archive implementations that can be utilized via:

```go
import (
    "github.com/cludden/concourse-go-sdk/pkg/archive"
)

type Source struct {
    Archive *archive.Config `json:"archive"`
}

func (r *Resource) Archive(ctx context.Context, s *Source) (archive.Archive, error) {
    if s != nil && s.Archive != nil {
        return archive.New(ctx, *s.Archive)
    }
    return nil, nil
}
```

### `boltdb`
an archive implementation that utilizes [boltdb](https://pkg.go.dev/github.com/boltdb/bolt) backed by [AWS S3](https://aws.amazon.com/s3/).



## License
Licensed under the [MIT License](LICENSE.md)  
Copyright (c) 2023 Chris Ludden