package testutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"testing"
)

/*
	UTILITY CODE
*/

// return the "file:line" of the caller's nth ancestor
func fileLinePrefix(n int) string {
	_, file, line, ok := runtime.Caller(n + 1)
	if ok {
		return fmt.Sprintf("%v:%v", filepath.Base(file), line)
	}
	return "unknown file/line"
}

// Use mockError when panicking in tests
// so that when we recover we can tell it
// apart from other panics.
type mockError string

func (m mockError) Error() string { return string(m) }

// mockT implements the testingT interface
type mockT struct{}

func (m mockT) Fatalf(format string, args ...interface{}) {
	panic(mockError(fmt.Sprintf(format, args...)))
}

// It doesn't matter what value
// of mockT you use; use this
var mock mockT

func expectFatal(t *testing.T, regex string, f func()) {
	re := regexp.MustCompile(regex)

	// Calculate prefix ahead of time because if there's
	// a panic, the defered function will get called from
	// the call site of the panic, and we can't predict
	// what that stack will look like.
	prefix := fileLinePrefix(1)
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("%v: function failed to call Fatalf", prefix)
		}
		me, ok := r.(mockError)
		if !ok {
			panic(r)
		}
		if !re.MatchString(string(me)) {
			t.Errorf("%v: unexpected call to Fatalf: want regex %v; got %v", prefix, re, me)
		}
	}()
	f()
}

func expectSuccess(t *testing.T, f func()) {
	// Calculate prefix ahead of time because if there's
	// a panic, the defered function will get called from
	// the call site of the panic, and we can't predict
	// what that stack will look like.
	prefix := fileLinePrefix(1)
	defer func() {
		r := recover()
		if me, ok := r.(mockError); ok {
			t.Errorf("%v: function called Fatalf: %v", prefix, me)
		} else if r != nil {
			panic(r)
		}
	}()
	f()
}

/*
	TESTS
*/

func TestSrcDir(t *testing.T) {
	d, ok := SrcDir()
	if !ok {
		t.Fatalf("SrcDir returned false")
	}
	if filepath.Base(d) != "testutil" {
		t.Fatalf("SrcDir returned wrong directory: %v", d)
	}

	// Since the testing code and the code itself are all
	// in the same package, the first test isn't very
	// likely to fail even if the code is wrong. Thus, verify
	// by calling through another package.
	v := reflect.ValueOf(SrcDir)
	ret := v.Call(nil)
	if !ret[1].Bool() || filepath.Base(ret[0].String()) != "runtime" {
		t.Fatalf("SrcDir returned wrong results: %v, %v", ret[0].Interface(), ret[1].Interface())
	}
}

func TestMust(t *testing.T) {
	expectSuccess(t, func() { mustImpl(mock, nil) })
	f := func() { mustImpl(mock, errors.New("foo")) }
	expectFatal(t, "testutil_test.go:[0-9]+: foo", f)
}

func TestMustPrefix(t *testing.T) {
	expectSuccess(t, func() { mustPrefix(mock, "", nil) })
	f := func() { mustPrefix(mock, "foo", errors.New("bar")) }
	expectFatal(t, "testutil_test.go:[0-9]+: foo: bar", f)
}

func TestMustTempFile(t *testing.T) {
	var name string
	expectSuccess(t, func() { mustTempFile(mock, "", "") })
	defer os.Remove(name)

	// Make a directory we know for a fact is empty
	dir := MustTempDir(t, "", "")
	defer os.RemoveAll(dir)
	nonexistant := filepath.Join(dir, "foo")
	re := "^testutil_test.go:[0-9]+: open " + nonexistant +
		".*: no such file or directory$"
	expectFatal(t, re, func() { mustTempFile(mock, nonexistant, "") })
}

func TestMustTempDir(t *testing.T) {
	var name string
	expectSuccess(t, func() { mustTempDir(mock, "", "") })
	defer os.Remove(name)

	// Make a directory we know for a fact is empty
	dir := MustTempDir(t, "", "")
	defer os.RemoveAll(dir)
	nonexistant := filepath.Join(dir, "foo")
	re := "^testutil_test.go:[0-9]+: mkdir " + nonexistant +
		".*: no such file or directory$"
	expectFatal(t, re, func() { mustTempDir(mock, nonexistant, "") })
}
