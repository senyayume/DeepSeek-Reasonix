package desktoplauncher

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func portableDesktopHome(goos, buildVersion, existingHome, installRoot string) (string, bool, error) {
	if goos != "windows" {
		return "", false, nil
	}
	buildVersion = strings.TrimSpace(buildVersion)
	if buildVersion == "" || strings.EqualFold(buildVersion, "dev") {
		return "", false, nil
	}
	if strings.TrimSpace(existingHome) != "" {
		return "", false, nil
	}
	installRoot = strings.TrimSpace(installRoot)
	if installRoot == "" || !filepath.IsAbs(installRoot) {
		return "", false, fmt.Errorf("desktoplauncher: invalid install root %q", installRoot)
	}
	return filepath.Join(filepath.Clean(installRoot), "data"), true, nil
}

func configurePortableDesktopCommand(cmd *exec.Cmd, goos, buildVersion, existingHome, installRoot string, inheritedEnv []string) error {
	home, enabled, err := portableDesktopHome(goos, buildVersion, existingHome, installRoot)
	if err != nil || !enabled {
		return err
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return fmt.Errorf("desktoplauncher: create data root %s: %w", home, err)
	}
	env := make([]string, 0, len(inheritedEnv)+1)
	for _, entry := range inheritedEnv {
		key, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "REASONIX_HOME") {
			continue
		}
		env = append(env, entry)
	}
	cmd.Env = append(env, "REASONIX_HOME="+home)
	return nil
}
