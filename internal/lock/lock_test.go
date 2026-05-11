package lock

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestTryLockCreatesLockFileWithPID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "composed.lock")

	locked, err := TryLock(path)
	if err != nil {
		t.Fatalf("TryLock() error = %v", err)
	}
	if !locked {
		t.Fatalf("TryLock() locked = false, want true")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	got := strings.TrimSpace(string(data))
	want := strconv.Itoa(os.Getpid())
	if got != want {
		t.Fatalf("lock file content = %q, want %q", got, want)
	}
}

func TestTryLockReturnsFalseWhenAlreadyLocked(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "composed.lock")

	locked, err := TryLock(path)
	if err != nil {
		t.Fatalf("first TryLock() error = %v", err)
	}
	if !locked {
		t.Fatalf("first TryLock() locked = false, want true")
	}

	locked, err = TryLock(path)
	if err != nil {
		t.Fatalf("second TryLock() error = %v", err)
	}
	if locked {
		t.Fatalf("second TryLock() locked = true, want false")
	}
}

func TestTryLockReclaimsStaleLock(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "composed.lock")

	if err := os.WriteFile(path, []byte("99999999\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	locked, err := TryLock(path)
	if err != nil {
		t.Fatalf("TryLock() error = %v", err)
	}
	if !locked {
		t.Fatalf("TryLock() locked = false, want true")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	got := strings.TrimSpace(string(data))
	want := strconv.Itoa(os.Getpid())
	if got != want {
		t.Fatalf("lock file content = %q, want %q", got, want)
	}
}

func TestTryLockReturnsErrorForInvalidLockPID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "composed.lock")

	if err := os.WriteFile(path, []byte("not-a-pid\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	locked, err := TryLock(path)
	if err == nil {
		t.Fatalf("TryLock() error = nil, want non-nil")
	}
	if locked {
		t.Fatalf("TryLock() locked = true, want false")
	}
}

func TestLockReturnsErrAlreadyLocked(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "composed.lock")

	if err := Lock(path); err != nil {
		t.Fatalf("first Lock() error = %v", err)
	}

	err := Lock(path)
	if !errors.Is(err, ErrAlreadyLocked) {
		t.Fatalf("second Lock() error = %v, want ErrAlreadyLocked", err)
	}
}

func TestUnlockRemovesExistingLockFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "composed.lock")

	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := Unlock(path); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock file still exists or unexpected stat error: %v", err)
	}
}

func TestUnlockIgnoresMissingLockFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.lock")

	if err := Unlock(path); err != nil {
		t.Fatalf("Unlock() error = %v, want nil", err)
	}
}

func TestUnlockReturnsErrLockNotOwned(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "composed.lock")

	foreignPID := os.Getpid() + 1
	if err := os.WriteFile(path, []byte(strconv.Itoa(foreignPID)+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err := Unlock(path)
	if !errors.Is(err, ErrLockNotOwned) {
		t.Fatalf("Unlock() error = %v, want ErrLockNotOwned", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected lock file to remain, stat error = %v", err)
	}
}
