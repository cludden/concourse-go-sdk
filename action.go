package sdk

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/cludden/concourse-go-sdk/pkg/archive"
	"github.com/fatih/color"
	"github.com/hashicorp/go-multierror"
	"github.com/tidwall/gjson"
)

// argument enum describing the supported concourse action input values
type argument int

const (
	contextInput argument = iota
	sourceInput
	versionInput
	pathInput
	paramsInput
)

// returnValue enum describing the supported concourse action return values
type returnValue int

const (
	errOutput returnValue = iota
	versionOutput
	versionsOutput
	metadataOnlyOutput
	archiveOutput
)

// Action describes a concourse resource function
type Action struct {
	method       string
	arguments    []argument
	requirePath  bool
	returnValues returnValue
}

var (
	// archiveAction describes an optional Archive method that a resource can
	// implement to opt in to resource version archiving
	archiveAction = &Action{
		method: "Archive",
		arguments: []argument{
			contextInput,
			sourceInput,
		},
		returnValues: archiveOutput,
	}

	// initAction describes an optional Initialize method that a resource can
	// implement to perform common resource bootstrapping logic before an action
	// is invoked
	initAction = &Action{
		method: "Initialize",
		arguments: []argument{
			contextInput,
			sourceInput,
		},
	}

	// checkAction describes the required Check method that should check for new
	// resource versions, up to and including the last retrieved version if
	// provided, and return them in chronological order
	checkAction = &Action{
		method: "Check",
		arguments: []argument{
			contextInput,
			sourceInput,
			versionInput,
		},
		returnValues: versionsOutput,
	}

	// inAction describes the required In method that should retrieve the
	// specified resource version, write it the specified directory, and
	// return the version along with any relevant metadata
	inAction = &Action{
		method: "In",
		arguments: []argument{
			contextInput,
			sourceInput,
			versionInput,
			pathInput,
			paramsInput,
		},
		requirePath:  true,
		returnValues: versionOutput,
	}

	// inMetadataOnlyAction describes the required In method that should retrieve the
	// specified resource version, write it the specified directory, and
	// returns only any relevant metadata
	inMetadataOnlyAction = &Action{
		method: "In",
		arguments: []argument{
			contextInput,
			sourceInput,
			versionInput,
			pathInput,
			paramsInput,
		},
		requirePath:  true,
		returnValues: metadataOnlyOutput,
	}

	// outAction describes the required Out method that perform any and all
	// logic requried to generate a new resource version, publish any and all
	// artifacts, and return the version along with any relevant metadata
	outAction = &Action{
		method: "Out",
		arguments: []argument{
			contextInput,
			sourceInput,
			pathInput,
			paramsInput,
		},
		requirePath:  true,
		returnValues: versionOutput,
	}
)

func Initialize() *Action {
	return initAction
}

func Check() *Action {
	return checkAction
}

func In() *Action {
	return inAction
}

func InMetadataOnly() *Action {
	return inMetadataOnlyAction
}

func Out() *Action {
	return outAction
}

// Exec parses, validates, and executes an action
func (action *Action) Exec(ctx context.Context, path string, resource reflect.Value, req gjson.Result) (resp interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	method := resource.MethodByName(action.method)
	if !method.IsValid() {
		return nil, fmt.Errorf("resource is missing required method: %s", action.method)
	}

	if action.requirePath {
		if stat, err := os.Stat(path); err != nil || !stat.IsDir() {
			return nil, fmt.Errorf("path must be valid directory")
		}
		if err := os.Chdir(path); err != nil {
			return nil, fmt.Errorf("error setting resource working directory: %v", err)
		}
	}

	var archiver archive.Archive
	if impl := resource.MethodByName("Archive"); impl.IsValid() && (action.method == checkAction.method || action.method == outAction.method) {
		raw, err := archiveAction.Exec(ctx, path, resource, req)
		if err != nil {
			return nil, fmt.Errorf("error initializing archive: %v", err)
		}
		if raw != nil {
			archiver = raw.(archive.Archive)
			defer func() {
				if err := archiver.Close(ctx); err != nil {
					color.Red("error closing archive: %v", err)
				}
			}()
		}
	}

	return action.exec(ctx, path, method, req, archiver)
}

func (action *Action) exec(ctx context.Context, path string, method reflect.Value, req gjson.Result, archiver archive.Archive) (resp any, err error) {
	args, err := action.validateArgs(ctx, method.Type(), req, path)
	if err != nil {
		return nil, err
	}

	// attempt to populate latest version for check operations if no existing version provided
	// and archive is configured
	var history [][]byte
	var historyLength int
	var latest []byte
	if action.method == checkAction.method && archiver != nil {
		color.Yellow("fetching archived resource version history...")

		hasLatest := !args[2].IsNil()
		if hasLatest {
			latest, err = json.Marshal(args[2].Interface())
			if err != nil {
				return nil, fmt.Errorf("error fetching archive history: error serializing latest version: %v", err)
			}
		}

		history, err = archiver.History(ctx, latest)
		if err != nil {
			return nil, fmt.Errorf("error hydrating archived version history: %v", err)
		}
		historyLength = len(history)

		if historyLength > 0 && !hasLatest {
			color.Yellow("using existing resource version from version history...")
			historyLatest := history[len(history)-1]
			arg, err := validateArg(ctx, args[2].Type(), gjson.ParseBytes(historyLatest), true)
			if err != nil {
				return nil, fmt.Errorf("error parsing archived version history: %v", err)
			}
			args[2], latest = arg, historyLatest
		}
	}

	results := method.Call(args)
	if err, ok := results[len(results)-1].Interface().(error); ok && err != nil {
		return nil, err
	}

	switch action.returnValues {
	case archiveOutput:
		if results[0].IsNil() {
			return nil, nil
		}
		a, ok := results[0].Interface().(archive.Archive)
		if !ok {
			return nil, fmt.Errorf("expected return value to be archive.Archive, got: %s", results[0].Type().String())
		}
		return a, nil
	case metadataOnlyOutput:
		var version interface{}
		for i, arg := range action.arguments {
			if arg == versionInput {
				version = args[i].Interface()
				break
			}
		}
		if version == nil {
			return nil, fmt.Errorf("result missing required version")
		}

		resp = Response{
			Version:  version,
			Metadata: results[0].Interface().([]Metadata),
		}

		return resp, nil
	case versionsOutput:
		if results[0].IsNil() {
			return reflect.MakeSlice(reflect.SliceOf(results[0].Type().Elem()), 0, 0).Interface(), nil
		}

		// define versions result
		versions := reflect.MakeSlice(reflect.SliceOf(results[0].Type().Elem()), 0, 0)

		// append any existing history retrieved earlier in the operation to list of versions
		// keep track of versions seen
		archived := make(map[string]struct{}, historyLength)
		for i := 0; i < len(history); i++ {
			serialized := history[i]
			sum := md5.Sum(serialized)
			parsed, err := validateArg(ctx, args[2].Elem().Type(), gjson.ParseBytes(serialized), true)
			if err != nil {
				return nil, fmt.Errorf("error parsing archived resource version: %v", err)
			}
			versions = reflect.Append(versions, parsed)
			archived[string(sum[:])] = struct{}{}
		}

		// define set of unarchived versions
		var unarchived [][]byte

		// add returned versions to the result if not present in history
		for i := 0; i < results[0].Len(); i++ {
			version := results[0].Index(i)
			serialized, err := json.Marshal(version.Addr().Interface())
			if err != nil {
				return nil, fmt.Errorf("error serializing version for archival: %v", err)
			}
			sum := md5.Sum(serialized)
			if _, seen := archived[string(sum[:])]; !seen {
				versions = reflect.Append(versions, version)
				// if archive is configured, serialize version, and add any new versions to unarchived set
				if action.method == checkAction.method && archiver != nil {
					unarchived = append(unarchived, serialized)
				}
			}
		}

		// archive new versions emitted by check operations
		if len(unarchived) > 0 {
			if err := archiver.Put(ctx, unarchived...); err != nil {
				return nil, fmt.Errorf("error archiving new versions: %v", err)
			}
		}

		return versions.Interface(), nil
	case versionOutput:
		version := results[0].Interface()
		if version == nil {
			return nil, fmt.Errorf("result missing required version")
		}

		// archive new versions emitted by out operations
		if archiver != nil && action.method == outAction.method {
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

		resp = Response{
			Version:  version,
			Metadata: results[1].Interface().([]Metadata),
		}

		return resp, nil
	default:
		return nil, nil
	}
}

// validate invokes an optional Validate method on various input values
// if implemented by the provider
func validate(ctx context.Context, ptr reflect.Value) error {
	if ptr.Elem().Type().Implements(reflect.TypeOf((*Validatable)(nil)).Elem()) {
		if err := ptr.Elem().Interface().(Validatable).Validate(ctx); err != nil {
			return err
		}
	}
	return nil
}

// validateArg initializes a new value of the specified type, unmarshals the
// raw payload, and performs optional validation if implemented by the consumer
func validateArg(ctx context.Context, t reflect.Type, v gjson.Result, required bool) (reflect.Value, error) {
	if required && v.Type == gjson.Null {
		return reflect.Zero(t), fmt.Errorf("missing required input")
	}
	arg := reflect.New(t)
	if v.IsObject() {
		if err := json.Unmarshal([]byte(v.Raw), arg.Interface()); err != nil {
			return reflect.Zero(t), fmt.Errorf("error unmarshalling input: %v", err)
		}
	}
	if err := validate(ctx, arg); err != nil {
		return reflect.Zero(t), fmt.Errorf("invalid input: %v", err)
	}
	return arg.Elem(), nil
}

// validateArgs parses and validates method inputs and outputs
func (action *Action) validateArgs(ctx context.Context, signature reflect.Type, req gjson.Result, path string) (args []reflect.Value, err error) {
	if signature.NumIn() != len(action.arguments) {
		return nil, fmt.Errorf("expected method to require %d arguments, got %d", len(action.arguments), signature.NumIn())
	}

	errs := multierror.Append(nil)

	for i, arg := range action.arguments {
		parameter := signature.In(i)
		switch arg {
		case contextInput:
			if !parameter.Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
				errs = multierror.Append(errs, fmt.Errorf("argument %d must be of type context.Context, got %s", i, parameter))
			}
			args = append(args, reflect.ValueOf(ctx))
		case sourceInput:
			if parameter.Kind() != reflect.Ptr || parameter.Elem().Kind() != reflect.Struct {
				errs = multierror.Append(errs, fmt.Errorf("argument %d must be pointer to source struct, got %s", i, parameter))
				break
			}
			arg, err := validateArg(ctx, parameter, req.Get("source"), false)
			if err != nil {
				errs = multierror.Append(errs, fmt.Errorf("error parsing source argument: %v", err))
			}
			args = append(args, arg)
		case pathInput:
			if parameter.Kind() != reflect.String {
				errs = multierror.Append(errs, fmt.Errorf("argument %d must be path string", i))
				break
			}
			args = append(args, reflect.ValueOf(path))
		case versionInput:
			if parameter.Kind() != reflect.Ptr || parameter.Elem().Kind() != reflect.Struct {
				errs = multierror.Append(errs, fmt.Errorf("argument %d must be pointer to version struct, got %s", i, parameter))
				break
			}
			arg, err := validateArg(ctx, parameter, req.Get("version"), i != len(action.arguments)-1)
			if err != nil {
				errs = multierror.Append(errs, fmt.Errorf("error parsing version argument: %v", err))
			}
			args = append(args, arg)
		case paramsInput:
			if parameter.Kind() != reflect.Ptr || parameter.Elem().Kind() != reflect.Struct {
				errs = multierror.Append(errs, fmt.Errorf("argument %d must be pointer to params struct, got %s", i, parameter))
			}
			arg, err := validateArg(ctx, parameter, req.Get("params"), false)
			if err != nil {
				errs = multierror.Append(errs, fmt.Errorf("error parsing params argument: %v", err))
			}
			args = append(args, arg)
		default:
			panic(fmt.Errorf("unsupported arg: %v", arg))
		}
	}

	switch action.returnValues {
	case errOutput:
		if signature.NumOut() != 1 {
			errs = multierror.Append(errs, fmt.Errorf("requires 1 return value, got %d", signature.NumOut()))
		}
	case archiveOutput:
		if signature.NumOut() != 2 {
			errs = multierror.Append(errs, fmt.Errorf("requires 2 return values, got %d", signature.NumOut()))
		} else {
			if signature.Out(0) != reflect.TypeOf((*archive.Archive)(nil)).Elem() {
				errs = multierror.Append(errs, fmt.Errorf("first return value must be %s, got %s", reflect.TypeOf((*archive.Archive)(nil)).Elem(), signature.Out(0).String()))
			}
		}
	case metadataOnlyOutput:
		if signature.NumOut() != 2 {
			errs = multierror.Append(errs, fmt.Errorf("requires 2 return values, got %d", signature.NumOut()))
		} else {
			if signature.Out(0).Kind() != reflect.Slice || signature.Out(0).Elem() != reflect.TypeOf(&Metadata{}).Elem() {
				errs = multierror.Append(errs, fmt.Errorf("second return value must be slice of metadata, got %s", signature.Out(0).Kind()))
			}
		}
	case versionOutput:
		if signature.NumOut() != 3 {
			errs = multierror.Append(errs, fmt.Errorf("requires 3 return values, got %d", signature.NumOut()))
		} else {
			if signature.Out(0).Kind() != reflect.Ptr || signature.Out(0).Elem().Kind() != reflect.Struct {
				errs = multierror.Append(errs, fmt.Errorf("first return value must be pointer to version struct, got %s", signature.Out(0).Kind().String()))
			}
			if signature.Out(1).Kind() != reflect.Slice || signature.Out(1).Elem() != reflect.TypeOf(&Metadata{}).Elem() {
				errs = multierror.Append(errs, fmt.Errorf("second return value must be slice of metadata, got %s", signature.Out(1).Kind()))
			}
		}
	case versionsOutput:
		if signature.NumOut() != 2 {
			errs = multierror.Append(errs, fmt.Errorf("requires 2 return values, got %d", signature.NumOut()))
		} else {
			if signature.Out(0).Kind() != reflect.Slice {
				errs = multierror.Append(errs, fmt.Errorf("first return value must be slice of versions, got %s", signature.Out(0).Kind().String()))
			} else if signature.Out(0).Elem().Kind() != reflect.Struct {
				errs = multierror.Append(errs, fmt.Errorf("first return value must be slice of version structs, got %s", signature.Out(0).Elem().Kind().String()))
			}
		}
	default:
		panic(fmt.Errorf("unsupported return values: %v", action.returnValues))
	}

	if signature.NumOut() == 0 || !signature.Out(signature.NumOut()-1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		errs = multierror.Append(errs, fmt.Errorf("last return value must be of type error"))
	}

	if err := errs.ErrorOrNil(); err != nil {
		return nil, err
	}

	for i, arg := range action.arguments {
		if arg == versionInput {
			if action.returnValues == versionOutput || action.returnValues == versionsOutput {
				if signature.In(i).Elem() != signature.Out(0).Elem() {
					return nil, fmt.Errorf("version input and output must be same type")
				}
			}
		}
	}

	return args, nil
}
