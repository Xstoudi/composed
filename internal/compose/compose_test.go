package compose

import (
	"composed/internal/stack"
	"reflect"
	"testing"
)

func TestPartitionStacks(t *testing.T) {
	api := stack.NewStack("api")
	api.Up()

	worker := stack.NewStack("worker")
	worker.Down()

	web := stack.NewStack("web")
	web.Up()

	stacksToDown, stacksToUp := partitionStacks([]*stack.Stack{api, worker, web})

	if len(stacksToDown) != 1 || stacksToDown[0].Name != "worker" {
		t.Fatalf("stacksToDown = %#v, want only worker", stackNames(stacksToDown))
	}

	upNames := stackNames(stacksToUp)
	wantUp := []string{"api", "web"}
	if !reflect.DeepEqual(upNames, wantUp) {
		t.Fatalf("stackNames(stacksToUp) = %#v, want %#v", upNames, wantUp)
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
