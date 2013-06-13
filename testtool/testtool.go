// Copyright 2013 Apcera Inc. All rights reserved.

package testtool

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/apcera/logging"
	"github.com/apcera/logging/unittest"
)

// Common interface that can be used to allow testing.B and testing.T objects
// to by passed to the same function.
type Logger interface {
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Failed() bool
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Skip(args ...interface{})
	Skipf(format string, args ...interface{})
}

// -----------------------------------------------------------------------
// Initialization, cleanup, and shutdown functions.
// -----------------------------------------------------------------------

// Stores output from the logging system so it can be written only if
// the test actually fails.
var LogBuffer unittest.LogBuffer = unittest.SetupBuffer()

// This is a list of functions that will be run on test completion. Having
// this allows us to clean up temporary directories or files after the
// test is done which is a huge win.
var Finalizers []func() = nil

// Adds a function to be called once the test finishes.
func AddTestFinalizer(f func()) {
	Finalizers = append(Finalizers, f)
}

// Called at the start of a test to setup all the various state bits that
// are needed. All tests in this module should start by calling this
// function.
func StartTest(l Logger) {
	LogBuffer = unittest.SetupBuffer()
}

// Called as a defer to a test in order to clean up after a test run. All
// tests in this module should call this function as a defer right after
// calling StartTest()
func FinishTest(l Logger) {
	for i := range Finalizers {
		Finalizers[len(Finalizers)-1-i]()
	}
	Finalizers = nil
	LogBuffer.FinishTest(l)
}

// Call this to require that your test is run as root. NOTICE: this does not
// cause the test to FAIL. This seems like the most sane thing to do based on
// the shortcomings of Go's test utilities.
func TestRequiresRoot(l Logger) {
	if os.Getuid() != 0 {
		l.Skipf("This test must be run as root. Skipping.")
	}
}

// -----------------------------------------------------------------------
// Temporary file helpers.
// -----------------------------------------------------------------------

// Writes contents to a temporary file, sets up a Finalizer to remove
// the file once the test is complete, and then returns the newly
// created filename to the caller.
func WriteTempFile(l Logger, contents string) string {
	return WriteTempFileMode(l, contents, os.FileMode(0644))
}

// Like WriteTempFile but sets the mode.
func WriteTempFileMode(l Logger, contents string, mode os.FileMode) string {
	f, err := ioutil.TempFile("", "golangunittest")
	if f == nil {
		Fatalf(l, "ioutil.TempFile() return nil.")
	} else if err != nil {
		Fatalf(l, "ioutil.TempFile() return an err: %s", err)
	} else if err := os.Chmod(f.Name(), mode); err != nil {
		Fatalf(l, "os.Chmod() returned an error: %s", err)
	}
	defer f.Close()
	Finalizers = append(Finalizers, func() {
		os.Remove(f.Name())
	})
	contentsBytes := []byte(contents)
	n, err := f.Write(contentsBytes)
	if err != nil {
		Fatalf(l, "Error writing to %s: %s", f.Name(), err)
	} else if n != len(contentsBytes) {
		Fatalf(l, "Short write to %s", f.Name())
	}
	return f.Name()
}

// Makes a temporary directory
func TempDir(l Logger) string {
	return TempDirMode(l, os.FileMode(0755))
}

// Makes a temporary directory with the given mode.
func TempDirMode(l Logger, mode os.FileMode) string {
	f, err := ioutil.TempDir(RootTempDir(l), "golangunittest")
	if f == "" {
		Fatalf(l, "ioutil.TempFile() return an empty string.")
	} else if err != nil {
		Fatalf(l, "ioutil.TempFile() return an err: %s", err)
	} else if err := os.Chmod(f, mode); err != nil {
		Fatalf(l, "os.Chmod failure.")
	}

	Finalizers = append(Finalizers, func() {
		os.RemoveAll(f)
	})
	return f
}

// Allocate a temporary file and ensure that it gets cleaned up when the
// test is completed.
func TempFile(l Logger) string {
	return TempFileMode(l, os.FileMode(0644))
}

// Writes a temp file with the given mode.
func TempFileMode(l Logger, mode os.FileMode) string {
	f, err := ioutil.TempFile(RootTempDir(l), "unittest")
	if err != nil {
		Fatalf(l, "Error making temporary file: %s", err)
	} else if err := os.Chmod(f.Name(), mode); err != nil {
		Fatalf(l, "os.Chmod failure.")
	}
	defer f.Close()
	name := f.Name()
	Finalizers = append(Finalizers, func() {
		os.RemoveAll(name)
	})
	return name
}

// -----------------------------------------------------------------------
// Fatalf wrapper.
// -----------------------------------------------------------------------

// This function wraps Fatalf in order to provide a functional stack trace
// on failures rather than just a line number of the failing check. This
// helps if you have a test that fails in a loop since it will show
// the path to get there as well as the error directly.
func Fatalf(l Logger, f string, args ...interface{}) {
	lines := make([]string, 0, 100)
	msg := fmt.Sprintf(f, args...)
	lines = append(lines, msg)

	// Get the directory of testtool in order to ensure that we don't show
	// it in the stack traces (it can be spammy).
	_, myfile, _, _ := runtime.Caller(0)
	mydir := path.Dir(myfile)

	// Generate the Stack of callers:
	for i := 0; true; i++ {
		_, file, line, ok := runtime.Caller(i)
		if ok == false {
			break
		}
		// Don't print the stack line if its within testtool since its
		// annoying to see the testtool internals.
		if path.Dir(file) == mydir {
			continue
		}
		msg := fmt.Sprintf("%d - %s:%d", i, file, line)
		lines = append(lines, msg)
	}

	logging.Errorf("Test has failed: %s", msg)
	l.Fatalf("%s", strings.Join(lines, "\n"))
}

func Fatal(t Logger, args ...interface{}) {
	Fatalf(t, "%s", fmt.Sprint(args...))
}

// -----------------------------------------------------------------------
// Simple Timeout functions
// -----------------------------------------------------------------------

// runs the given function until 'timeout' has passed, sleeping 'sleep'
// duration in between runs. If the function returns true this exits,
// otherwise after timeout this will fail the test.
func Timeout(
	l Logger, timeout time.Duration, sleep time.Duration,
	f func() bool,
) {
	end := time.Now().Add(timeout)
	for time.Now().Before(end) {
		if f() == true {
			return
		}
		time.Sleep(sleep)
	}
	Fatalf(l, "testtool: Timeout after %v", timeout)
}

// -----------------------------------------------------------------------
// Error object handling functions.
// -----------------------------------------------------------------------

// Fatal's the test if err is nil.
func TestExpectError(l Logger, err error) {
	if err == nil {
		Fatalf(l, "Expected error not returned.")
	}
}

// Fatal's the test if err is not nil.
func TestExpectSuccess(l Logger, err error) {
	if err != nil {
		Fatalf(l, "Unexpected error: %s", err)
	}
}
