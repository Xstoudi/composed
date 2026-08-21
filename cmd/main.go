package main

import (
	"composed/internal/compose"
	"composed/internal/config"
	"composed/internal/lock"
	"composed/internal/notify"
	"composed/internal/stack"
	"composed/internal/vsc"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

type runOptions struct {
	workingDir string
	dryRun     bool
	force      bool
	verbose    bool
	jsonLogs   bool
}

func main() {
	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	configureLogger(opts.verbose, opts.jsonLogs)

	if err := run(opts); err != nil {
		slog.Error("composed run failed", "error", err)
		os.Exit(1)
	}
}

func run(opts runOptions) (err error) {
	start := time.Now()
	workingDir := ""
	changedFileCount := 0
	stackCount := 0
	downCount := 0
	upCount := 0

	defer func() {
		status := "ok"
		if err != nil {
			status = "error"
		}

		slog.Info("composed run summary",
			"status", status,
			"working_dir", workingDir,
			"dry_run", opts.dryRun,
			"force", opts.force,
			"json_logs", opts.jsonLogs,
			"changed_files", changedFileCount,
			"stacks_total", stackCount,
			"stacks_down", downCount,
			"stacks_up", upCount,
			"duration", time.Since(start).String(),
		)
	}()

	workingDir, err = resolveWorkingDir([]string{opts.workingDir})
	if err != nil {
		return fmt.Errorf("resolve working dir: %w", err)
	}
	slog.Info("resolved working directory", "working_dir", workingDir)

	if err := config.Init(workingDir); err != nil {
		return fmt.Errorf("init config: %w", err)
	}

	cfg := config.Get()

	notifier := notify.New(cfg.NotifyURL)
	notificationSummary := notify.EventSummary{}
	defer func() {
		if opts.dryRun {
			slog.Info("notification skipped", "reason", "dry-run")
			return
		}

		if err != nil {
			notificationSummary.Error = err.Error()
		}

		if !notificationSummary.HasContent() {
			slog.Info("notification skipped", "reason", "empty summary")
			return
		}

		if !notifier.Enabled() {
			slog.Info("notification skipped", "reason", "notify url disabled")
			return
		}

		if sendErr := notifier.Send("Composed update", notificationSummary); sendErr != nil {
			slog.Warn("failed to send notification", "error", sendErr)
			return
		}

		slog.Info("notification sent",
			"created", len(notificationSummary.Created),
			"updated", len(notificationSummary.Updated),
			"deleted", len(notificationSummary.Deleted),
			"error", notificationSummary.Error != "",
		)
	}()

	if err := lock.Lock(cfg.LockFile); err != nil {
		if errors.Is(err, lock.ErrAlreadyLocked) {
			return fmt.Errorf("already running")
		}
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	defer func() {
		if err := lock.Unlock(cfg.LockFile); err != nil {
			slog.Warn("failed to release lock", "error", err)
		}
	}()
	slog.Info("lock acquired", "lock_file", cfg.LockFile)

	files, err := vsc.Fetch(workingDir)
	if err != nil {
		return fmt.Errorf("failed to fetch changes: %w", err)
	}
	changedFileCount = len(files)
	slog.Info("fetched git changes", "changed_files", changedFileCount)

	stacks, err := stack.Find(workingDir, files)
	if err != nil {
		return fmt.Errorf("failed to find stacks: %w", err)
	}

	if opts.force {
		stack.ForceUp(stacks)
		slog.Info("force mode enabled", "stack_count", len(stacks))
	}

	stackCount = len(stacks)
	downCount, upCount = countStackActions(stacks)
	slog.Info("calculated stack actions", "stacks_total", stackCount, "stacks_down", downCount, "stacks_up", upCount)
	slog.Debug("stack action targets", "down_stacks", stackActionNames(stacks, true), "up_stacks", stackActionNames(stacks, false))

	notificationSummary = buildNotificationSummary(files, stacks)

	if opts.dryRun {
		slog.Info("dry-run enabled, skipping compose synchronize")
		return nil
	}

	if err := compose.Synchronize(workingDir, stacks); err != nil {
		return fmt.Errorf("failed to synchronize stacks: %w", err)
	}
	slog.Info("stack synchronization completed")

	return nil
}

func resolveWorkingDir(args []string) (string, error) {
	if len(args) == 0 || args[0] == "" {
		return os.Getwd()
	}

	path := args[0]

	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return filepath.Abs(filepath.Join(workingDir, path))
}

func countStackActions(stacks []*stack.Stack) (int, int) {
	downCount := 0
	upCount := 0

	for _, stack := range stacks {
		if stack.ShouldDown() {
			downCount++
		}
		if stack.ShouldUp() {
			upCount++
		}
	}

	return downCount, upCount
}

func stackActionNames(stacks []*stack.Stack, down bool) []string {
	names := make([]string, 0)
	for _, st := range stacks {
		if down && st.ShouldDown() {
			names = append(names, st.Name)
		}
		if !down && st.ShouldUp() {
			names = append(names, st.Name)
		}
	}
	return names
}

func configureLogger(verbose bool, jsonLogs bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	handlerOpts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler = slog.NewTextHandler(os.Stderr, handlerOpts)
	if jsonLogs {
		handler = slog.NewJSONHandler(os.Stderr, handlerOpts)
	}

	slog.SetDefault(slog.New(handler))
}

func buildNotificationSummary(files []vsc.ChangedFile, stacks []*stack.Stack) notify.EventSummary {
	created := make(map[string]struct{})

	for _, file := range files {
		if file.ChangeType() != vsc.ChangeTypeAdded {
			continue
		}
		if !config.Get().IsComposeFile(file.Path()) {
			continue
		}

		stackName := file.StackName()
		if stackName == "" {
			continue
		}

		created[stackName] = struct{}{}
	}

	summary := notify.EventSummary{
		Created: make([]string, 0),
		Updated: make([]string, 0),
		Deleted: make([]string, 0),
	}

	for _, st := range stacks {
		if st.ShouldDown() {
			summary.Deleted = append(summary.Deleted, st.Name)
			continue
		}

		if st.ShouldUp() {
			if _, isCreated := created[st.Name]; isCreated {
				summary.Created = append(summary.Created, st.Name)
				continue
			}
			summary.Updated = append(summary.Updated, st.Name)
		}
	}

	return summary
}

func parseArgs(args []string) (runOptions, error) {
	fs := flag.NewFlagSet("composed", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	opts := runOptions{}
	fs.BoolVar(&opts.dryRun, "dry-run", false, "compute and log actions without running docker compose")
	fs.BoolVar(&opts.force, "force", false, "force all discovered stacks to be started")
	fs.BoolVar(&opts.verbose, "verbose", false, "enable debug logs")
	fs.BoolVar(&opts.jsonLogs, "json", false, "emit logs in JSON format")

	if err := fs.Parse(args); err != nil {
		return runOptions{}, err
	}

	remaining := fs.Args()
	if len(remaining) > 1 {
		return runOptions{}, fmt.Errorf("unexpected arguments: %v", remaining[1:])
	}

	if len(remaining) == 1 {
		opts.workingDir = remaining[0]
	}

	return opts, nil
}
