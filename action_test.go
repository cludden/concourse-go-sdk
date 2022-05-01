package sdk

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

type (
	testSource struct {
		Addr string `json:"addr"`
	}
	testVersion struct {
		ID string `json:"id"`
	}
	testGetParams struct {
		Color string `json:"color"`
	}
	testPutParams struct {
		Size int `json:"size"`
	}
)

func (s *testSource) Validate(context.Context) error {
	return validation.ValidateStruct(s, validation.Field(&s.Addr, validation.Required, is.URL))
}

func (v *testVersion) Validate(context.Context) error {
	return validation.ValidateStruct(v, validation.Field(&v.ID, validation.Required, is.UTFDigit))
}

func (p *testGetParams) Validate(context.Context) error {
	return validation.ValidateStruct(p, validation.Field(&p.Color, validation.Required, validation.In("blue", "green")))
}

func (p *testPutParams) Validate(context.Context) error {
	return validation.ValidateStruct(p, validation.Field(&p.Size, validation.Required, validation.Min(1), validation.Max(10)))
}

func TestInitialize(t *testing.T) {
	cases := map[string]struct {
		method  interface{}
		message []byte
		errors  []string
	}{
		"ok": {},
		"bad_signature_too_few_args": {
			errors: []string{"expected method to require 2 arguments, got 1"},
			method: func(context.Context) error {
				return nil
			},
		},
		"bad_signature_too_many_args": {
			errors: []string{"expected method to require 2 arguments, got 3"},
			method: func(context.Context, *testSource, *testVersion) error {
				return nil
			},
		},
		"bad_signature_invalid_args": {
			errors: []string{
				"argument 0 must be of type context.Context",
				"argument 1 must be pointer to source struct",
			},
			method: func(a, b int) error {
				return nil
			},
		},
		"bad_signature_too_few_return_values": {
			errors: []string{"requires 1 return value, got 0"},
			method: func(context.Context, *testSource) {},
		},
		"bad_signature_too_many_return_values": {
			errors: []string{"requires 1 return value, got 2"},
			method: func(context.Context, *testSource) (context.Context, error) {
				return nil, nil
			},
		},
		"bad_signature_wrong_return_values": {
			errors: []string{
				"last return value must be of type error",
			},
			method: func(context.Context, *testSource) context.Context {
				return nil
			},
		},
	}

	for alias, c := range cases {
		t.Run(alias, func(t *testing.T) {
			dir := t.TempDir()
			msg := c.message
			if msg == nil {
				msg = []byte(`{"source":{"addr":"localhost:8080"}}`)
			}
			method := c.method
			if method == nil {
				method = func(ctx context.Context, src *testSource) error {
					assert.NotNil(t, src, "source cannot be nil")
					assert.Equal(t, src.Addr, "localhost:8080")
					return nil
				}
			}
			result, err := initAction.Exec(context.Background(), dir, reflect.ValueOf(method), gjson.ParseBytes(msg))
			if len(c.errors) > 0 {
				if assert.Error(t, err) {
					for _, desc := range c.errors {
						assert.Contains(t, err.Error(), desc)
					}
				}
			} else {
				assert.NoError(t, err)
				assert.Nil(t, result)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	source := []byte(`{"addr":"localhost:8080"}`)
	version := []byte(`{"id":"123456"}`)
	params := []byte(`{"color":"blue","size":7}`)

	cases := map[string]struct {
		method  interface{}
		message []byte
		errors  []string
	}{
		"ok": {},
		"ok_no_version": {
			message: []byte(fmt.Sprintf(`{"source":%s}`, source)),
		},
		"ok_null_version": {
			message: []byte(fmt.Sprintf(`{"source":%s,"version":null}`, source)),
		},
		"bad_signature_too_few_args": {
			errors: []string{"expected method to require 3 arguments, got 2"},
			method: func(context.Context, *testSource) ([]testVersion, error) {
				return []testVersion{}, nil
			},
		},
		"bad_signature_too_many_args": {
			errors: []string{"expected method to require 3 arguments, got 4"},
			method: func(context.Context, *testSource, *testGetParams, *testPutParams) ([]testVersion, error) {
				return []testVersion{}, nil
			},
		},
		"bad_signature_invalid_args": {
			errors: []string{
				"argument 0 must be of type context.Context",
				"argument 1 must be pointer to source struct",
				"argument 2 must be pointer to version struct",
			},
			method: func(a, b, c int) ([]testVersion, error) {
				return []testVersion{}, nil
			},
		},
		"bad_signature_too_few_return_values": {
			errors: []string{"requires 2 return values, got 1"},
			method: func(context.Context, *testSource, *testVersion) error {
				return nil
			},
		},
		"bad_signature_too_many_return_values": {
			errors: []string{"requires 2 return values, got 3"},
			method: func(context.Context, *testSource, *testVersion) (*testVersion, []Metadata, error) {
				return nil, nil, nil
			},
		},
		"bad_signature_wrong_return_values": {
			errors: []string{
				"first return value must be slice of version structs",
				"last return value must be of type error",
			},
			method: func(context.Context, *testSource, *testVersion) ([]int, []Metadata) {
				return nil, nil
			},
		},
		"bad_signature_version_mismatch": {
			errors: []string{
				"version input and output must be same type",
			},
			method: func(context.Context, *testSource, *testVersion) ([]testSource, error) {
				return nil, nil
			},
		},
		"invalid_args_source_empty": {
			message: []byte(fmt.Sprintf(`{"source":{"addr":""},"version":%s}`, version)),
			errors: []string{
				"error parsing source argument: invalid input: addr: cannot be blank",
			},
		},
		"invalid_args_source_invalid": {
			message: []byte(fmt.Sprintf(`{"source":{"addr":":)"},"version":%s}`, version)),
			errors: []string{
				"error parsing source argument: invalid input: addr: must be a valid URL",
			},
		},
		"invalid_args_version_empty": {
			message: []byte(fmt.Sprintf(`{"source":%s,"version":{}}`, source)),
			errors: []string{
				"error parsing version argument: invalid input: id: cannot be blank",
			},
		},
		"invalid_args_version_invalid": {
			message: []byte(fmt.Sprintf(`{"source":%s,"version":{"id":"foo"}}`, source)),
			errors: []string{
				"error parsing version argument: invalid input: id: must contain unicode decimal digits only",
			},
		},
	}

	for alias, c := range cases {
		t.Run(alias, func(t *testing.T) {
			msg := c.message
			if msg == nil {
				msg = []byte(fmt.Sprintf(`{"source":%s,"version":%s,"params":%s}`, source, version, params))
			}
			method := c.method
			if method == nil {
				method = func(ctx context.Context, src *testSource, v *testVersion) ([]testVersion, error) {
					assert.NotNil(t, src)
					assert.Equal(t, src.Addr, "localhost:8080")
					if v != nil {
						assert.Equal(t, v.ID, "123456")
						return []testVersion{*v}, nil
					}
					return []testVersion{}, nil
				}
			}
			result, err := checkAction.Exec(context.Background(), "", reflect.ValueOf(method), gjson.ParseBytes(msg))
			if len(c.errors) > 0 {
				if assert.Error(t, err) {
					for _, desc := range c.errors {
						assert.Contains(t, err.Error(), desc)
					}
				}
			} else {
				if assert.NoError(t, err) {
					assert.NotNil(t, result)
				}
			}
		})
	}
}

func TestIn(t *testing.T) {
	source := []byte(`{"addr":"localhost:8080"}`)
	version := []byte(`{"id":"123456"}`)
	params := []byte(`{"color":"blue","size":7}`)

	cases := map[string]struct {
		method  interface{}
		message []byte
		errors  []string
	}{
		"ok": {},
		"ok_no_params": {
			message: []byte(fmt.Sprintf(`{"source":%s,"version":%s}`, source, version)),
		},
		"ok_null_params": {
			message: []byte(fmt.Sprintf(`{"source":%s,"version":%s,"params":null}`, source, version)),
		},
		"bad_signature_too_few_args": {
			errors: []string{"expected method to require 5 arguments, got 2"},
			method: func(context.Context, *testSource) ([]testVersion, error) {
				return []testVersion{}, nil
			},
		},
		"bad_signature_too_many_args": {
			errors: []string{"expected method to require 5 arguments, got 6"},
			method: func(context.Context, *testSource, *testVersion, string, *testGetParams, *testPutParams) ([]testVersion, error) {
				return []testVersion{}, nil
			},
		},
		"bad_signature_invalid_args": {
			errors: []string{
				"argument 0 must be of type context.Context",
				"argument 1 must be pointer to source struct",
				"argument 2 must be pointer to version struct",
				"argument 3 must be path string",
				"argument 4 must be pointer to params struct",
			},
			method: func(a, b, c, d, e int) ([]testVersion, error) {
				return []testVersion{}, nil
			},
		},
		"bad_signature_too_few_return_values": {
			errors: []string{"requires 3 return values, got 1"},
			method: func(context.Context, *testSource, *testVersion, string, *testGetParams) error {
				return nil
			},
		},
		"bad_signature_too_many_return_values": {
			errors: []string{"requires 3 return values, got 4"},
			method: func(context.Context, *testSource, *testVersion, string, *testGetParams) (*testVersion, []Metadata, error, error) {
				return nil, nil, nil, nil
			},
		},
		"bad_signature_wrong_return_values": {
			errors: []string{
				"first return value must be pointer to version struct",
				"second return value must be slice of metadata",
				"last return value must be of type error",
			},
			method: func(context.Context, *testSource, *testVersion, string, *testGetParams) (context.Context, []context.Context, context.Context) {
				return nil, nil, nil
			},
		},
		"invalid_args_params_empty": {
			message: []byte(fmt.Sprintf(`{"source":%s,"version":%s,"params":{"color":""}}`, source, version)),
			errors: []string{
				"error parsing params argument: invalid input: color: cannot be blank",
			},
		},
		"invalid_args_params_invalid": {
			message: []byte(fmt.Sprintf(`{"source":%s,"version":%s,"params":{"color":"red"}}`, source, version)),
			errors: []string{
				"error parsing params argument: invalid input: color: must be a valid value",
			},
		},
		"empty_source_and_params": {
			message: []byte(fmt.Sprintf(`{"source":null,"version":%s,"params":null}`, version)),
		},
	}

	for alias, c := range cases {
		t.Run(alias, func(t *testing.T) {
			dir := t.TempDir()
			msg := c.message
			if msg == nil {
				msg = []byte(fmt.Sprintf(`{"source":%s,"version":%s,"params":%s}`, source, version, params))
			}
			method := c.method
			if method == nil {
				method = func(ctx context.Context, src *testSource, v *testVersion, path string, params *testGetParams) (*testVersion, []Metadata, error) {
					if src != nil {
						assert.Equal(t, src.Addr, "localhost:8080")
					}
					assert.NotNil(t, v, "version cannot be nil")
					assert.Equal(t, v.ID, "123456")
					assert.Equal(t, dir, path)
					if params != nil {
						assert.Equal(t, "blue", params.Color)
					}
					return v, []Metadata{{Name: "foo", Value: "bar"}}, nil
				}
			}
			result, err := inAction.Exec(context.Background(), dir, reflect.ValueOf(method), gjson.ParseBytes(msg))
			if len(c.errors) > 0 {
				if assert.Error(t, err) {
					for _, desc := range c.errors {
						assert.Contains(t, err.Error(), desc)
					}
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestOut(t *testing.T) {
	source := []byte(`{"addr":"localhost:8080"}`)
	params := []byte(`{"color":"blue","size":7}`)

	cases := map[string]struct {
		method  interface{}
		message []byte
		errors  []string
	}{
		"ok": {},
		"ok_no_params": {
			message: []byte(fmt.Sprintf(`{"source":%s}`, source)),
		},
		"ok_null_params": {
			message: []byte(fmt.Sprintf(`{"source":%s,"params":null}`, source)),
		},
		"bad_signature_too_few_args": {
			errors: []string{"expected method to require 4 arguments, got 2"},
			method: func(context.Context, *testSource) ([]testVersion, error) {
				return []testVersion{}, nil
			},
		},
		"bad_signature_too_many_args": {
			errors: []string{"expected method to require 4 arguments, got 6"},
			method: func(context.Context, *testSource, *testVersion, string, *testGetParams, *testPutParams) ([]testVersion, error) {
				return []testVersion{}, nil
			},
		},
		"bad_signature_invalid_args": {
			errors: []string{
				"argument 0 must be of type context.Context",
				"argument 1 must be pointer to source struct",
				"argument 2 must be path string",
				"argument 3 must be pointer to params struct",
			},
			method: func(a, b, c, d int) ([]testVersion, error) {
				return []testVersion{}, nil
			},
		},
		"bad_signature_too_few_return_values": {
			errors: []string{"requires 3 return values, got 1"},
			method: func(context.Context, *testSource, string, *testPutParams) error {
				return nil
			},
		},
		"bad_signature_too_many_return_values": {
			errors: []string{"requires 3 return values, got 4"},
			method: func(context.Context, *testSource, string, *testPutParams) (*testVersion, []Metadata, error, error) {
				return nil, nil, nil, nil
			},
		},
		"bad_signature_wrong_return_values": {
			errors: []string{
				"first return value must be pointer to version struct",
				"second return value must be slice of metadata",
				"last return value must be of type error",
			},
			method: func(context.Context, *testSource, string, *testPutParams) (context.Context, []context.Context, context.Context) {
				return nil, nil, nil
			},
		},
		"invalid_args_params_empty": {
			message: []byte(fmt.Sprintf(`{"source":%s,"params":{}}`, source)),
			errors: []string{
				"error parsing params argument: invalid input: size: cannot be blank",
			},
		},
		"invalid_args_params_invalid": {
			message: []byte(fmt.Sprintf(`{"source":%s,"params":{"size":100}}`, source)),
			errors: []string{
				"error parsing params argument: invalid input: size: must be no greater than 10",
			},
		},
	}

	for alias, c := range cases {
		t.Run(alias, func(t *testing.T) {
			dir := t.TempDir()
			msg := c.message
			if msg == nil {
				msg = []byte(fmt.Sprintf(`{"source":%s,"params":%s}`, source, params))
			}
			method := c.method
			if method == nil {
				method = func(ctx context.Context, src *testSource, path string, params *testPutParams) (*testVersion, []Metadata, error) {
					assert.NotNil(t, src, "source cannot be nil")
					assert.Equal(t, src.Addr, "localhost:8080")
					assert.Equal(t, dir, path)
					if params != nil {
						assert.Equal(t, 7, params.Size)
					}
					return &testVersion{ID: "123456"}, []Metadata{{Name: "foo", Value: "bar"}}, nil
				}
			}
			result, err := outAction.Exec(context.Background(), dir, reflect.ValueOf(method), gjson.ParseBytes(msg))
			if len(c.errors) > 0 {
				if assert.Error(t, err) {
					for _, desc := range c.errors {
						assert.Contains(t, err.Error(), desc)
					}
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}
