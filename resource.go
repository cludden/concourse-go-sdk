package sdk

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/fatih/color"
	"github.com/hashicorp/go-multierror"
	"github.com/tidwall/gjson"
)

type (
	// Archive describes a Concourse version archive
	Archive interface {
		Close(ctx context.Context) error
		History(ctx context.Context, latest []byte) ([][]byte, error)
		Put(ctx context.Context, versions ...[]byte) error
	}

	// Metadata describes resource version metadata, returned by get/put steps
	Metadata struct {
		Name  string
		Value string
	}

	// Op implements an enumeration of supported resource operations
	Op int

	// Resource describes a Concourse custom resource implementation
	Resource[Source any, Version any, GetParams any, PutParams any] interface {
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

	// Reponse describes a in/out response payload
	Response[Version any] struct {
		Version  Version    `json:"version"`
		Metadata []Metadata `json:"metadata"`
	}

	// Validatable describes an interface that can be implemented by
	// third party types to opt into automatic resource validation
	Validatable interface {
		Validate(context.Context) error
	}
)

// Operation describes the resource operation to perform, set via linker flags
var Operation = "check"

// Supported operations
const (
	invalidOp Op = iota
	CheckOp
	InOp
	OutOp
)

// Main executes a Concourse custom resource operation
func Main[Source any, Version any, GetParams any, PutParams any](r Resource[Source, Version, GetParams, PutParams]) {
	var op Op
	switch strings.TrimSpace(strings.ToLower(Operation)) {
	case "check":
		op = CheckOp
	case "in":
		op = InOp
	case "out":
		op = OutOp
	default:
		op = invalidOp
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	if err := Exec(ctx, op, r, os.Stdin, os.Stdout, os.Stderr, os.Args); err != nil {
		color.New(color.FgRed).Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Exec implements a shared entrypoint for all supported resource operations
// and handles parsing and validating resource configuration and initializing
// the resource if implemented
func Exec[Source any, Version any, GetParams any, PutParams any](
	ctx context.Context,
	op Op,
	r Resource[Source, Version, GetParams, PutParams],
	stdin io.Reader,
	stdout, stderr io.Writer,
	args []string,
) (err error) {
	// blah, configure global color settings
	color.NoColor = false
	color.Output = stderr

	// inject reference to stderr into context
	ctx = ContextWithStdErr(ctx, stderr)

	// validate path
	var path string
	if op == InOp || op == OutOp {
		if len(args) < 2 {
			return fmt.Errorf("invalid operation: path argument required")
		}
		path = args[1]
		if err := os.Chdir(path); err != nil {
			return fmt.Errorf("error changing to build working directory: %w", err)
		}
	}

	// parse input payload
	payload, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("error reading input: %v", err)
	}

	if !gjson.ValidBytes(payload) {
		return fmt.Errorf("error reading input: invalid json")
	}

	req, errs := gjson.ParseBytes(payload), multierror.Append(nil)

	// parse source
	var source *Source
	if x := req.Get("source"); x.Exists() && x.Type != gjson.Null {
		var s Source
		if err := json.Unmarshal([]byte(x.Raw), &s); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("error parsing source: %w", err))
		} else {
			source = &s
		}
	}

	// validate source
	if source != nil {
		if v, ok := interface{}(source).(Validatable); ok {
			if err := v.Validate(ctx); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("invalid source: %w", err))
			}
		}
	}

	// parse version
	var version *Version
	if x := req.Get("version"); x.Exists() && x.Type != gjson.Null {
		var v Version
		if err := json.Unmarshal([]byte(x.Raw), &v); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("error parsing version: %w", err))
		} else {
			version = &v
		}
	}

	// validate version
	if version != nil {
		if v, ok := interface{}(version).(Validatable); ok {
			if err := v.Validate(ctx); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("invalid version: %w", err))
			}
		}
	}

	if errs.Len() > 0 {
		return errs.ErrorOrNil()
	}

	// call Initialize method if defined
	if err := r.Initialize(ctx, source); err != nil {
		return fmt.Errorf("error initializing resource: %w", err)
	}
	defer func() {
		if err := r.Close(ctx); err != nil {
			color.Red("error closing resource: %v", err)
		}
	}()

	// initialize archive
	var archiver Archive
	if op == CheckOp || op == OutOp {
		archiver, err = r.Archive(ctx, source)
		if err != nil {
			return fmt.Errorf("error initializing archive: %w", err)
		}
		if archiver != nil {
			defer func() {
				if err := archiver.Close(ctx); err != nil {
					color.Red("error closing archive: %v", err)
				}
			}()
		}
	}

	// execute Step
	var resp any
	switch op {
	case CheckOp:
		resp, err = check(ctx, r, archiver, source, version)
	case InOp:
		resp, err = in(ctx, r, source, version, path, req.Get("params"))
	case OutOp:
		resp, err = out(ctx, r, archiver, source, path, req.Get("params"))
	}
	if err != nil {
		return err
	}

	if err := json.NewEncoder(stdout).Encode(resp); err != nil {
		return fmt.Errorf("error writing response: %v", err)
	}

	return nil
}

// check executs a Check operation on the provided resource
func check[S any, V any, G any, P any](ctx context.Context, r Resource[S, V, G, P], archiver Archive, source *S, version *V) ([]V, error) {
	// attempt to populate latest version for check operations if no existing version provided
	// and archive is configured
	var history [][]byte
	var historyLength int
	var err error
	if archiver != nil {
		color.Yellow("fetching archived resource version history...")

		var latest []byte
		if version != nil {
			latest, err = json.Marshal(version)
			if err != nil {
				return nil, fmt.Errorf("error fetching archive history: error serializing latest version: %w", err)
			}
		}

		history, err = archiver.History(ctx, latest)
		if err != nil {
			return nil, fmt.Errorf("error hydrating archived version history: %w", err)
		}
		historyLength = len(history)

		if historyLength > 0 && version == nil {
			color.Yellow("using existing resource version from version history...")
			historyLatest := history[len(history)-1]
			var v V
			if err := json.Unmarshal(historyLatest, &v); err != nil {
				return nil, fmt.Errorf("error parsing history version: %w", err)
			} else {
				version = &v
			}
		}
	}

	// initialize check versions
	var versions []V

	// append any archived history retrieved earlier in the operation to list of versions
	// keep track of versions seen
	archived := make(map[string]struct{}, historyLength)
	for _, version := range history {
		sum := md5.Sum(version)
		var v V
		if err := json.Unmarshal(version, &v); err != nil {
			return nil, fmt.Errorf("error parsing archived resource version: %v", err)
		}
		versions = append(versions, v)
		archived[string(sum[:])] = struct{}{}
	}

	// execute Check operation
	newVersions, err := r.Check(ctx, source, version)
	if err != nil {
		return nil, err
	}

	// add returned versions to the result if not present in history
	var unarchived [][]byte
	for _, version := range newVersions {
		serialized, err := json.Marshal(&version)
		if err != nil {
			return nil, fmt.Errorf("error serializing version for archival: %v", err)
		}
		sum := md5.Sum(serialized)
		if _, seen := archived[string(sum[:])]; !seen {
			versions = append(versions, version)
			// keep track of new versions in order to archive
			if archiver != nil {
				unarchived = append(unarchived, serialized)
			}
		}
	}

	// archive new versions emitted by check operations
	if archiver != nil && len(unarchived) > 0 {
		if err := archiver.Put(ctx, unarchived...); err != nil {
			return nil, fmt.Errorf("error archiving new versions: %v", err)
		}
	}
	return versions, nil
}

// in executes an In operation on the provided resource
func in[S any, V any, G any, P any](ctx context.Context, r Resource[S, V, G, P], source *S, version *V, path string, getParams gjson.Result) (*Response[V], error) {
	errs := multierror.Append(nil)

	// verify version is not nil
	if version == nil {
		errs = multierror.Append(errs, errors.New("version required"))
	}

	// parse params
	var params *G
	if getParams.Exists() && getParams.Type != gjson.Null {
		var p G
		if err := json.Unmarshal([]byte(getParams.Raw), &p); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("error parsing get parameters: %w", err))
		} else {
			params = &p
		}
	}

	// validate params
	if params != nil {
		if v, ok := interface{}(params).(Validatable); ok {
			if err := v.Validate(ctx); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("invalid get parameters: %w", err))
			}
		}
	}

	if errs.Len() > 0 {
		return nil, errs.ErrorOrNil()
	}

	// execute In
	meta, err := r.In(ctx, source, version, path, params)
	if err != nil {
		return nil, err
	}
	return &Response[V]{
		Version:  *version,
		Metadata: meta,
	}, nil
}

// out executes an Out operation on the provided resource
func out[S any, V any, G any, P any](ctx context.Context, r Resource[S, V, G, P], archiver Archive, source *S, path string, putParams gjson.Result) (*Response[V], error) {
	errs := multierror.Append(nil)

	// parse params
	var params *P
	if putParams.Exists() && putParams.Type != gjson.Null {
		var p P
		if err := json.Unmarshal([]byte(putParams.Raw), &p); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("error parsing get parameters: %w", err))
		} else {
			params = &p
		}
	}

	// validate params
	if params != nil {
		if v, ok := interface{}(params).(Validatable); ok {
			if err := v.Validate(ctx); err != nil {
				errs = multierror.Append(errs, fmt.Errorf("invalid get parameters: %w", err))
			}
		}
	}

	if errs.Len() > 0 {
		return nil, errs.ErrorOrNil()
	}

	// execute In
	version, meta, err := r.Out(ctx, source, path, params)
	if err != nil {
		return nil, err
	}

	// archive new versions emitted by out operations
	if archiver != nil {
		color.Yellow("archiving new version...")
		serialized, err := json.Marshal(version)
		if err != nil {
			return nil, fmt.Errorf("error serializing version for archival: %v", err)
		}
		if err := archiver.Put(ctx, serialized); err != nil {
			color.Red("error archiving new version: %v", err)
			return nil, fmt.Errorf("error archiving new version: %v", err)
		}
	}

	return &Response[V]{
		Version:  version,
		Metadata: meta,
	}, nil
}
