package graph

import "testing"

func TestReachable(t *testing.T) {
	g := Digraph{
		"a": {"b"},
		"b": {"c"},
		"c": nil,
	}
	if !Reachable(g, "a", "c") {
		t.Fatal("expected reachability")
	}
	if Reachable(g, "c", "a") {
		t.Fatal("unexpected reachability")
	}
}

func TestTopologicalSort(t *testing.T) {
	g := Digraph{
		"a": {"b", "c"},
		"b": {"d"},
		"c": {"d"},
		"d": nil,
	}
	order, err := TopologicalSort(g)
	if err != nil {
		t.Fatal(err)
	}
	pos := map[string]int{}
	for i, n := range order {
		pos[n] = i
	}
	if !(pos["a"] < pos["b"] && pos["a"] < pos["c"] && pos["b"] < pos["d"] && pos["c"] < pos["d"]) {
		t.Fatalf("invalid order: %v", order)
	}
}

func TestTopologicalSortCycle(t *testing.T) {
	g := Digraph{
		"a": {"b"},
		"b": {"a"},
	}
	if _, err := TopologicalSort(g); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestSCC(t *testing.T) {
	g := Digraph{
		"a": {"b"},
		"b": {"a", "c"},
		"c": {"d"},
		"d": {"c"},
		"e": nil,
	}
	comps := SCC(g)
	if len(comps) != 3 {
		t.Fatalf("expected 3 components, got %d", len(comps))
	}
}

func TestMaxFlowAndMinCut(t *testing.T) {
	net := FlowNetwork{
		"s": {"a": 3, "b": 2},
		"a": {"b": 1, "t": 2},
		"b": {"t": 3},
		"t": {},
	}
	if flow := MaxFlow(net, "s", "t"); flow != 5 {
		t.Fatalf("expected flow 5, got %d", flow)
	}
	cut := MinCut(net, "s", "t")
	if _, ok := cut["s"]; !ok {
		t.Fatal("source side cut should include s")
	}
	if _, ok := cut["t"]; ok {
		t.Fatal("sink should not be in source-side cut")
	}
}
