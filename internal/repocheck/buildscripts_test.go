// SPDX-License-Identifier: GPL-3.0-only

package repocheck

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The three build scripts (build-deb.sh, build-rpm.sh, windows/build-windows.ps1)
// are used the same way as the ones of Planche-Contact: same options, same
// handling of VERSION. They share no code, so these tests keep them aligned.

// Each script offers the same six options, in the syntax of its shell.
func TestBuildScriptsOfferTheSameOptions(t *testing.T) {
	root := repoRoot(t)
	for _, script := range []string{"build-deb.sh", "build-rpm.sh"} {
		text := read(t, filepath.Join(root, script))
		for _, option := range []string{"--major)", "--minor)", "--patch)", "--no-version-change)", "--clean)", "--help|-h)"} {
			if !strings.Contains(text, option) {
				t.Errorf("%s does not handle the option %s", script, strings.TrimSuffix(option, ")"))
			}
		}
	}
	text := read(t, filepath.Join(root, "windows", "build-windows.ps1"))
	for _, parameter := range []string{"[switch]$Major", "[switch]$Minor", "[switch]$Patch", "[switch]$NoVersionChange", "[switch]$Clean", "[switch]$Help"} {
		if !strings.Contains(text, parameter) {
			t.Errorf("windows/build-windows.ps1 does not declare %s", parameter)
		}
	}
	relay := read(t, filepath.Join(root, "build-windows.ps1"))
	if !strings.Contains(relay, `windows\build-windows.ps1" @args`) {
		t.Error("build-windows.ps1 at the root must pass its arguments on to windows\\build-windows.ps1")
	}
}

var nextVersion = regexp.MustCompile(`(?s)\nnext_version\(\) \{.*?\n\}\n`)

// build-deb.sh and build-rpm.sh compute the next version with the very same
// function (the PowerShell script has its own, checked by the CI).
func TestBuildScriptsComputeVersionsTheSameWay(t *testing.T) {
	root := repoRoot(t)
	deb := nextVersion.FindString(read(t, filepath.Join(root, "build-deb.sh")))
	rpm := nextVersion.FindString(read(t, filepath.Join(root, "build-rpm.sh")))
	if deb == "" || rpm == "" {
		t.Fatal("next_version() not found in build-deb.sh or build-rpm.sh")
	}
	if deb != rpm {
		t.Error("next_version() differs between build-deb.sh and build-rpm.sh")
	}
}

// VERSION is written once, after the package exists: a build that fails must
// never leave a version without its package. build-windows.sh, which the
// PowerShell script calls, never writes it.
func TestVersionIsWrittenOnlyAfterTheBuild(t *testing.T) {
	root := repoRoot(t)
	cases := []struct {
		script, write, built string
	}{
		{"build-deb.sh", `> "$VERSION_FILE"`, "dpkg-deb --root-owner-group"},
		{"build-rpm.sh", `> "$VERSION_FILE"`, "rpmbuild -bb"},
		{filepath.Join("windows", "build-windows.ps1"), "WriteAllText($VersionFile", `Invoke-Msys "bash $BuildScript$installDeps --version`},
	}
	for _, c := range cases {
		text := read(t, filepath.Join(root, c.script))
		if n := strings.Count(text, c.write); n != 1 {
			t.Errorf("%s writes VERSION %d times, want once", c.script, n)
			continue
		}
		build := strings.Index(text, c.built)
		if build < 0 {
			t.Errorf("%s: cannot find the build command %q", c.script, c.built)
			continue
		}
		if strings.Index(text, c.write) < build {
			t.Errorf("%s writes VERSION before building the package", c.script)
		}
	}
	if strings.Contains(read(t, filepath.Join(root, "windows", "build-windows.sh")), `> "$PROJECT_DIR/VERSION"`) {
		t.Error("windows/build-windows.sh must not write VERSION: build-windows.ps1 does it after the build")
	}
}
