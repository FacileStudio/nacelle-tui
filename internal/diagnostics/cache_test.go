package diagnostics

import "testing"

func TestSiftKeepsEverythingOnTheFirstEdit(t *testing.T) {
	first := prior.sift(t.Name(), []finding{
		{loc: "a.go:1:1", rule: "r", msg: "one"},
		{loc: "a.go:2:2", rule: "r", msg: "two"},
	})
	if len(first) != 2 {
		t.Fatalf("first edit kept %d findings, want 2", len(first))
	}
}

func TestSiftSuppressesOnlyWhatWasAlreadyShown(t *testing.T) {
	path := t.Name()
	prior.sift(path, []finding{{loc: "a.go:1:1", rule: "r", msg: "one"}})
	fresh := prior.sift(path, []finding{
		{loc: "a.go:1:1", rule: "r", msg: "one"},
		{loc: "a.go:3:3", rule: "r", msg: "three"},
	})
	if len(fresh) != 1 || fresh[0].msg != "three" {
		t.Fatalf("sift kept %v, want only three", fresh)
	}
}

func TestLearnReplacesTheWholeMemory(t *testing.T) {
	path := t.Name()
	prior.learn(path, []finding{{loc: "a.go:1:1", rule: "r", msg: "one"}})
	prior.learn(path, []finding{{loc: "a.go:2:2", rule: "r", msg: "two"}})
	fresh := prior.sift(path, []finding{
		{loc: "a.go:1:1", rule: "r", msg: "one"},
		{loc: "a.go:2:2", rule: "r", msg: "two"},
	})
	if len(fresh) != 1 || fresh[0].msg != "one" {
		t.Fatalf("sift kept %v, want only one after learn replaced the memory", fresh)
	}
}
