// Code generated by mockery v2.16.0. DO NOT EDIT.

package archive

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockArchive is an autogenerated mock type for the Archive type
type MockArchive struct {
	mock.Mock
}

// Close provides a mock function with given fields: ctx
func (_m *MockArchive) Close(ctx context.Context) error {
	ret := _m.Called(ctx)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// History provides a mock function with given fields: ctx, latest
func (_m *MockArchive) History(ctx context.Context, latest []byte) ([][]byte, error) {
	ret := _m.Called(ctx, latest)

	var r0 [][]byte
	if rf, ok := ret.Get(0).(func(context.Context, []byte) [][]byte); ok {
		r0 = rf(ctx, latest)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([][]byte)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, []byte) error); ok {
		r1 = rf(ctx, latest)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Put provides a mock function with given fields: ctx, versions
func (_m *MockArchive) Put(ctx context.Context, versions ...[]byte) error {
	_va := make([]interface{}, len(versions))
	for _i := range versions {
		_va[_i] = versions[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, ...[]byte) error); ok {
		r0 = rf(ctx, versions...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewMockArchive interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockArchive creates a new instance of MockArchive. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockArchive(t mockConstructorTestingTNewMockArchive) *MockArchive {
	mock := &MockArchive{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
