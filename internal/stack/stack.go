package stack

import (
	"composed/internal/config"
	"composed/internal/vsc"
	"os"
	"path/filepath"
	"strings"
)

type State string

const (
	StateShouldUp   State = "up"
	StateShouldDown State = "down"
	StateNothing    State = "nothing"
)

type Stack struct {
	Name  string
	state State
}

func NewStack(name string) *Stack {
	return &Stack{
		Name:  name,
		state: StateNothing,
	}
}

func (stack *Stack) Up() {
	if stack.state == StateShouldDown {
		return
	}
	stack.state = StateShouldUp
}

func (stack *Stack) ForceUp() {
	stack.state = StateShouldUp
}

func (stack *Stack) Down() {
	stack.state = StateShouldDown
}

func (stack *Stack) ShouldUp() bool {
	return stack.state == StateShouldUp
}

func (stack *Stack) ShouldDown() bool {
	return stack.state == StateShouldDown
}

type StackWork struct {
	Down []*Stack
	Up   []*Stack
}

func GetOrCreate(stacks *[]*Stack, name string) *Stack {
	for _, stack := range *stacks {
		if stack.Name == name {
			return stack
		}
	}
	stack := NewStack(name)
	*stacks = append(*stacks, stack)
	return stack
}

func ForceUp(stacks []*Stack) {
	for _, st := range stacks {
		st.ForceUp()
	}
}

func Find(workingDir string, files []vsc.ChangedFile) ([]*Stack, error) {
	// find all existing stacks
	var folders, err = os.ReadDir(filepath.Join(workingDir, config.Get().StacksFolder))
	if err != nil {
		return []*Stack{}, err
	}

	stacks := make([]*Stack, 0)

	for _, folder := range folders {
		if !folder.IsDir() {
			continue
		}

		GetOrCreate(&stacks, folder.Name())
	}

	// find new stacks
	for _, file := range files {
		name := stackName(file.Path())
		if name == "" {
			continue
		}

		if config.Get().IsProjectFile(file.Path()) && config.Get().IsComposeFile(file.Path()) && file.ChangeType() == vsc.ChangeTypeAdded {
			stack := GetOrCreate(&stacks, name)
			stack.Up()
		}
	}

	// find in changes
	for _, file := range files {
		if !config.Get().IsProjectFile(file.Path()) {
			continue
		}

		name := stackName(file.Path())
		if name == "" {
			continue
		}

		stack := GetOrCreate(&stacks, name)
		if file.ChangeType() == vsc.ChangeTypeDeleted {
			stack.Down()
		}
		stack.Up()
	}

	return stacks, nil
}

func stackName(path string) string {
	if path == "" {
		return ""
	}

	cleanPath := filepath.ToSlash(filepath.Clean(path))
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
