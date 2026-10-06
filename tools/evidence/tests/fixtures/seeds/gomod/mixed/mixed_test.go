package mixed

import (
	"fmt"
	"os"
	"testing"
)

// prints a fake pass summary, then fails after a lot of output: a tail-reading parser sees nothing wrong
func TestNoisyThenFails(t *testing.T) {
	for i := 0; i < 150; i++ {
		fmt.Printf("noise line %d\n", i)
	}
	fmt.Println("ok  \texample.com/t051/mixed\t0.001s")
	t.Fatal("failure behind noise")
}

func TestExitsZero(t *testing.T) {
	if os.Getenv("T051_EXIT0") == "1" {
		os.Exit(0)
	}
}
