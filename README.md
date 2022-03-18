# concourse-go-sdk
[![Go Reference](https://pkg.go.dev/badge/github.com/cludden/concourse-go-sdk.svg)](https://pkg.go.dev/github.com/cludden/concourse-go-sdk)  

a minimal SDK for implementing an idiomatic [Concourse](https://concourse-ci.org) [custom resource type](https://concourse-ci.org/implementing-resource-types.html) in go.

## Getting Started
1. Define 4 [required types](#required-types) as structs with appropriate json struct tags
2. Define resource struct type with 3 [required methods](#required-methods) that leverage a combination of the types defined in step 1. 
3. Define a `func main() {}` that invokes this sdk's `Main` function, passing a pointer to your resource value
4. Build a `check`, `in`, and `out` binary using [goreleaser](example/.goreleaser.yaml) or directly passing the appropriate linker flags to configure the build variable (e.g. `-ldflags="-X 'github.com/cludden/concourse-go-sdk.Operation={check,in,out}'"`)

*Example*
```go
package main

import (
    concourse "github.com/cludden/concourse-go-sdk"
)

// 1. Define required type definitions
type (
    GetParams struct {}

    PutParams struct {}

    Source struct {}

    Version struct {}
)

// 2. Define resource type and corresponding methods
type Resource struct {}

func (r *Resource) Initialize(context.Context, *Source) error {}

func (r *Resource) Check(context.Context, *Source, *Version) ([]Version, error) {}

func (r *Resource) In(context.Context, *Source, *Version, string, *GetParams) (*Version, []concourse.Metadata, error) {}

func (r *Resource) Out(context.Context, *Source, string, *PutParams) (*Version, []concourse.Metadata, error) {}

// 3. Invoke the Main function provided by this sdk
func main() {
    concourse.Main(&Resource{})
}
```

## Required Types
The various [required methods](#required-methods) leverage a combination of 4 required types ([Source](#source), [Version](#version), [GetParams](#getparams), [PutParams](#putparams)), which should be implemented as [Go struct types](https://gobyexample.com/structs) with appropriate [struct tags](https://gobyexample.com/json) defined for accurate JSON decoding. Note that the names of these types are not important, but their position in the various method signatures *is*.

### `Source`
an arbitrary JSON object which specifies the runtime configuration of the resource, including any credentials. This is passed verbatim from the [resource configuration](https://concourse-ci.org/resources.html#schema.resource.source). For the `git` resource, this would include the repo URI, the branch, and the private key, if necessary.

*Example*
```go
// Source describes the available configuration for a git resource
type Source struct {
    URI           string   `json:"uri" validate:"required,uri"`
    Branch        string   `json:"branch"`
    PrivateKey    string   `json:"private_key" validate:"file"`
    Paths         []string `json:"paths" validate:"file"`
    IgnorePaths   []string `json:"ignore_paths" validate:"file"`
    DisableCISkip bool     `json:"disable_ci_skip"`
}

func (s *Source) Validate(ctx context.Context) error {
    return validator.New().StructContext(ctx, s)
}
```

### `Version`
a JSON object with string fields, used to uniquely identify an instance of the resource. For `git` this would be the commit's SHA.

*Example*
```go
// Version describes the attributes that uniquely identify a git resource version
type Version struct {
    Ref string `json:"ref" validate:"required,hexadecimal,len=6"`
}

func (v *Version) Validate(ctx context.Context) error {
    return validator.New().StructContext(ctx, v)
}
```

### `GetParams`
an arbitrary JSON object passed along verbatim from [get step params](https://concourse-ci.org/get-step.html#schema.get.params) on a get step.

*Example*
```go
// GetParams describes the available parameters for a git resource get step
type GetParams struct {
    Depth      int      `json:"depth" validate:"min=0,max=127"`
    FetchTags  bool     `json:"fetch_tags"`
    Submodules []string `json:"submodules"`
}

func (p *GetParams) Validate(ctx context.Context) error {
    return validator.New().StructContext(ctx, p)
}
```

### `PutParams`
an arbitrary JSON object passed along verbatim from [put step params](https://concourse-ci.org/put-step.html#schema.put.params) on a put step.

*Example*
```go
// PutParams describes the available parameters for a git resource put step
type PutParams struct {
    Repository string `json:"repository" validate:"required,file"`
    Rebase     bool   `json:"rebase"`
    Merge      bool   `json:"merge"`
    Tag        string `json:"tag" validate:"file"`
    Force      bool   `json:"force"`
}

func (p *PutParams) Validate(ctx context.Context) error {
    return validator.New().StructContext(ctx, p)
}
```

### Validation
Any of the above types can optionally choose to implement the `Validatable` interface shown below, in which case the sdk will perform runtime validation prior to invoking action methods.

```go
type Validatable interface {
    Validate(context.Context) error
}
```

## Required Methods
A resource must implement a [Check](#check), [In](#in), and [Out](#out) method with the correct signature. A resource can optionally implement an [Initialize](#initialize) method which will be invoked anytime a resource container is launched, prior any action method. Note that unlike the [required types](#required-types), the names of these methods *are* important, and the resource will fail if a required method is not implemented.

### `Initialize`
An *optional* method for performing common initialization logic.

```go
func (r *Resource) Initialize(context.Context, *Source) error {}
```

### `Check`
[Check for new versions](https://concourse-ci.org/implementing-resource-types.html#resource-check)

```go
func (r *Resource) Check(ctx context.Context, s *Source, v *Version) ([]Version, error) {}
```

### `In`
[Fetch a given resource](https://concourse-ci.org/implementing-resource-types.html#resource-in)

```go
func (r *Resource) In(ctx context.Context, s *Source, v *Version, dir string, p *GetParams) (*Version, []concourse.Metadata, error) {}
```

### `Out`
[Update a resource](https://concourse-ci.org/implementing-resource-types.html#resource-out)

```go
func (r *Resource) Out(ctx context.Context, s *Source, dir string, p *GetParams) (*Version, []concourse.Metadata, error) {}
```

## License
Licensed under the [MIT License](LICENSE.md)  
Copyright (c) 2022 Chris Ludden