package compose

import (
	"composed/internal/config"
	"composed/internal/stack"
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v5/pkg/api"
	"github.com/docker/compose/v5/pkg/compose"
)

func Synchronize(workingDir string, stacks []*stack.Stack) error {
	stacksToDown, stacksToUp := partitionStacks(stacks)
	slog.Debug("compose synchronize plan",
		"stacks_total", len(stacks),
		"stacks_down", len(stacksToDown),
		"stacks_up", len(stacksToUp),
	)

	if err := down(workingDir, stacksToDown); err != nil {
		return fmt.Errorf("failed to down stacks: %w", err)
	}

	if err := up(workingDir, stacksToUp); err != nil {
		return fmt.Errorf("failed to up stacks: %w", err)
	}

	return nil
}

func down(workingDir string, stacks []*stack.Stack) error {
	if len(stacks) == 0 {
		slog.Debug("compose down skipped", "reason", "no stacks")
		return nil
	}
	slog.Info("compose down start", "stack_count", len(stacks), "stacks", stackNames(stacks))

	ctx := context.Background()

	cli, err := command.NewDockerCli()
	if err != nil {
		return err
	}

	err = cli.Initialize(&flags.ClientOptions{})
	if err != nil {
		return err
	}

	service, err := compose.NewComposeService(cli)
	if err != nil {
		return err
	}

	for _, stack := range stacks {
		slog.Debug("compose down stack", "stack", stack.Name)
		project, err := service.LoadProject(ctx, api.ProjectLoadOptions{
			WorkingDir: filepath.Join(workingDir, config.Get().StacksFolder, stack.Name),
		})
		if err != nil {
			return fmt.Errorf("failed to load compose project: %w", err)
		}

		err = service.Down(ctx, stack.Name, api.DownOptions{
			RemoveOrphans: true,
			Project:       project,
		})
		if err != nil {
			return fmt.Errorf("failed to stop stack %q: %w", stack.Name, err)
		}
		slog.Info("compose down completed", "stack", stack.Name)
	}

	return nil
}

func up(workingDir string, stacks []*stack.Stack) error {
	if len(stacks) == 0 {
		slog.Debug("compose up skipped", "reason", "no stacks")
		return nil
	}
	slog.Info("compose up start", "stack_count", len(stacks), "stacks", stackNames(stacks))

	ctx := context.Background()

	cli, err := command.NewDockerCli()
	if err != nil {
		return err
	}

	err = cli.Initialize(&flags.ClientOptions{})
	if err != nil {
		return err
	}

	service, err := compose.NewComposeService(cli)
	if err != nil {
		return err
	}

	for _, stack := range stacks {
		slog.Debug("compose up stack", "stack", stack.Name)
		project, err := service.LoadProject(ctx, api.ProjectLoadOptions{
			WorkingDir: filepath.Join(workingDir, config.Get().StacksFolder, stack.Name),
		})
		if err != nil {
			return fmt.Errorf("failed to load compose project: %w", err)
		}

		err = service.Pull(ctx, project, api.PullOptions{})
		if err != nil {
			return fmt.Errorf("failed to pull stack %q: %w", stack.Name, err)
		}
		err = service.Up(ctx, project, api.UpOptions{
			Create: api.CreateOptions{
				Build: &api.BuildOptions{
					Pull: true,
				},
				RemoveOrphans: true,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to start stack %q: %w", stack.Name, err)
		}
		slog.Info("compose up completed", "stack", stack.Name)
	}

	return nil
}

func partitionStacks(stacks []*stack.Stack) ([]*stack.Stack, []*stack.Stack) {
	stacksToDown := make([]*stack.Stack, 0)
	stacksToUp := make([]*stack.Stack, 0)

	for _, stack := range stacks {
		if stack.ShouldDown() {
			stacksToDown = append(stacksToDown, stack)
		}
		if stack.ShouldUp() {
			stacksToUp = append(stacksToUp, stack)
		}
	}

	return stacksToDown, stacksToUp
}

func stackNames(stacks []*stack.Stack) []string {
	names := make([]string, 0, len(stacks))
	for _, stack := range stacks {
		names = append(names, stack.Name)
	}
	return names
}
