package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/fatih/color"
	"github.com/oklog/run"
)

var (
	// Operation defines the resource operation to be invoked at runtime
	// Set this using build flags (e.g. go build -ldflags="-X 'github.com/cludden/concourse-go-sdk.Operation=check'")
	Operation string = "check"
)

type (
	// Message describes an input payload to a resource operation
	Message struct {
		Params  json.RawMessage `json:"params"`
		Source  json.RawMessage `json:"source"`
		Version json.RawMessage `json:"version"`
	}

	// Metadata describes a key value pair associated with a resource version
	Metadata struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}

	// Response describes an output payload from an in or out operation
	Response struct {
		Version  interface{} `json:"version"`
		Metadata []Metadata  `json:"metadata,omitempty"`
	}

	// Validatable describes an optional behavior users of this package can
	// implement on their types to have the framework perform runtime validation
	Validatable interface {
		Validate(context.Context) error
	}
)

// operation enum describes the supported concourse resource operations
type operation int

const (
	invalidOp operation = iota
	CheckOp
	InOp
	OutOp
)

// Main defines a resource binary's entrypoint
func Main(resource interface{}) {
	var op operation
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

	g := &run.Group{}
	g.Add(run.SignalHandler(context.Background(), os.Interrupt, os.Kill))

	ctx, cancel := context.WithCancel(context.Background())
	g.Add(
		func() error {
			return Exec(ctx, op, resource, os.Stdin, os.Stdout, os.Stderr, os.Args)
		},
		func(error) {
			cancel()
		},
	)

	if err := g.Run(); err != nil {
		color.New(color.FgRed).Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// =============================================================================

// Exec implements a shared entrypoint for all supported resource operations
// and handles parsing and validating resource configuration and initializing
// the resource if implemented
func Exec(ctx context.Context, op operation, resource interface{}, stdin io.Reader, stdout, stderr io.Writer, args []string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	ctx = ContextWithStdErr(ctx, stderr)

	pv := reflect.ValueOf(resource)
	if pv.Kind() != reflect.Ptr || pv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("resource provider must be pointer to struct")
	}

	if len(args) < 2 && (op == InOp || op == OutOp) {
		return fmt.Errorf("invalid operation: path argument required")
	}

	var act *action
	var path string
	switch op {
	case CheckOp:
		act = checkAction
	case InOp:
		act, path = inAction, args[1]
	case OutOp:
		act, path = outAction, args[1]
	default:
		return fmt.Errorf("unsupported op: %v", op)
	}

	var req Message
	if err := json.NewDecoder(stdin).Decode(&req); err != nil {
		return fmt.Errorf("invalid operation: error reading message: %v", err)
	}

	method := pv.MethodByName(act.method)
	if !method.IsValid() {
		return fmt.Errorf("resource is missing required method: %s", act.method)
	}

	if init := pv.MethodByName("Initialize"); init.IsValid() {
		if _, err := initAction.Exec(ctx, "", init, &req); err != nil {
			return fmt.Errorf("error initializing resource: %v", err)
		}
	}

	result, err := act.Exec(ctx, path, method, &req)
	if err != nil {
		return fmt.Errorf("error executing %s: %v", act.method, err)
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		return fmt.Errorf("error writing response: %v", err)
	}
	return nil
}
