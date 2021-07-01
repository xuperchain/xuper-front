package clixchain

import (
	"testing"
)

func TestGroup(t *testing.T) {
	groupA := Group{
		GroupID:    "xuper",
		Admin:      []string{"A", "B"},
		Identities: []string{"A", "B", "C"},
	}
	set := make(map[string]bool)
	result := groupA.GetAddrs(set)
	for _, a := range result {
		if a != "A" && a != "B" && a != "C" {
			t.Errorf("group GetAddrs error")
		}
	}
	groupB := Group{
		GroupID:    "xuper",
		Admin:      []string{"A", "B"},
		Identities: []string{"D"},
	}
	result = groupB.GetAddrs(set)
	if len(result) != 1 {
		t.Errorf("group GetAddrs error")
	}
}

func TestGroupCache(t *testing.T) {
	gc := groupCache{
		close: make(chan int64, 1),
		ch:    make(chan []string, 1),
		value: []string{"A", "B", "C"},
	}
	gc.Start()
	gc.ch <- []string{"D"}
	resultB := gc.Get()
	if len(resultB) != 1 {
		t.Errorf("groupCache Get error, result = %v", resultB)
	}
	gc.close <- 1
}
