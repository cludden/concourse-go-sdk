package sdk

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/cludden/concourse-go-sdk/mocks"
	"github.com/cludden/concourse-go-sdk/pkg/archive"
	"github.com/cludden/concourse-go-sdk/pkg/archive/inmem"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/tidwall/gjson"
)

type (
	testSource struct {
		Addr string `json:"addr" validate:"required,url"`
	}
	testVersion struct {
		ID string `json:"id" validate:"required,numeric"`
	}
	testGetParams struct {
		Color string `json:"color" validate:"required,oneof=blue green"`
	}
	testPutParams struct {
		Size int `json:"size" validate:"required,min=1,max=10"`
	}
)

func (s *testSource) Validate(ctx context.Context) error {
	if s == nil {
		return nil
	}
	return validator.New().StructCtx(ctx, s)
}

func (v *testVersion) Validate(ctx context.Context) error {
	if v == nil {
		return nil
	}
	return validator.New().StructCtx(ctx, v)
}

func (p *testGetParams) Validate(ctx context.Context) error {
	if p == nil {
		return nil
	}
	return validator.New().StructCtx(ctx, p)
}

func (p *testPutParams) Validate(ctx context.Context) error {
	if p == nil {
		return nil
	}
	return validator.New().StructCtx(ctx, p)
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
			result, err := initAction.exec(context.Background(), dir, reflect.ValueOf(method), gjson.ParseBytes(msg), nil)
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
		archive func(t *testing.T) archive.Archive
		message []byte
		errors  []string
		assert  func(t *testing.T, a archive.Archive, result any)
	}{
		"ok": {
			method: func(ctx context.Context, src *testSource, v *testVersion) ([]testVersion, error) {
				assert.NotNil(t, src)
				assert.Equal(t, src.Addr, "localhost:8080")
				assert.NotNil(t, v, "expected version to not be nil")
				assert.Equal(t, v.ID, "123456")
				return []testVersion{}, nil
			},
		},
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
				"required",
			},
		},
		"invalid_args_source_invalid": {
			message: []byte(fmt.Sprintf(`{"source":{"addr":":)"},"version":%s}`, version)),
			errors: []string{
				"url",
			},
		},
		"invalid_args_version_empty": {
			message: []byte(fmt.Sprintf(`{"source":%s,"version":{}}`, source)),
			errors: []string{
				"required",
			},
		},
		"invalid_args_version_invalid": {
			message: []byte(fmt.Sprintf(`{"source":%s,"version":{"id":"foo"}}`, source)),
			errors: []string{
				"numeric",
			},
		},
		"ok_archive_no_history_single_version_emitted": {
			message: []byte(fmt.Sprintf(`{"source":%s}`, source)),
			archive: func(t *testing.T) archive.Archive {
				a, _ := archive.New(context.Background(), archive.Config{Inmem: &inmem.Config{}})
				return a
			},
			method: func(ctx context.Context, src *testSource, v *testVersion) ([]testVersion, error) {
				assert.NotNil(t, src)
				assert.Equal(t, src.Addr, "localhost:8080")
				assert.Nil(t, v)
				return []testVersion{{ID: "abcdef"}}, nil
			},
			assert: func(t *testing.T, a archive.Archive, result any) {
				history, err := a.History(context.Background(), nil)
				assert.NoError(t, err)
				assert.Len(t, history, 1)
				assert.Equal(t, "abcdef", gjson.ParseBytes(history[0]).Get("id").String())
			},
		},
		"ok_archive_no_history_multiple_versions_emitted": {
			message: []byte(fmt.Sprintf(`{"source":%s}`, source)),
			archive: func(t *testing.T) archive.Archive {
				a, _ := archive.New(context.Background(), archive.Config{Inmem: &inmem.Config{}})
				return a
			},
			method: func(ctx context.Context, src *testSource, v *testVersion) ([]testVersion, error) {
				assert.NotNil(t, src)
				assert.Equal(t, src.Addr, "localhost:8080")
				assert.Nil(t, v)
				return []testVersion{{ID: "1"}, {ID: "2"}, {ID: "3"}}, nil
			},
			assert: func(t *testing.T, a archive.Archive, result any) {
				versions := reflect.ValueOf(result)
				assert.Equal(t, 3, versions.Len())
				assert.Equal(t, "1", versions.Index(0).Interface().(testVersion).ID)
				assert.Equal(t, "2", versions.Index(1).Interface().(testVersion).ID)
				assert.Equal(t, "3", versions.Index(2).Interface().(testVersion).ID)

				history, err := a.History(context.Background(), nil)
				assert.NoError(t, err)
				assert.Len(t, history, 3)
				assert.Equal(t, "1", gjson.ParseBytes(history[0]).Get("id").String())
				assert.Equal(t, "2", gjson.ParseBytes(history[1]).Get("id").String())
				assert.Equal(t, "3", gjson.ParseBytes(history[2]).Get("id").String())
			},
		},
		"ok_archive_existing_history_with_source_version": {
			archive: func(t *testing.T) archive.Archive {
				m := &mocks.Archive{}
				matchVersion := func(id string) interface{} {
					return mock.MatchedBy(func(version []byte) bool {
						return assert.Equal(t, id, gjson.ParseBytes(version).Get("id").String())
					})
				}
				m.On("History", mock.Anything, mock.Anything).Return(nil, nil)
				m.On("Put", mock.Anything, matchVersion("123456"), matchVersion("4")).Return(nil)
				return m
			},
			method: func(ctx context.Context, src *testSource, v *testVersion) ([]testVersion, error) {
				assert.NotNil(t, src)
				assert.Equal(t, src.Addr, "localhost:8080")
				assert.NotNil(t, v)
				assert.Equal(t, "123456", v.ID)
				return []testVersion{{ID: "123456"}, {ID: "4"}}, nil
			},
			assert: func(t *testing.T, a archive.Archive, result any) {
				versions := reflect.ValueOf(result)
				assert.Equal(t, 2, versions.Len())
				assert.Equal(t, "123456", versions.Index(0).Interface().(testVersion).ID)
				assert.Equal(t, "4", versions.Index(1).Interface().(testVersion).ID)
			},
		},
		"ok_archive_existing_history_single_version_emitted": {
			message: []byte(fmt.Sprintf(`{"source":%s}`, source)),
			archive: func(t *testing.T) archive.Archive {
				a, _ := archive.New(context.Background(), archive.Config{Inmem: &inmem.Config{
					History: []string{
						`{"id":"1"}`,
						`{"id":"2"}`,
						`{"id":"3"}`,
					},
				}})
				return a
			},
			method: func(ctx context.Context, src *testSource, v *testVersion) ([]testVersion, error) {
				assert.NotNil(t, src)
				assert.Equal(t, src.Addr, "localhost:8080")
				assert.NotNil(t, v)
				assert.Equal(t, "3", v.ID)
				return []testVersion{{ID: "3"}, {ID: "4"}}, nil
			},
			assert: func(t *testing.T, a archive.Archive, result any) {
				versions := reflect.ValueOf(result)
				assert.Equal(t, 4, versions.Len())
				assert.Equal(t, "1", versions.Index(0).Interface().(testVersion).ID)
				assert.Equal(t, "2", versions.Index(1).Interface().(testVersion).ID)
				assert.Equal(t, "3", versions.Index(2).Interface().(testVersion).ID)
				assert.Equal(t, "4", versions.Index(3).Interface().(testVersion).ID)

				history, err := a.History(context.Background(), nil)
				assert.NoError(t, err)
				assert.Len(t, history, 4)
				assert.Equal(t, "1", gjson.ParseBytes(history[0]).Get("id").String())
				assert.Equal(t, "2", gjson.ParseBytes(history[1]).Get("id").String())
				assert.Equal(t, "3", gjson.ParseBytes(history[2]).Get("id").String())
				assert.Equal(t, "4", gjson.ParseBytes(history[3]).Get("id").String())
			},
		},
		"ok_archive_existing_history_multiple_versions_emitted": {
			message: []byte(fmt.Sprintf(`{"source":%s}`, source)),
			archive: func(t *testing.T) archive.Archive {
				a, _ := archive.New(context.Background(), archive.Config{Inmem: &inmem.Config{
					History: []string{
						`{"id":"1"}`,
						`{"id":"2"}`,
						`{"id":"3"}`,
					},
				}})
				return a
			},
			method: func(ctx context.Context, src *testSource, v *testVersion) ([]testVersion, error) {
				assert.NotNil(t, src)
				assert.Equal(t, src.Addr, "localhost:8080")
				assert.NotNil(t, v)
				assert.Equal(t, "3", v.ID)
				return []testVersion{{ID: "3"}, {ID: "4"}, {ID: "5"}}, nil
			},
			assert: func(t *testing.T, a archive.Archive, result any) {
				versions := reflect.ValueOf(result)
				assert.Equal(t, 5, versions.Len())
				assert.Equal(t, "1", versions.Index(0).Interface().(testVersion).ID)
				assert.Equal(t, "2", versions.Index(1).Interface().(testVersion).ID)
				assert.Equal(t, "3", versions.Index(2).Interface().(testVersion).ID)
				assert.Equal(t, "4", versions.Index(3).Interface().(testVersion).ID)
				assert.Equal(t, "5", versions.Index(4).Interface().(testVersion).ID)

				history, err := a.History(context.Background(), nil)
				assert.NoError(t, err)
				assert.Len(t, history, 5)
				assert.Equal(t, "1", gjson.ParseBytes(history[0]).Get("id").String())
				assert.Equal(t, "2", gjson.ParseBytes(history[1]).Get("id").String())
				assert.Equal(t, "3", gjson.ParseBytes(history[2]).Get("id").String())
				assert.Equal(t, "4", gjson.ParseBytes(history[3]).Get("id").String())
				assert.Equal(t, "5", gjson.ParseBytes(history[4]).Get("id").String())
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
			var a archive.Archive
			if c.archive != nil {
				a = c.archive(t)
			}
			result, err := checkAction.exec(context.Background(), "", reflect.ValueOf(method), gjson.ParseBytes(msg), a)
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
			if c.assert != nil {
				c.assert(t, a, result)
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
				"Error:Field validation for 'Color' failed on the 'required' tag",
			},
		},
		"invalid_args_params_invalid": {
			message: []byte(fmt.Sprintf(`{"source":%s,"version":%s,"params":{"color":"red"}}`, source, version)),
			errors: []string{
				"Error:Field validation for 'Color' failed on the 'oneof' tag",
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
			result, err := inAction.exec(context.Background(), dir, reflect.ValueOf(method), gjson.ParseBytes(msg), nil)
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
		archive func(t *testing.T) archive.Archive
		errors  []string
		assert  func(t *testing.T, a archive.Archive, result any)
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
				"Key: 'testPutParams.Size' Error:Field validation for 'Size' failed on the 'required' tag",
			},
		},
		"invalid_args_params_invalid": {
			message: []byte(fmt.Sprintf(`{"source":%s,"params":{"size":100}}`, source)),
			errors: []string{
				"Key: 'testPutParams.Size' Error:Field validation for 'Size' failed on the 'max' tag",
			},
		},
		"ok_archive": {
			archive: func(t *testing.T) archive.Archive {
				a, _ := archive.New(context.Background(), archive.Config{Inmem: &inmem.Config{}})
				return a
			},
			assert: func(t *testing.T, a archive.Archive, result any) {
				history, err := a.History(context.Background(), nil)
				assert.NoError(t, err)
				assert.Len(t, history, 1)
				assert.Equal(t, "123456", gjson.ParseBytes(history[0]).Get("id").String())
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
			var a archive.Archive
			if c.archive != nil {
				a = c.archive(t)
			}
			result, err := outAction.exec(context.Background(), dir, reflect.ValueOf(method), gjson.ParseBytes(msg), a)
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
			if c.assert != nil {
				c.assert(t, a, result)
			}
		})
	}
}
