package testtool

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
)

const startupInterceptorToken = "sdf908s0dijflk23423"

// This function is used to intercept the process startup and check to see if
// if its a clean up process.
func init() {
	if len(os.Args) != 3 {
		return
	} else if os.Args[1] != startupInterceptorToken {
		return
	}

	// Do NOT remove anything unless its in the temporary directory.
	if !strings.HasPrefix(os.Args[2], os.TempDir()) {
		fmt.Fprintf(
			os.Stderr, "Will not run on %s, its not in %s",
			os.Args[2], os.TempDir())
		os.Exit(1)
	}

	// Wait for stdin to be closed, once that happens we nuke the directory
	// in the third argument.
	if _, err := ioutil.ReadAll(os.Stdin); err != nil {
		fmt.Fprintf(
			os.Stderr, "Error cleaning up directory %s: %s\n",
			os.Args[2], err)
	} else if err := os.RemoveAll(os.Args[2]); err != nil {
		fmt.Fprintf(
			os.Stderr, "Error cleaning up directory %s: %s\n",
			os.Args[2], err)
	}
	os.Exit(0)
}

// Stores the persistent root directory.
var rootDirectory string

// Used to ensure that we only make the root directory once.
var rootDirectoryOnce sync.Once

var rootDirectoryStdin io.Writer

// Creates a directory that will exist until the process running the tests
// exits.
func RootTempDir(t *testing.T) string {
	rootDirectoryOnce.Do(func() {
		var reader *os.File
		var err error

		mode := os.FileMode(0777)
		rootDirectory, err = ioutil.TempDir("", "rootunittest")
		if rootDirectory == "" {
			t.Fatalf("ioutil.TempFile() return an empty string.")
		} else if err != nil {
			t.Fatalf("ioutil.TempFile() return an err: %s", err)
		} else if err := os.Chmod(rootDirectory, mode); err != nil {
			t.Fatalf("os.Chmod error: %s", err)
		} else if reader, rootDirectoryStdin, err = os.Pipe(); err != nil {
			t.Fatalf("io.Pipe() failed: %s", err)
		}
		cmd := exec.Command(os.Args[0], startupInterceptorToken, rootDirectory)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = reader
		if err := cmd.Start(); err != nil {
			t.Fatalf("cmd.Start() failed: %s", err)
		} else if err := reader.Close(); err != nil {
			t.Fatalf("Close() error: %s", err)
		}
	})
	return rootDirectory
}
