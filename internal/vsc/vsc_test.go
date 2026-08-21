package vsc

import (
	"composed/internal/config"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
)

var (
	initConfigOnce sync.Once
	initConfigErr  error
)

func ensureConfigInitialized(t *testing.T) {
	t.Helper()

	initConfigOnce.Do(func() {
		initConfigErr = config.Init(t.TempDir())
	})

	if initConfigErr != nil {
		t.Fatalf("config.Init() error = %v", initConfigErr)
	}
}

func writeAndCommit(t *testing.T, repo *git.Repository, repoDir, path, content, message string) plumbing.Hash {
	t.Helper()

	fullPath := filepath.Join(repoDir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(fullPath), err)
	}

	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", fullPath, err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatalf("repo.Worktree() error = %v", err)
	}

	if _, err := worktree.Add(path); err != nil {
		t.Fatalf("worktree.Add(%q) error = %v", path, err)
	}

	hash, err := worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("worktree.Commit() error = %v", err)
	}

	return hash
}

func headHash(t *testing.T, repo *git.Repository) plumbing.Hash {
	t.Helper()

	head, err := repo.Head()
	if err != nil {
		t.Fatalf("repo.Head() error = %v", err)
	}

	return head.Hash()
}

func TestFetchPullsAndReturnsChangedFiles(t *testing.T) {
	ensureConfigInitialized(t)

	root := t.TempDir()
	originDir := filepath.Join(root, "origin")
	if err := os.MkdirAll(originDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(origin) error = %v", err)
	}

	originRepo, err := git.PlainInit(originDir, false)
	if err != nil {
		t.Fatalf("git.PlainInit() error = %v", err)
	}

	firstCommit := writeAndCommit(t, originRepo, originDir, "stacks/api/compose.yaml", "services:\n  api:\n    image: api:v1\n", "initial")

	cloneDir := filepath.Join(root, "clone")
	cloneRepo, err := git.PlainClone(cloneDir, &git.CloneOptions{URL: originDir})
	if err != nil {
		t.Fatalf("git.PlainClone() error = %v", err)
	}

	if got := headHash(t, cloneRepo); got != firstCommit {
		t.Fatalf("clone head = %s, want %s", got, firstCommit)
	}

	secondCommit := writeAndCommit(t, originRepo, originDir, "stacks/api/compose.yaml", "services:\n  api:\n    image: api:v2\n", "update image")

	files, err := Fetch(cloneDir)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("len(files) = %d, want %d", len(files), 1)
	}
	if files[0].Path() != "stacks/api/compose.yaml" {
		t.Fatalf("files[0].Path() = %q, want %q", files[0].Path(), "stacks/api/compose.yaml")
	}
	if files[0].ChangeType() != ChangeTypeModified {
		t.Fatalf("files[0].ChangeType() = %q, want %q", files[0].ChangeType(), ChangeTypeModified)
	}

	if got := headHash(t, cloneRepo); got != secondCommit {
		t.Fatalf("clone head after Fetch = %s, want %s", got, secondCommit)
	}
}

func TestFetchRejectsDirtyWorktree(t *testing.T) {
	ensureConfigInitialized(t)

	root := t.TempDir()
	originDir := filepath.Join(root, "origin")
	if err := os.MkdirAll(originDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(origin) error = %v", err)
	}

	originRepo, err := git.PlainInit(originDir, false)
	if err != nil {
		t.Fatalf("git.PlainInit() error = %v", err)
	}

	writeAndCommit(t, originRepo, originDir, "stacks/api/compose.yaml", "services:\n  api:\n    image: api:v1\n", "initial")

	cloneDir := filepath.Join(root, "clone")
	_, err = git.PlainClone(cloneDir, &git.CloneOptions{URL: originDir})
	if err != nil {
		t.Fatalf("git.PlainClone() error = %v", err)
	}

	writeAndCommit(t, originRepo, originDir, "stacks/api/compose.yaml", "services:\n  api:\n    image: api:v2\n", "remote update")

	dirtyPath := filepath.Join(cloneDir, "stacks", "api", "compose.yaml")
	if err := os.WriteFile(dirtyPath, []byte("services:\n  api:\n    image: api:local\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(dirty) error = %v", err)
	}

	_, err = Fetch(cloneDir)
	if err == nil {
		t.Fatalf("Fetch() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "worktree has local changes") {
		t.Fatalf("Fetch() error = %q, want message containing %q", err.Error(), "worktree has local changes")
	}
}

func TestFetchReturnsNilWhenNoRemoteChanges(t *testing.T) {
	ensureConfigInitialized(t)

	root := t.TempDir()
	originDir := filepath.Join(root, "origin")
	if err := os.MkdirAll(originDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(origin) error = %v", err)
	}

	originRepo, err := git.PlainInit(originDir, false)
	if err != nil {
		t.Fatalf("git.PlainInit() error = %v", err)
	}

	writeAndCommit(t, originRepo, originDir, "stacks/api/compose.yaml", "services:\n  api:\n    image: api:v1\n", "initial")

	cloneDir := filepath.Join(root, "clone")
	_, err = git.PlainClone(cloneDir, &git.CloneOptions{URL: originDir})
	if err != nil {
		t.Fatalf("git.PlainClone() error = %v", err)
	}

	files, err := Fetch(cloneDir)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if files != nil {
		t.Fatalf("Fetch() files = %#v, want nil", files)
	}
}

func TestGitAuthReturnsNilWhenNoSSHPrivateKeyConfigured(t *testing.T) {
	auth, err := gitAuth(t.TempDir(), &config.Config{})
	if err != nil {
		t.Fatalf("gitAuth() error = %v", err)
	}
	if auth != nil {
		t.Fatalf("gitAuth() auth = %#v, want nil", auth)
	}
}

func TestGitAuthResolvesRelativeSSHPrivateKeyFromWorkingDir(t *testing.T) {
	workingDir := t.TempDir()

	_, err := gitAuth(workingDir, &config.Config{
		SSHPrivateKey: "keys/deploy",
		SSHUser:       "git",
	})
	if err == nil {
		t.Fatalf("gitAuth() error = nil, want non-nil")
	}

	want := filepath.Join(workingDir, "keys", "deploy")
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("gitAuth() error = %q, want message containing %q", err.Error(), want)
	}
}

func TestChangedFileStackName(t *testing.T) {
	ensureConfigInitialized(t)

	cases := []struct {
		name string
		path string
		want string
	}{
		{name: "valid stack file", path: "stacks/api/compose.yaml", want: "api"},
		{name: "stacks root", path: "stacks", want: ""},
		{name: "outside stacks folder", path: "README.md", want: ""},
		{name: "relative traversal normalized", path: "stacks/../stacks/worker/compose.yaml", want: "worker"},
	}

	for _, tc := range cases {
		got := ChangedFile{path: tc.path}.StackName()
		if got != tc.want {
			t.Fatalf("%s: StackName() = %q, want %q", tc.name, got, tc.want)
		}
	}
}
