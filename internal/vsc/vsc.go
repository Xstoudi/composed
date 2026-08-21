package vsc

import (
	"composed/internal/config"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/transport"
	gitssh "github.com/go-git/go-git/v6/plumbing/transport/ssh"
)

type ChangeType string

const (
	ChangeTypeAdded    ChangeType = "added"
	ChangeTypeModified ChangeType = "modified"
	ChangeTypeDeleted  ChangeType = "deleted"
)

type ChangedFile struct {
	path       string
	changeType ChangeType
}

func (f ChangedFile) Path() string {
	return f.path
}

func (f ChangedFile) ChangeType() ChangeType {
	return f.changeType
}

func (f ChangedFile) StackName() string {
	if f.path == "" {
		return ""
	}

	cleanPath := filepath.ToSlash(filepath.Clean(f.path))
	stacksFolder := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(config.Get().StacksFolder)), "/")

	if cleanPath == stacksFolder {
		return ""
	}

	prefix := stacksFolder + "/"
	if !strings.HasPrefix(cleanPath, prefix) {
		return ""
	}

	relativePath := strings.TrimPrefix(cleanPath, prefix)
	parts := strings.Split(relativePath, "/")
	if len(parts) == 0 || parts[0] == "" || parts[0] == "." {
		return ""
	}

	return parts[0]
}

func CurrentRevision(workingDir string) (string, error) {
	repository, err := openRepository(workingDir)
	if err != nil {
		return "", err
	}

	revision, err := repository.ResolveRevision("HEAD")
	if err != nil {
		return "", err
	}

	return revision.String(), nil
}

func Fetch(workingDir string) ([]ChangedFile, error) {
	repository, err := openRepository(workingDir)
	if err != nil {
		return nil, err
	}

	worktree, err := repository.Worktree()
	if err != nil {
		return nil, err
	}

	status, err := worktree.Status()
	if err != nil {
		return nil, err
	}

	if !status.IsClean() {
		return nil, fmt.Errorf("worktree has local changes")
	}

	headRef, err := repository.Head()
	if err != nil {
		return nil, err
	}

	beforeRef := headRef.Hash()

	auth, err := gitAuth(workingDir, config.Get())
	if err != nil {
		return nil, err
	}

	err = repository.Fetch(&git.FetchOptions{
		RemoteName: "origin",
		Auth:       auth,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil, err
	}

	remoteRefName := plumbing.NewRemoteReferenceName("origin", headRef.Name().Short())
	remoteRef, err := repository.Reference(remoteRefName, true)
	if err != nil {
		return nil, err
	}

	afterRef := remoteRef.Hash()

	if beforeRef.Equal(afterRef) {
		return nil, nil
	}

	err = worktree.Pull(&git.PullOptions{
		RemoteName:    "origin",
		ReferenceName: headRef.Name(),
		SingleBranch:  true,
		Auth:          auth,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil, fmt.Errorf("pull latest changes: %w", err)
	}

	headRef, err = repository.Head()
	if err != nil {
		return nil, err
	}

	afterRef = headRef.Hash()
	if beforeRef.Equal(afterRef) {
		return nil, nil
	}

	beforeCommit, err := repository.CommitObject(beforeRef)
	if err != nil {
		return nil, err
	}

	afterCommit, err := repository.CommitObject(afterRef)
	if err != nil {
		return nil, err
	}

	return changedFilesBetween(beforeCommit, afterCommit)
}

func openRepository(workingDir string) (*git.Repository, error) {
	repository, err := git.PlainOpen(workingDir)
	if err != nil {
		if errors.Is(err, git.ErrRepositoryNotExists) {
			return nil, fmt.Errorf("repository not found: %w", err)
		}
		return nil, err
	}

	return repository, nil
}

func gitAuth(workingDir string, cfg *config.Config) (transport.AuthMethod, error) {
	if cfg == nil || cfg.SSHPrivateKey == "" {
		return nil, nil
	}

	keyPath, err := resolveConfiguredPath(workingDir, cfg.SSHPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("resolve ssh private key path: %w", err)
	}

	passphrase := ""
	if cfg.SSHPrivateKeyPassphraseEnv != "" {
		passphrase = os.Getenv(cfg.SSHPrivateKeyPassphraseEnv)
	}

	sshUser := cfg.SSHUser
	if sshUser == "" {
		sshUser = "git"
	}

	auth, err := gitssh.NewPublicKeysFromFile(sshUser, keyPath, passphrase)
	if err != nil {
		return nil, fmt.Errorf("load ssh private key %q: %w", keyPath, err)
	}

	return auth, nil
}

func resolveConfiguredPath(workingDir, path string) (string, error) {
	if path == "" {
		return "", nil
	}

	if path == "~" {
		home, err := userHomeDir()
		if err != nil {
			return "", err
		}
		return home, nil
	}

	if strings.HasPrefix(path, "~/") {
		home, err := userHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}

	if filepath.IsAbs(path) {
		return path, nil
	}

	return filepath.Join(workingDir, path), nil
}

func userHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if home == "" {
		return "", fmt.Errorf("user home directory not found")
	}

	return home, nil
}

func changedFilesBetween(beforeCommit, afterCommit *object.Commit) ([]ChangedFile, error) {
	patch, err := beforeCommit.Patch(afterCommit)
	if err != nil {
		return nil, err
	}

	changedFiles := make([]ChangedFile, 0, len(patch.FilePatches()))

	for _, filePath := range patch.FilePatches() {
		from, to := filePath.Files()

		var fromPath, toPath string
		if from != nil {
			fromPath = from.Path()
		}
		if to != nil {
			toPath = to.Path()
		}

		if !config.Get().IsProjectFile(toPath) && !config.Get().IsProjectFile(fromPath) {
			continue
		}

		switch {
		case from == nil && to != nil:
			changedFiles = append(changedFiles, ChangedFile{
				path:       toPath,
				changeType: ChangeTypeAdded,
			})
		case from != nil && to == nil:
			changedFiles = append(changedFiles, ChangedFile{
				path:       fromPath,
				changeType: ChangeTypeDeleted,
			})
		case fromPath != toPath:
			changedFiles = append(changedFiles, ChangedFile{
				path:       fromPath,
				changeType: ChangeTypeDeleted,
			})
			changedFiles = append(changedFiles, ChangedFile{
				path:       toPath,
				changeType: ChangeTypeAdded,
			})
		default:
			changedFiles = append(changedFiles, ChangedFile{
				path:       toPath,
				changeType: ChangeTypeModified,
			})
		}
	}

	return changedFiles, nil
}
