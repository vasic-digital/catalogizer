package bad

import (
	"fmt"
	"testing"
)

func TestFirstPasses(t *testing.T) { fmt.Println("ok first") }

// seeded failure: expects 5, computes 2-3
func TestSeededFailure(t *testing.T) {
	if got := 2 - 3; got != 5 {
		t.Errorf("seeded failure: got %d want 5", got)
	}
}

func TestSubtests(t *testing.T) {
	t.Run("fine", func(t *testing.T) {})
	t.Run("hidden", func(t *testing.T) { t.Fatal("subtest failure") })
}

func TestLastPasses(t *testing.T) {}
