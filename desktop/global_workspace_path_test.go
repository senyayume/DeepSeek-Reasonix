package main

import (
	"path/filepath"
	"testing"

	"reasonix/internal/config"
)

func TestDesktopGlobalWorkspaceUsesConfigPathOwner(t *testing.T) {
	isolateDesktopUserDirs(t)
	if got, want := globalWorkspaceRoot(), config.GlobalWorkspaceRoot(); got != want {
		t.Fatalf("globalWorkspaceRoot() = %q, want %q", got, want)
	}
}

func TestDesktopGlobalWorkspaceUsesRootSessionDirectory(t *testing.T) {
	isolateDesktopUserDirs(t)
	if got, want := desktopSessionDir(globalWorkspaceRoot()), config.SessionDir(); got != want {
		t.Fatalf("desktopSessionDir(global) = %q, want %q", got, want)
	}
}

func TestDesktopExternalProjectKeepsProjectSessionDirectory(t *testing.T) {
	isolateDesktopUserDirs(t)
	project := filepath.Join(t.TempDir(), "Unholy_Maiden")
	if got, want := desktopSessionDir(project), config.ProjectSessionDir(project); got != want {
		t.Fatalf("desktopSessionDir(external) = %q, want %q", got, want)
	}
}
