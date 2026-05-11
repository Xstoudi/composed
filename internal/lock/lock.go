package lock

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

var ErrAlreadyLocked = errors.New("already locked")
var ErrLockNotOwned = errors.New("lock is owned by another process")

func TryLock(path string) (bool, error) {
	locked, err := createLockFile(path)
	if err == nil {
		return locked, nil
	}

	if !errors.Is(err, os.ErrExist) {
		return false, err
	}

	stale, err := isStaleLock(path)
	if err != nil {
		return false, err
	}

	if !stale {
		return false, nil
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("remove stale lock file: %w", err)
	}

	locked, err = createLockFile(path)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return false, nil
		}
		return false, err
	}

	return locked, nil
}

func Lock(path string) error {
	locked, err := TryLock(path)
	if err != nil {
		return err
	}
	if !locked {
		return ErrAlreadyLocked
	}
	return nil
}

func Unlock(path string) error {
	ownerPID, err := readLockPID(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	if ownerPID != os.Getpid() {
		return fmt.Errorf("%w: owner=%d current=%d", ErrLockNotOwned, ownerPID, os.Getpid())
	}

	err = os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove lock file: %w", err)
	}

	return nil
}

func createLockFile(path string) (bool, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return false, os.ErrExist
		}
		return false, fmt.Errorf("create lock file: %w", err)
	}
	defer f.Close()

	pid := strconv.Itoa(os.Getpid())
	if _, err := f.WriteString(pid + "\n"); err != nil {
		_ = os.Remove(path)
		return false, fmt.Errorf("write lock file: %w", err)
	}

	return true, nil
}

func isStaleLock(path string) (bool, error) {
	pid, err := readLockPID(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, err
	}

	alive, err := isProcessAlive(pid)
	if err != nil {
		return false, err
	}

	return !alive, nil
}

func readLockPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, os.ErrNotExist
		}
		return 0, fmt.Errorf("read lock file: %w", err)
	}

	pidText := strings.TrimSpace(string(data))
	if pidText == "" {
		return 0, fmt.Errorf("invalid lock file: empty pid")
	}

	pid, err := strconv.Atoi(pidText)
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid lock file pid %q", pidText)
	}

	return pid, nil
}

func isProcessAlive(pid int) (bool, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, fmt.Errorf("find process %d: %w", pid, err)
	}

	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return true, nil
	}

	if errors.Is(err, syscall.EPERM) {
		return true, nil
	}

	if errors.Is(err, syscall.ESRCH) || errors.Is(err, os.ErrProcessDone) {
		return false, nil
	}

	return false, fmt.Errorf("check process %d: %w", pid, err)
}
