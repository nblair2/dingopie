package cmd

import (
	"errors"
	"os"
	"testing"

	"github.com/spf13/cobra"
)

// withStdin temporarily replaces os.Stdin for the duration of the test, restoring it on cleanup.
func withStdin(t *testing.T, f *os.File) {
	t.Helper()

	orig := os.Stdin
	os.Stdin = f

	t.Cleanup(func() {
		os.Stdin = orig
	})
}

//nolint:paralleltest // swaps the global os.Stdin; must not run in parallel
func TestGetData_NoDataStdinEmpty(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("w.Close: %v", err)
	}

	t.Cleanup(func() {
		_ = r.Close()
	})

	withStdin(t, r)

	cmd := &cobra.Command{}

	_, err = getData(cmd, "", nil)
	if !errors.Is(err, ErrNoData) {
		t.Errorf("errors.Is(err, ErrNoData) = false, err: %v", err)
	}
}

//nolint:paralleltest // swaps the global os.Stdin; must not run in parallel
func TestGetData_NoArgsNoStdin(t *testing.T) {
	devNull, err := os.Open("/dev/null")
	if err != nil {
		t.Skipf("cannot open /dev/null to simulate a character device: %v", err)
	}

	t.Cleanup(func() {
		_ = devNull.Close()
	})

	withStdin(t, devNull)

	cmd := &cobra.Command{}

	_, err = getData(cmd, "", nil)
	if !errors.Is(err, ErrNoData) {
		t.Errorf("errors.Is(err, ErrNoData) = false, err: %v", err)
	}
}

//nolint:paralleltest // swaps the global os.Stdin; must not run in parallel
func TestGetData_TooManyArgs(t *testing.T) {
	devNull, err := os.Open("/dev/null")
	if err != nil {
		t.Skipf("cannot open /dev/null to simulate a character device: %v", err)
	}

	t.Cleanup(func() {
		_ = devNull.Close()
	})

	withStdin(t, devNull)

	cmd := &cobra.Command{}

	_, err = getData(cmd, "", []string{"one", "two"})
	if !errors.Is(err, ErrTooManyArgs) {
		t.Errorf("errors.Is(err, ErrTooManyArgs) = false, err: %v", err)
	}
}

//nolint:paralleltest // swaps the global os.Stdin; must not run in parallel
func TestGetData_SingleArg(t *testing.T) {
	devNull, err := os.Open("/dev/null")
	if err != nil {
		t.Skipf("cannot open /dev/null to simulate a character device: %v", err)
	}

	t.Cleanup(func() {
		_ = devNull.Close()
	})

	withStdin(t, devNull)

	cmd := &cobra.Command{}

	data, err := getData(cmd, "", []string{"hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(data) != "hello" {
		t.Errorf("data = %q, want %q", data, "hello")
	}
}
