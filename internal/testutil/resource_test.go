package testutil

import (
	context "context"
	"reflect"
	"testing"

	sdk "github.com/cludden/concourse-go-sdk"
	"github.com/cludden/concourse-go-sdk/mocks"
	archive "github.com/cludden/concourse-go-sdk/pkg/archive"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/tidwall/gjson"
)

func TestExec(t *testing.T) {
	cases := map[string]struct {
		action   *sdk.Action
		req      []byte
		resource func(t *testing.T) any
		assert   func(t *testing.T, resource, result any, err error)
	}{
		"archive": {
			action: sdk.Check(),
			req:    []byte(`{"source":{"archive":{"inmem":{"history":["{\"qux\":\"1\"}"]}}},"version":null}`),
			resource: func(t *testing.T) any {
				r := NewMockResourceArchive(t)
				r.On("Archive", mock.Anything, mock.Anything).Return(
					func(ctx context.Context, s *Source) archive.Archive {
						a := mocks.NewArchive(t)
						a.On("History", mock.Anything).Return(
							[][]byte{
								[]byte(`{"qux":"1"}`),
							},
							nil,
						)
						a.On("Close", mock.Anything).Return(nil)
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
					return versions
				}, nil)
				return r
			},
			assert: func(t *testing.T, resource, result any, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for desc, c := range cases {
		t.Run(desc, func(t *testing.T) {
			dir := t.TempDir()
			resource := c.resource(t)
			result, err := c.action.Exec(context.Background(), dir, reflect.ValueOf(resource), gjson.ParseBytes(c.req))
			c.assert(t, resource, result, err)
		})
	}
}
