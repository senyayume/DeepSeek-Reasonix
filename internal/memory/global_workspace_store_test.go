package memory

import (
	"path/filepath"
	"testing"

	"reasonix/internal/config"
)

func TestStoreForGlobalWorkspaceUsesRootMemory(t *testing.T) {
	home := t.TempDir()
	state := t.TempDir()
	t.Setenv("REASONIX_HOME", home)
	store := StoreFor(state, config.GlobalWorkspaceRoot())
	if got, want := store.Dir, filepath.Join(state, "memory"); got != want {
		t.Fatalf("StoreFor(global).Dir = %q, want %q", got, want)
	}
	if got, want := store.GlobalDir, filepath.Join(state, "memory", "global"); got != want {
		t.Fatalf("StoreFor(global).GlobalDir = %q, want %q", got, want)
	}
}

func TestStoreForExternalProjectKeepsProjectMemory(t *testing.T) {
	state := t.TempDir()
	project := filepath.Join(t.TempDir(), "Unholy_Maiden")
	store := StoreFor(state, project)
	want := filepath.Join(state, "projects", config.WorkspaceSlug(absOf(project)), "memory")
	if store.Dir != want {
		t.Fatalf("StoreFor(external).Dir = %q, want %q", store.Dir, want)
	}
}
