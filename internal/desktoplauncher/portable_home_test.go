package desktoplauncher

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPortableDesktopHomeUsesInstallRootDataOnWindowsRelease(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows path semantics")
	}
	got, enabled, err := portableDesktopHome("windows", "v1.33.0", "", `F:\P\Reasonix`)
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Fatal("portableDesktopHome enabled = false, want true")
	}
	if got != `F:\P\Reasonix\data` {
		t.Fatalf("portableDesktopHome = %q, want %q", got, `F:\P\Reasonix\data`)
	}
}

func TestPortableDesktopHomeIsDisabledOutsideWindows(t *testing.T) {
	got, enabled, err := portableDesktopHome("linux", "v1.33.0", "", "/opt/reasonix")
	if err != nil {
		t.Fatal(err)
	}
	if enabled || got != "" {
		t.Fatalf("portableDesktopHome = (%q, %v), want disabled", got, enabled)
	}
}

func TestPortableDesktopHomeIsDisabledForDevelopmentBuilds(t *testing.T) {
	for _, version := range []string{"", "dev", " DEV "} {
		got, enabled, err := portableDesktopHome("windows", version, "", `D:\Reasonix`)
		if err != nil {
			t.Fatalf("version %q: %v", version, err)
		}
		if enabled || got != "" {
			t.Fatalf("version %q: portableDesktopHome = (%q, %v), want disabled", version, got, enabled)
		}
	}
}

func TestPortableDesktopHomePreservesExplicitHome(t *testing.T) {
	got, enabled, err := portableDesktopHome("windows", "v1.33.0", `E:\ReasonixData`, `D:\Reasonix`)
	if err != nil {
		t.Fatal(err)
	}
	if enabled || got != "" {
		t.Fatalf("portableDesktopHome = (%q, %v), want explicit home preserved", got, enabled)
	}
}

func TestPortableDesktopHomeRejectsInvalidInstallRoot(t *testing.T) {
	for _, root := range []string{"", "Reasonix"} {
		got, enabled, err := portableDesktopHome("windows", "v1.33.0", "", root)
		if err == nil {
			t.Fatalf("root %q: err = nil, want invalid install root", root)
		}
		if enabled || got != "" {
			t.Fatalf("root %q: portableDesktopHome = (%q, %v), want disabled", root, got, enabled)
		}
	}
}

func TestConfigurePortableDesktopCommandCreatesDataAndSetsChildHome(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command("reasonix-desktop")
	inherited := []string{"PATH=test", "REASONIX_STATE_HOME=state", "REASONIX_CACHE_HOME=cache"}
	if err := configurePortableDesktopCommand(cmd, "windows", "v1.33.0", "", root, inherited); err != nil {
		t.Fatal(err)
	}
	wantHome := filepath.Join(root, "data")
	if info, err := os.Stat(wantHome); err != nil || !info.IsDir() {
		t.Fatalf("data root stat = (%v, %v), want directory", info, err)
	}
	wantEnv := "REASONIX_HOME=" + wantHome
	found := false
	for _, entry := range cmd.Env {
		if entry == wantEnv {
			found = true
		}
	}
	if !found {
		t.Fatalf("cmd.Env = %#v, want %q", cmd.Env, wantEnv)
	}
}

func TestConfigurePortableDesktopCommandReplacesEmptyInheritedHome(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command("reasonix-desktop")
	if err := configurePortableDesktopCommand(cmd, "windows", "v1.33.0", "", root, []string{
		"REASONIX_HOME=",
		"REASONIX_STATE_HOME=state",
		"REASONIX_CACHE_HOME=cache",
	}); err != nil {
		t.Fatal(err)
	}
	homeEntries := 0
	for _, entry := range cmd.Env {
		if len(entry) >= len("REASONIX_HOME=") && entry[:len("REASONIX_HOME=")] == "REASONIX_HOME=" {
			homeEntries++
			if entry != "REASONIX_HOME="+filepath.Join(root, "data") {
				t.Fatalf("REASONIX_HOME entry = %q", entry)
			}
		}
	}
	if homeEntries != 1 {
		t.Fatalf("REASONIX_HOME entries = %d, want 1; env=%#v", homeEntries, cmd.Env)
	}
}

func TestConfigurePortableDesktopCommandLeavesExplicitHomeInherited(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command("reasonix-desktop")
	if err := configurePortableDesktopCommand(cmd, "windows", "v1.33.0", `E:\ReasonixData`, root, []string{
		`REASONIX_HOME=E:\ReasonixData`,
	}); err != nil {
		t.Fatal(err)
	}
	if cmd.Env != nil {
		t.Fatalf("cmd.Env = %#v, want nil inheritance for explicit home", cmd.Env)
	}
	if _, err := os.Stat(filepath.Join(root, "data")); !os.IsNotExist(err) {
		t.Fatalf("explicit home created install-local data root: %v", err)
	}
}

func TestConfigurePortableDesktopCommandFailsWhenDataRootCannotBeCreated(t *testing.T) {
	root := filepath.Join(t.TempDir(), "reasonix.exe")
	if err := os.WriteFile(root, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("reasonix-desktop")
	if err := configurePortableDesktopCommand(cmd, "windows", "v1.33.0", "", root, nil); err == nil {
		t.Fatal("configurePortableDesktopCommand err = nil, want data-root creation failure")
	}
	if cmd.Env != nil {
		t.Fatalf("cmd.Env = %#v after failure, want nil", cmd.Env)
	}
}
