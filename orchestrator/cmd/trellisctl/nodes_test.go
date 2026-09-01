package main

import (
	"strings"
	"testing"

	"github.com/clofour/trellis/internal/api"
	"github.com/google/uuid"
)

func TestResolveNodeReference(t *testing.T) {
	first := api.NodeResponse{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Host: "node-a", Port: 8128}
	second := api.NodeResponse{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Host: "node-b", Port: 8128}
	nodes := api.NodeListResponse{first, second}

	for _, ref := range []string{"node-a", "node-a:8128", "11111111", first.ID.String()} {
		got, err := resolveNodeReference(nodes, ref)
		if err != nil {
			t.Fatalf("resolve %q: %v", ref, err)
		}
		if got.ID != first.ID {
			t.Fatalf("resolve %q = %s", ref, got.ID)
		}
	}
}

func TestResolveNodeReferenceRejectsAmbiguousPrefix(t *testing.T) {
	nodes := api.NodeListResponse{
		{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Host: "a", Port: 8128},
		{ID: uuid.MustParse("11112222-2222-2222-2222-222222222222"), Host: "b", Port: 8128},
	}
	_, err := resolveNodeReference(nodes, "1111")
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguity, got %v", err)
	}
}
