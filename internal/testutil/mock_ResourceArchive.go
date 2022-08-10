// Code generated by mockery v2.13.1. DO NOT EDIT.

package testutil

import (
	context "context"

	archive "github.com/cludden/concourse-go-sdk/pkg/archive"

	mock "github.com/stretchr/testify/mock"

	sdk "github.com/cludden/concourse-go-sdk"
)

// MockResourceArchive is an autogenerated mock type for the ResourceArchive type
type MockResourceArchive struct {
	mock.Mock
}

// Archive provides a mock function with given fields: _a0, _a1
func (_m *MockResourceArchive) Archive(_a0 context.Context, _a1 *Source) (archive.Archive, error) {
	ret := _m.Called(_a0, _a1)

	var r0 archive.Archive
	if rf, ok := ret.Get(0).(func(context.Context, *Source) archive.Archive); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(archive.Archive)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *Source) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Check provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockResourceArchive) Check(_a0 context.Context, _a1 *Source, _a2 *Version) ([]Version, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 []Version
	if rf, ok := ret.Get(0).(func(context.Context, *Source, *Version) []Version); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]Version)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *Source, *Version) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// In provides a mock function with given fields: _a0, _a1, _a2, _a3, _a4
func (_m *MockResourceArchive) In(_a0 context.Context, _a1 *Source, _a2 *Version, _a3 string, _a4 *GetParams) (*Version, []sdk.Metadata, error) {
	ret := _m.Called(_a0, _a1, _a2, _a3, _a4)

	var r0 *Version
	if rf, ok := ret.Get(0).(func(context.Context, *Source, *Version, string, *GetParams) *Version); ok {
		r0 = rf(_a0, _a1, _a2, _a3, _a4)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*Version)
		}
	}

	var r1 []sdk.Metadata
	if rf, ok := ret.Get(1).(func(context.Context, *Source, *Version, string, *GetParams) []sdk.Metadata); ok {
		r1 = rf(_a0, _a1, _a2, _a3, _a4)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).([]sdk.Metadata)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(context.Context, *Source, *Version, string, *GetParams) error); ok {
		r2 = rf(_a0, _a1, _a2, _a3, _a4)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// Out provides a mock function with given fields: _a0, _a1, _a2, _a3
func (_m *MockResourceArchive) Out(_a0 context.Context, _a1 *Source, _a2 string, _a3 *PutParams) (*Version, []sdk.Metadata, error) {
	ret := _m.Called(_a0, _a1, _a2, _a3)

	var r0 *Version
	if rf, ok := ret.Get(0).(func(context.Context, *Source, string, *PutParams) *Version); ok {
		r0 = rf(_a0, _a1, _a2, _a3)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*Version)
		}
	}

	var r1 []sdk.Metadata
	if rf, ok := ret.Get(1).(func(context.Context, *Source, string, *PutParams) []sdk.Metadata); ok {
		r1 = rf(_a0, _a1, _a2, _a3)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).([]sdk.Metadata)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(context.Context, *Source, string, *PutParams) error); ok {
		r2 = rf(_a0, _a1, _a2, _a3)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

type mockConstructorTestingTNewMockResourceArchive interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockResourceArchive creates a new instance of MockResourceArchive. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockResourceArchive(t mockConstructorTestingTNewMockResourceArchive) *MockResourceArchive {
	mock := &MockResourceArchive{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
