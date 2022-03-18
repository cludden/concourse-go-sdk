package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"

	"github.com/hashicorp/go-multierror"
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
)

// action describes a concourse resource function
type action struct {
	method       string
	arguments    []argument
	requirePath  bool
	returnValues returnValue
}

var (
	// initAction describes an optional Initialize method that a resource can
	// implement to perform common resource bootstrapping logic before an action
	// is invoked
	initAction = &action{
		method: "Initialize",
		arguments: []argument{
			contextInput,
			sourceInput,
		},
	}

	// checkAction describes the required Check method that should check for new
	// resource versions, up to and including the last retrieved version if
	// provided, and return them in chronological order
	checkAction = &action{
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
	inAction = &action{
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

	// outAction describes the required Out method that perform any and all
	// logic requried to generate a new resource version, publish any and all
	// artifacts, and return the version along with any relevant metadata
	outAction = &action{
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

// Exec parses, validates, and executes an action
func (a *action) Exec(ctx context.Context, path string, method reflect.Value, req *Message) (resp interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	t, errs := method.Type(), multierror.Append(nil)

	if t.NumIn() != len(a.arguments) {
		return nil, fmt.Errorf("expected method to require %d arguments, got %d", len(a.arguments), t.NumIn())
	}

	var args []reflect.Value
	for i, arg := range a.arguments {
		at := t.In(i)
		switch arg {
		case contextInput:
			if !at.Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
				errs = multierror.Append(errs, fmt.Errorf("argument %d must be of type context.Context, got %s", i, at))
			}
			args = append(args, reflect.ValueOf(ctx))
		case sourceInput:
			if at.Kind() != reflect.Ptr || at.Elem().Kind() != reflect.Struct {
				errs = multierror.Append(errs, fmt.Errorf("argument %d must be pointer to source struct, got %s", i, at))
				break
			}
			args, err = validateArg(args, ctx, at, req.Source, true)
			if err != nil {
				errs = multierror.Append(errs, fmt.Errorf("error parsing source argument: %v", err))
			}
		case pathInput:
			if at.Kind() != reflect.String {
				errs = multierror.Append(errs, fmt.Errorf("argument %d must be path string", i))
				break
			}
			args = append(args, reflect.ValueOf(path))
		case versionInput:
			if at.Kind() != reflect.Ptr || at.Elem().Kind() != reflect.Struct {
				errs = multierror.Append(errs, fmt.Errorf("argument %d must be pointer to version struct, got %s", i, at))
				break
			}
			args, err = validateArg(args, ctx, at, req.Version, i != len(a.arguments)-1)
			if err != nil {
				errs = multierror.Append(errs, fmt.Errorf("error parsing version argument: %v", err))
			}
		case paramsInput:
			if at.Kind() != reflect.Ptr || at.Elem().Kind() != reflect.Struct {
				errs = multierror.Append(errs, fmt.Errorf("argument %d must be pointer to params struct, got %s", i, at))
			}
			args, err = validateArg(args, ctx, at, req.Params, i != len(a.arguments)-1)
			if err != nil {
				errs = multierror.Append(errs, fmt.Errorf("error parsing params argument: %v", err))
			}
		default:
			panic(fmt.Errorf("unsupported arg: %v", arg))
		}
	}

	switch a.returnValues {
	case errOutput:
		if t.NumOut() != 1 {
			errs = multierror.Append(errs, fmt.Errorf("requires 1 return value, got %d", t.NumOut()))
		}
	case versionOutput:
		if t.NumOut() != 3 {
			errs = multierror.Append(errs, fmt.Errorf("requires 3 return values, got %d", t.NumOut()))
		} else {
			if t.Out(0).Kind() != reflect.Ptr || t.Out(0).Elem().Kind() != reflect.Struct {
				errs = multierror.Append(errs, fmt.Errorf("first return value must be pointer to version struct, got %s", t.Out(0).Kind().String()))
			}
			if t.Out(1).Kind() != reflect.Slice || t.Out(1).Elem() != reflect.TypeOf(&Metadata{}).Elem() {
				errs = multierror.Append(errs, fmt.Errorf("second return value must be slice of metadata, got %s", t.Out(1).Kind()))
			}
		}
	case versionsOutput:
		if t.NumOut() != 2 {
			errs = multierror.Append(errs, fmt.Errorf("requires 2 return values, got %d", t.NumOut()))
		} else {
			if t.Out(0).Kind() != reflect.Slice {
				errs = multierror.Append(errs, fmt.Errorf("first return value must be slice of versions, got %s", t.Out(0).Kind().String()))
			} else if t.Out(0).Elem().Kind() != reflect.Struct {
				errs = multierror.Append(errs, fmt.Errorf("first return value must be slice of version structs, got %s", t.Out(0).Elem().Kind().String()))
			}
		}
	default:
		panic(fmt.Errorf("unsupported return values: %v", a.returnValues))
	}

	if t.NumOut() == 0 || !t.Out(t.NumOut()-1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		errs = multierror.Append(errs, fmt.Errorf("last return value must be of type error"))
	}

	if err := errs.ErrorOrNil(); err != nil {
		return nil, err
	}

	for i, arg := range a.arguments {
		if arg == versionInput {
			if a.returnValues == versionOutput || a.returnValues == versionsOutput {
				if t.In(i).Elem() != t.Out(0).Elem() {
					return nil, fmt.Errorf("version input and output must be same type")
				}
			}
		}
	}

	if a.requirePath {
		if stat, err := os.Stat(path); err != nil || !stat.IsDir() {
			return nil, fmt.Errorf("path must be valid directory")
		}
		if err := os.Chdir(path); err != nil {
			return nil, fmt.Errorf("error setting resource working directory: %v", err)
		}
	}

	result := method.Call(args)
	if err, ok := result[len(result)-1].Interface().(error); ok {
		return nil, err
	}

	switch a.returnValues {
	case versionsOutput:
		if result[0].IsNil() {
			return reflect.MakeSlice(reflect.SliceOf(result[0].Type()), 0, 0).Interface(), nil
		}
		return result[0].Interface(), nil
	case versionOutput:
		resp = Response{
			Version:  result[0].Interface(),
			Metadata: result[1].Interface().([]Metadata),
		}
		if result[0].IsNil() {
			return nil, fmt.Errorf("result missing required version")
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
func validateArg(args []reflect.Value, ctx context.Context, t reflect.Type, raw json.RawMessage, required bool) ([]reflect.Value, error) {
	if required && len(raw) == 0 {
		return args, fmt.Errorf("missing required input")
	}
	arg := reflect.New(t)
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, arg.Interface()); err != nil {
			return nil, fmt.Errorf("error unmarshalling input: %v", err)
		}
		if err := validate(ctx, arg); err != nil {
			return nil, fmt.Errorf("invalid input: %v", err)
		}
	}
	return append(args, arg.Elem()), nil
}
