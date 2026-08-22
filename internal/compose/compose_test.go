package compose

import (
	"composed/internal/stack"
	"reflect"
	"testing"

	composeTypes "github.com/compose-spec/compose-go/v2/types"
)

func TestPartitionStacks(t *testing.T) {
	api := stack.NewStack("api")
	api.Up()

	worker := stack.NewStack("worker")
	worker.Down()

	web := stack.NewStack("web")
	web.Up()

	idle := stack.NewStack("idle")

	stacksToDown, stacksToUp, stacksToPull := partitionStacks([]*stack.Stack{api, worker, web, idle})

	if len(stacksToDown) != 1 || stacksToDown[0].Name != "worker" {
		t.Fatalf("stacksToDown = %#v, want only worker", stackNames(stacksToDown))
	}

	upNames := stackNames(stacksToUp)
	wantUp := []string{"api", "web"}
	if !reflect.DeepEqual(upNames, wantUp) {
		t.Fatalf("stackNames(stacksToUp) = %#v, want %#v", upNames, wantUp)
	}

	pullNames := stackNames(stacksToPull)
	wantPull := []string{"idle"}
	if !reflect.DeepEqual(pullNames, wantPull) {
		t.Fatalf("stackNames(stacksToPull) = %#v, want %#v", pullNames, wantPull)
	}
}

func TestStackNames(t *testing.T) {
	stacks := []*stack.Stack{stack.NewStack("api"), stack.NewStack("worker")}

	got := stackNames(stacks)
	want := []string{"api", "worker"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stackNames() = %#v, want %#v", got, want)
	}
}

func TestProjectImageRefsReturnsSortedUniqueServiceImages(t *testing.T) {
	project := &composeTypes.Project{
		Services: composeTypes.Services{
			"api": {
				Name:  "api",
				Image: "example/api:latest",
			},
			"worker": {
				Name:  "worker",
				Image: "example/worker:latest",
			},
			"api-copy": {
				Name:  "api-copy",
				Image: "example/api:latest",
			},
			"local-build": {
				Name: "local-build",
			},
		},
	}

	got := projectImageRefs(project)
	want := []string{"example/api:latest", "example/worker:latest"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("projectImageRefs() = %#v, want %#v", got, want)
	}
}

func TestImageSnapshotChanged(t *testing.T) {
	before := map[string]string{
		"example/api:latest":    "sha256:old",
		"example/worker:latest": "sha256:same",
	}

	unchanged := map[string]string{
		"example/api:latest":    "sha256:old",
		"example/worker:latest": "sha256:same",
	}
	if imageSnapshotChanged(before, unchanged) {
		t.Fatalf("imageSnapshotChanged() = true, want false")
	}

	changed := map[string]string{
		"example/api:latest":    "sha256:new",
		"example/worker:latest": "sha256:same",
	}
	if !imageSnapshotChanged(before, changed) {
		t.Fatalf("imageSnapshotChanged() = false, want true")
	}
}
