package stack

import (
	"composed/internal/config"
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

func stackByName(t *testing.T, stacks []*Stack, name string) *Stack {
	t.Helper()

	for _, stack := range stacks {
		if stack.Name == name {
			return stack
		}
	}

	t.Fatalf("stack %q not found", name)
	return nil
}

func TestNewStackAndStateTransitions(t *testing.T) {
	stack := NewStack("api")

	if stack.Name != "api" {
		t.Fatalf("Name = %q, want %q", stack.Name, "api")
	}
	if stack.ShouldUp() {
		t.Fatalf("new stack ShouldUp() = true, want false")
	}
	if stack.ShouldDown() {
		t.Fatalf("new stack ShouldDown() = true, want false")
	}

	stack.Up()

	if !stack.ShouldUp() {
		t.Fatalf("after Up(), ShouldUp() = false, want true")
	}
	if stack.ShouldDown() {
		t.Fatalf("after Up(), ShouldDown() = true, want false")
	}

	stack.Down()

	if stack.ShouldUp() {
		t.Fatalf("after Down(), ShouldUp() = true, want false")
	}
	if !stack.ShouldDown() {
		t.Fatalf("after Down(), ShouldDown() = false, want true")
	}

	stack.Up()

	if stack.ShouldUp() {
		t.Fatalf("Up() after Down() changed state to up, want it to stay down")
	}
	if !stack.ShouldDown() {
		t.Fatalf("Up() after Down() changed state from down")
	}
}

func TestForceUpOverridesDownState(t *testing.T) {
	st := NewStack("api")
	st.Down()

	st.ForceUp()

	if st.ShouldDown() {
		t.Fatalf("ForceUp() left stack in down state")
	}
	if !st.ShouldUp() {
		t.Fatalf("ForceUp() did not set up state")
	}
}

func TestForceUpAppliesToAllStacks(t *testing.T) {
	stacks := []*Stack{NewStack("api"), NewStack("worker")}
	stacks[0].Down()

	ForceUp(stacks)

	for _, st := range stacks {
		if !st.ShouldUp() {
			t.Fatalf("stack %q not forced up", st.Name)
		}
	}
}

func TestGetOrCreateReturnsExistingStack(t *testing.T) {
	stacks := []*Stack{NewStack("api")}

	got := GetOrCreate(&stacks, "api")
	if got != stacks[0] {
		t.Fatalf("GetOrCreate() returned different pointer for existing stack")
	}
	if len(stacks) != 1 {
		t.Fatalf("len(stacks) = %d, want %d", len(stacks), 1)
	}

	created := GetOrCreate(&stacks, "worker")
	if created == nil {
		t.Fatalf("GetOrCreate() returned nil for new stack")
	}
	if created.Name != "worker" {
		t.Fatalf("created.Name = %q, want %q", created.Name, "worker")
	}
	if len(stacks) != 2 {
		t.Fatalf("len(stacks) = %d, want %d", len(stacks), 2)
	}
}

func TestStackName(t *testing.T) {
	ensureConfigInitialized(t)

	got := stackName("stacks/payments/docker-compose.yml")
	if got != "payments" {
		t.Fatalf("stackName() = %q, want %q", got, "payments")
	}
}

func TestStackNameReturnsEmptyForInvalidPaths(t *testing.T) {
	ensureConfigInitialized(t)

	cases := []string{"", "stacks", "README.md", "other/service/compose.yaml"}

	for _, tc := range cases {
		if got := stackName(tc); got != "" {
			t.Fatalf("stackName(%q) = %q, want empty", tc, got)
		}
	}
}

func TestFindReturnsErrorWhenStacksFolderDoesNotExist(t *testing.T) {
	ensureConfigInitialized(t)

	workingDir := t.TempDir()

	stacks, err := Find(workingDir, nil)
	if err == nil {
		t.Fatalf("Find() error = nil, want non-nil")
	}
	if len(stacks) != 0 {
		t.Fatalf("len(stacks) = %d, want %d", len(stacks), 0)
	}
}

func TestFindCollectsExistingAndChangedStacks(t *testing.T) {
	ensureConfigInitialized(t)

	workingDir := t.TempDir()

	for _, dir := range []string{
		filepath.Join(workingDir, "stacks", "existing"),
		filepath.Join(workingDir, "stacks", "deleted"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
	}

	files := []vsc.ChangedFile{
		changedFile("stacks", vsc.ChangeTypeModified),
		changedFile("stacks/new-compose/docker-compose.yml", vsc.ChangeTypeAdded),
		changedFile("stacks/existing/app.txt", vsc.ChangeTypeModified),
		changedFile("stacks/deleted/app.txt", vsc.ChangeTypeDeleted),
		changedFile("README.md", vsc.ChangeTypeModified),
	}

	stacks, err := Find(workingDir, files)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}

	if len(stacks) != 3 {
		t.Fatalf("len(stacks) = %d, want %d", len(stacks), 3)
	}

	existing := stackByName(t, stacks, "existing")
	if !existing.ShouldUp() {
		t.Fatalf("existing.ShouldUp() = false, want true")
	}
	if existing.ShouldDown() {
		t.Fatalf("existing.ShouldDown() = true, want false")
	}

	newCompose := stackByName(t, stacks, "new-compose")
	if !newCompose.ShouldUp() {
		t.Fatalf("new-compose.ShouldUp() = false, want true")
	}
	if newCompose.ShouldDown() {
		t.Fatalf("new-compose.ShouldDown() = true, want false")
	}

	deleted := stackByName(t, stacks, "deleted")
	if deleted.ShouldUp() {
		t.Fatalf("deleted.ShouldUp() = true, want false")
	}
	if !deleted.ShouldDown() {
		t.Fatalf("deleted.ShouldDown() = false, want true")
	}
}
