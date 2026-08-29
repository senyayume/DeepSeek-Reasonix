package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobalWorkspaceRootUsesReasonixHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("REASONIX_HOME", home)
	if got, want := GlobalWorkspaceRoot(), filepath.Join(home, "global-workspace"); got != want {
		t.Fatalf("GlobalWorkspaceRoot() = %q, want %q", got, want)
	}
}

func TestProjectSessionDirUsesRootSessionsForGlobalWorkspace(t *testing.T) {
	home := t.TempDir()
	state := t.TempDir()
	t.Setenv("REASONIX_HOME", home)
	t.Setenv("REASONIX_STATE_HOME", state)
	if got, want := ProjectSessionDir(GlobalWorkspaceRoot()), filepath.Join(state, "sessions"); got != want {
		t.Fatalf("ProjectSessionDir(global) = %q, want %q", got, want)
	}
}

func TestProjectSessionDirRecognizesWindowsGlobalWorkspaceCaseInsensitively(t *testing.T) {
	setRuntimeGOOS(t, "windows")
	home := t.TempDir()
	t.Setenv("REASONIX_HOME", home)
	t.Setenv("REASONIX_STATE_HOME", "")
	upperRoot := strings.ToUpper(GlobalWorkspaceRoot())
	if got, want := ProjectSessionDir(upperRoot), SessionDir(); got != want {
		t.Fatalf("ProjectSessionDir(upper global) = %q, want %q", got, want)
	}
}

func TestProjectSessionDirKeepsExternalProjectsIsolated(t *testing.T) {
	state := t.TempDir()
	t.Setenv("REASONIX_HOME", t.TempDir())
	t.Setenv("REASONIX_STATE_HOME", state)
	project := filepath.Join(t.TempDir(), "Unholy_Maiden")
	want := filepath.Join(state, "projects", WorkspaceSlug(project), "sessions")
	if got := ProjectSessionDir(project); got != want {
		t.Fatalf("ProjectSessionDir(external) = %q, want %q", got, want)
	}
}
