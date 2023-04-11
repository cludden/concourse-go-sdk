package testutil

import (
	"bytes"
	context "context"
	"fmt"
	"testing"

	sdk "github.com/cludden/concourse-go-sdk"
	"github.com/cludden/concourse-go-sdk/mocks"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/tidwall/gjson"
)

func TestExec(t *testing.T) {
	cases := map[string]struct {
		operation sdk.Op
		req       []byte
		resource  func(t *testing.T) sdk.Resource[Source, Version, GetParams, PutParams]
		assert    func(t *testing.T, resource any, result *gjson.Result, err error)
	}{
		"archive": {
			operation: sdk.CheckOp,
			req:       []byte(`{"source":{"archive":{"inmem":{"history":["{\"qux\":\"1\"}"]}}},"version":null}`),
			resource: func(t *testing.T) sdk.Resource[Source, Version, GetParams, PutParams] {
				r := NewMockResource(t)
				r.On("Initialize", mock.Anything, mock.AnythingOfType("*testutil.Source")).
					Return(nil)
				r.On("Close", mock.Anything).
					Return(nil)
				r.On("Archive", mock.Anything, mock.Anything).
					Return(
						func(ctx context.Context, s *Source) sdk.Archive {
							a := mocks.NewArchive(t)
							a.On("History", mock.Anything, mock.Anything).
								Return(
									[][]byte{
										[]byte(`{"qux":"1"}`),
									},
									nil,
								)
							a.On("Put", mock.Anything, mock.Anything).
								Return(func(ctx context.Context, versions ...[]byte) error {
									if len(versions) != 1 {
										return fmt.Errorf("expected Put to be called with 1 version, got: %d", len(versions))
									}
									if string(versions[0]) != `{"qux":"2"}` {
										return fmt.Errorf("invalid version")
									}
									return nil
								})
							a.On("Close", mock.Anything).
								Return(nil)
							return a
						},
						nil,
					)
				r.On(
					"Check",
					mock.Anything,
					mock.MatchedBy(func(s *Source) bool {
						ok := assert.NotNil(t, s)
						ok = ok && assert.NotNil(t, s.Archive)
						return ok
					}),
					mock.MatchedBy(func(v *Version) bool {
						ok := assert.NotNil(t, v)
						ok = ok && assert.Equal(t, "1", v.Qux)
						return ok
					}),
				).Return(func(ctx context.Context, s *Source, v *Version) (versions []Version) {
					if v != nil {
						versions = append(versions, *v)
					}
					versions = append(versions, Version{Qux: "2"})
					return versions
				}, nil)
				return r
			},
			assert: func(t *testing.T, resource any, result *gjson.Result, err error) {
				assert.NoError(t, err)
			},
		},
		"check_null_version_no_history": {
			operation: sdk.CheckOp,
			req:       []byte(`{"source":{},"version":null}`),
			resource: func(t *testing.T) sdk.Resource[Source, Version, GetParams, PutParams] {
				r := NewMockResource(t)
				r.On("Initialize", mock.Anything, mock.AnythingOfType("*testutil.Source")).Return(nil)
				r.On("Archive", mock.Anything, mock.AnythingOfType("*testutil.Source")).Return(nil, nil)
				r.On("Close", mock.Anything).Return(nil)
				r.On(
					"Check",
					mock.Anything,
					mock.MatchedBy(func(s *Source) bool {
						ok := assert.NotNil(t, s)
						ok = ok && assert.Nil(t, s.Archive)
						return ok
					}),
					mock.MatchedBy(func(v *Version) bool {
						ok := assert.Nil(t, v)
						return ok
					}),
				).Return(func(ctx context.Context, s *Source, v *Version) (versions []Version) {
					if v != nil {
						versions = append(versions, *v)
					}
					return versions
				}, nil)

				return r
			},
			assert: func(t *testing.T, resource any, result *gjson.Result, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for desc, c := range cases {
		t.Run(desc, func(t *testing.T) {
			var args []string
			switch c.operation {
			case sdk.CheckOp:
				args = append(args, "/opt/resource/check")
			case sdk.InOp:
				args = append(args, "/opt/resource/in", t.TempDir())
			case sdk.OutOp:
				args = append(args, "/opt/resource/out", t.TempDir())
			}

			resource := c.resource(t)
			stderr, stdout := &bytes.Buffer{}, &bytes.Buffer{}
			stdin := bytes.NewBuffer(c.req)
			err := sdk.Exec(context.Background(), c.operation, resource, stdin, stdout, stderr, args)
			result := gjson.ParseBytes(stdout.Bytes())
			c.assert(t, resource, &result, err)
		})
	}
}
