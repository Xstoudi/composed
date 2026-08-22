package compose

import (
	"composed/internal/config"
	"composed/internal/stack"
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"

	composeTypes "github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v5/pkg/api"
	"github.com/docker/compose/v5/pkg/compose"
	mobyClient "github.com/moby/moby/client"
)

type runtime struct {
	ctx          context.Context
	service      api.Compose
	dockerClient mobyClient.APIClient
}

func Synchronize(workingDir string, stacks []*stack.Stack) error {
	stacksToDown, stacksToUp, stacksToPull := partitionStacks(stacks)
	slog.Debug("compose synchronize plan",
		"stacks_total", len(stacks),
		"stacks_down", len(stacksToDown),
		"stacks_up", len(stacksToUp),
		"stacks_pull", len(stacksToPull),
	)

	runtime, err := newRuntime()
	if err != nil {
		return err
	}

	if err := down(runtime, workingDir, stacksToDown); err != nil {
		return fmt.Errorf("failed to down stacks: %w", err)
	}

	stacksWithUpdatedImages, err := pullUpdatedImages(runtime, workingDir, stacksToPull)
	if err != nil {
		return fmt.Errorf("failed to pull stack images: %w", err)
	}

	if err := up(runtime, workingDir, stacksToUp, true); err != nil {
		return fmt.Errorf("failed to up stacks: %w", err)
	}

	if err := up(runtime, workingDir, stacksWithUpdatedImages, false); err != nil {
		return fmt.Errorf("failed to up stacks: %w", err)
	}

	return nil
}

func newRuntime() (*runtime, error) {
	cli, err := command.NewDockerCli()
	if err != nil {
		return nil, err
	}

	err = cli.Initialize(&flags.ClientOptions{})
	if err != nil {
		return nil, err
	}

	service, err := compose.NewComposeService(cli)
	if err != nil {
		return nil, err
	}

	return &runtime{
		ctx:          context.Background(),
		service:      service,
		dockerClient: cli.Client(),
	}, nil
}

func down(runtime *runtime, workingDir string, stacks []*stack.Stack) error {
	if len(stacks) == 0 {
		slog.Debug("compose down skipped", "reason", "no stacks")
		return nil
	}
	slog.Info("compose down start", "stack_count", len(stacks), "stacks", stackNames(stacks))

	for _, stack := range stacks {
		slog.Debug("compose down stack", "stack", stack.Name)
		project, err := runtime.service.LoadProject(runtime.ctx, api.ProjectLoadOptions{
			WorkingDir: filepath.Join(workingDir, config.Get().StacksFolder, stack.Name),
		})
		if err != nil {
			return fmt.Errorf("failed to load compose project: %w", err)
		}

		err = runtime.service.Down(runtime.ctx, stack.Name, api.DownOptions{
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

func up(runtime *runtime, workingDir string, stacks []*stack.Stack, pullBeforeUp bool) error {
	if len(stacks) == 0 {
		slog.Debug("compose up skipped", "reason", "no stacks")
		return nil
	}
	slog.Info("compose up start", "stack_count", len(stacks), "stacks", stackNames(stacks), "pull_before_up", pullBeforeUp)

	for _, stack := range stacks {
		slog.Debug("compose up stack", "stack", stack.Name)
		project, err := runtime.service.LoadProject(runtime.ctx, api.ProjectLoadOptions{
			WorkingDir: filepath.Join(workingDir, config.Get().StacksFolder, stack.Name),
		})
		if err != nil {
			return fmt.Errorf("failed to load compose project: %w", err)
		}

		if pullBeforeUp {
			err = runtime.service.Pull(runtime.ctx, project, api.PullOptions{})
			if err != nil {
				return fmt.Errorf("failed to pull stack %q: %w", stack.Name, err)
			}
		} else {
			disableServicePulls(project)
		}
		err = runtime.service.Up(runtime.ctx, project, api.UpOptions{
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

func pullUpdatedImages(runtime *runtime, workingDir string, stacks []*stack.Stack) ([]*stack.Stack, error) {
	if len(stacks) == 0 {
		slog.Debug("compose pull skipped", "reason", "no stacks")
		return nil, nil
	}
	slog.Info("compose pull start", "stack_count", len(stacks), "stacks", stackNames(stacks))
	stacksToUp := make([]*stack.Stack, 0)

	for _, stack := range stacks {
		slog.Debug("compose pull stack", "stack", stack.Name)
		project, err := runtime.service.LoadProject(runtime.ctx, api.ProjectLoadOptions{
			WorkingDir: filepath.Join(workingDir, config.Get().StacksFolder, stack.Name),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to load compose project: %w", err)
		}

		imageRefs := projectImageRefs(project)
		if len(imageRefs) == 0 {
			slog.Debug("compose pull stack skipped", "stack", stack.Name, "reason", "no service images")
			continue
		}

		before, err := inspectImages(runtime.ctx, runtime.dockerClient, imageRefs)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect images before pull for stack %q: %w", stack.Name, err)
		}

		err = runtime.service.Pull(runtime.ctx, project, api.PullOptions{
			IgnoreFailures: true,
		})
		if err != nil {
			slog.Warn("compose pull stack failed during image check", "stack", stack.Name, "error", err)
		}

		after, err := inspectImages(runtime.ctx, runtime.dockerClient, imageRefs)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect images after pull for stack %q: %w", stack.Name, err)
		}

		if imageSnapshotChanged(before, after) {
			stack.ForceUp()
			stacksToUp = append(stacksToUp, stack)
			slog.Info("compose pull detected image update", "stack", stack.Name, "images", imageRefs)
			continue
		}

		slog.Debug("compose pull no image update", "stack", stack.Name)
	}

	return stacksToUp, nil
}

func partitionStacks(stacks []*stack.Stack) ([]*stack.Stack, []*stack.Stack, []*stack.Stack) {
	stacksToDown := make([]*stack.Stack, 0)
	stacksToUp := make([]*stack.Stack, 0)
	stacksToPull := make([]*stack.Stack, 0)

	for _, stack := range stacks {
		if stack.ShouldDown() {
			stacksToDown = append(stacksToDown, stack)
			continue
		}
		if stack.ShouldUp() {
			stacksToUp = append(stacksToUp, stack)
			continue
		}
		if stack.ShouldPull() {
			stacksToPull = append(stacksToPull, stack)
		}
	}

	return stacksToDown, stacksToUp, stacksToPull
}

func stackNames(stacks []*stack.Stack) []string {
	names := make([]string, 0, len(stacks))
	for _, stack := range stacks {
		names = append(names, stack.Name)
	}
	return names
}

func projectImageRefs(project *composeTypes.Project) []string {
	seen := make(map[string]struct{})
	images := make([]string, 0)

	for _, service := range project.Services {
		if service.Image == "" {
			continue
		}
		if _, ok := seen[service.Image]; ok {
			continue
		}
		seen[service.Image] = struct{}{}
		images = append(images, service.Image)
	}

	sort.Strings(images)
	return images
}

func disableServicePulls(project *composeTypes.Project) {
	for name, service := range project.Services {
		if service.Image == "" {
			continue
		}
		service.PullPolicy = composeTypes.PullPolicyNever
		project.Services[name] = service
	}
}

func inspectImages(ctx context.Context, dockerClient mobyClient.APIClient, imageRefs []string) (map[string]string, error) {
	images := make(map[string]string, len(imageRefs))

	for _, imageRef := range imageRefs {
		inspect, err := dockerClient.ImageInspect(ctx, imageRef)
		if err != nil {
			images[imageRef] = ""
			continue
		}
		images[imageRef] = inspect.ID
	}

	return images, nil
}

func imageSnapshotChanged(before, after map[string]string) bool {
	for imageRef, beforeID := range before {
		if after[imageRef] != beforeID {
			return true
		}
	}

	return false
}
