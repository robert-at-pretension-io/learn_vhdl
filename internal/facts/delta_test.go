package facts

import "testing"

func TestComputeDeltaAddsAndRemoves(t *testing.T) {
	prev := Tables{
		Entities: []EntityRow{
			{Name: "a", File: "f.vhd", Line: 1},
		},
		UseClauses: []UseClauseRow{
			{File: "f.vhd", Item: "ieee.std_logic_1164.all", Line: 2},
		},
	}
	next := Tables{
		Entities: []EntityRow{
			{Name: "b", File: "f.vhd", Line: 3},
		},
		UseClauses: []UseClauseRow{
			{File: "f.vhd", Item: "ieee.numeric_std.all", Line: 4},
		},
	}

	delta := ComputeDelta(prev, next)

	if len(delta.Added.Entities) != 1 || delta.Added.Entities[0].Name != "b" {
		t.Fatalf("expected entity b added, got %+v", delta.Added.Entities)
	}
	if len(delta.Removed.Entities) != 1 || delta.Removed.Entities[0].Name != "a" {
		t.Fatalf("expected entity a removed, got %+v", delta.Removed.Entities)
	}
	if len(delta.Added.UseClauses) != 1 || delta.Added.UseClauses[0].Item != "ieee.numeric_std.all" {
		t.Fatalf("expected use clause added, got %+v", delta.Added.UseClauses)
	}
	if len(delta.Removed.UseClauses) != 1 || delta.Removed.UseClauses[0].Item != "ieee.std_logic_1164.all" {
		t.Fatalf("expected use clause removed, got %+v", delta.Removed.UseClauses)
	}
}
