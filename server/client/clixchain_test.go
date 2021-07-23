package clixchain

import (
	"testing"
)

func TestGroup(t *testing.T) {
	groupA := group{
		GroupID:    "xuper",
		Admin:      []string{"A", "B"},
		Identities: []string{"A", "B", "C"},
	}
	result := groupA.GetAddrs()
	for _, a := range result {
		if a != "A" && a != "B" && a != "C" {
			t.Errorf("group GetAddrs error")
		}
	}
}

func TestGroupCache(t *testing.T) {
	gc := groupCache{
		value: []string{"A", "B", "C"},
	}
	gc.put([]string{"D"})
	resultB := gc.get()
	if len(resultB) != 1 {
		t.Errorf("groupCache Get error, result = %v", resultB)
	}
}
