package main

import (
	"composed/internal/config"
	"composed/internal/stack"
	"composed/internal/vsc"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"unsafe"
)

var initConfigOnce sync.Once

func ensureConfigInitialized(t *testing.T) {
	t.Helper()

	initConfigOnce.Do(func() {
		if err := config.Init(t.TempDir()); err != nil {
			t.Fatalf("config.Init() error = %v", err)
		}
	})
}

func changedFile(path string, changeType vsc.ChangeType) vsc.ChangedFile {
	file := vsc.ChangedFile{}

	value := reflect.ValueOf(&file).Elem()

	pathField := value.FieldByName("path")
	reflect.NewAt(pathField.Type(), unsafe.Pointer(pathField.UnsafeAddr())).Elem().SetString(path)

	changeTypeField := value.FieldByName("changeType")
	reflect.NewAt(changeTypeField.Type(), unsafe.Pointer(changeTypeField.UnsafeAddr())).Elem().Set(reflect.ValueOf(changeType))

	return file
}

func TestCountStackActions(t *testing.T) {
	up := stack.NewStack("up")
	up.Up()

	down := stack.NewStack("down")
	down.Down()

	nothing := stack.NewStack("nothing")

	downCount, upCount := countStackActions([]*stack.Stack{up, down, nothing})
	if downCount != 1 {
		t.Fatalf("downCount = %d, want %d", downCount, 1)
	}
	if upCount != 1 {
		t.Fatalf("upCount = %d, want %d", upCount, 1)
	}
}

func TestResolveWorkingDirWithAbsolutePath(t *testing.T) {
	got, err := resolveWorkingDir([]string{"/tmp/../tmp/composed"})
	if err != nil {
		t.Fatalf("resolveWorkingDir() error = %v", err)
	}

	want := filepath.Clean("/tmp/../tmp/composed")
	if got != want {
		t.Fatalf("resolveWorkingDir() = %q, want %q", got, want)
	}
}

func TestResolveWorkingDirWithRelativePath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	temp := t.TempDir()
	if err := os.Chdir(temp); err != nil {
		t.Fatalf("os.Chdir() error = %v", err)
	}

	got, err := resolveWorkingDir([]string{"nested/dir"})
	if err != nil {
		t.Fatalf("resolveWorkingDir() error = %v", err)
	}

	want := filepath.Join(temp, "nested", "dir")
	if got != want {
		t.Fatalf("resolveWorkingDir() = %q, want %q", got, want)
	}
}

func TestParseArgsFlagsAndWorkingDir(t *testing.T) {
	opts, err := parseArgs([]string{"--dry-run", "--force", "--verbose", "--json", "/tmp/work"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}

	if !opts.dryRun || !opts.force || !opts.verbose || !opts.jsonLogs {
		t.Fatalf("parseArgs() flags = %#v, want all flags true", opts)
	}
	if opts.workingDir != "/tmp/work" {
		t.Fatalf("parseArgs() workingDir = %q, want %q", opts.workingDir, "/tmp/work")
	}
}

func TestParseArgsDefaultJSONDisabled(t *testing.T) {
	opts, err := parseArgs(nil)
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}

	if opts.jsonLogs {
		t.Fatalf("parseArgs() jsonLogs = true, want false")
	}
}

func TestParseArgsRejectsMultiplePositionalArgs(t *testing.T) {
	_, err := parseArgs([]string{"one", "two"})
	if err == nil {
		t.Fatalf("parseArgs() error = nil, want non-nil")
	}
}

func TestStackActionNames(t *testing.T) {
	api := stack.NewStack("api")
	api.Up()

	worker := stack.NewStack("worker")
	worker.Down()

	stacks := []*stack.Stack{api, worker}

	gotDown := stackActionNames(stacks, true)
	if !reflect.DeepEqual(gotDown, []string{"worker"}) {
		t.Fatalf("stackActionNames(down) = %#v, want %#v", gotDown, []string{"worker"})
	}

	gotUp := stackActionNames(stacks, false)
	if !reflect.DeepEqual(gotUp, []string{"api"}) {
		t.Fatalf("stackActionNames(up) = %#v, want %#v", gotUp, []string{"api"})
	}
}

func TestBuildNotificationSummary(t *testing.T) {
	ensureConfigInitialized(t)

	api := stack.NewStack("api")
	api.Up()

	worker := stack.NewStack("worker")
	worker.Up()

	legacy := stack.NewStack("legacy")
	legacy.Down()

	files := []vsc.ChangedFile{
		changedFile("stacks/api/compose.yaml", vsc.ChangeTypeAdded),
		changedFile("stacks/worker/app.txt", vsc.ChangeTypeModified),
	}

	summary := buildNotificationSummary(files, []*stack.Stack{api, worker, legacy})

	if !reflect.DeepEqual(summary.Created, []string{"api"}) {
		t.Fatalf("Created = %#v, want %#v", summary.Created, []string{"api"})
	}
	if !reflect.DeepEqual(summary.Updated, []string{"worker"}) {
		t.Fatalf("Updated = %#v, want %#v", summary.Updated, []string{"worker"})
	}
	if !reflect.DeepEqual(summary.Deleted, []string{"legacy"}) {
		t.Fatalf("Deleted = %#v, want %#v", summary.Deleted, []string{"legacy"})
	}
}
