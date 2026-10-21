// SPDX-License-Identifier: GPL-3.0-only

package repocheck

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	changelogRelease = regexp.MustCompile(`(?m)^## \[(\d[^\]]*)\] - (\d{4}-\d{2}-\d{2})$`)
	appstreamRelease = regexp.MustCompile(`<release version="([^"]+)" date="([^"]+)"`)
	manPageHeader    = regexp.MustCompile(`(?m)^\.TH IMAGEPRINTLAYOUT 1 "([^"]*)" "imageprintlayout ([^"]*)"`)
)

// Every file that carries the version number says the same one, the one of
// VERSION, and the same release date: the CHANGELOG section, the AppStream
// release (shown by software centres) and the header of the manual page.
// A new version (build-deb.sh --minor...) is not finished until they all say
// it; this test fails until they do. A pre-release version (0.1.0-dev) has
// not been published and is not in them yet.
func TestVersionIsInEveryFileThatNamesIt(t *testing.T) {
	root := repoRoot(t)
	version := strings.TrimSpace(read(t, filepath.Join(root, "VERSION")))
	if strings.Contains(version, "-") {
		t.Skipf("VERSION %s is a pre-release: no release to find in the other files", version)
	}

	// CHANGELOG.md: the newest dated section is the current version.
	releases := changelogRelease.FindAllStringSubmatch(read(t, filepath.Join(root, "CHANGELOG.md")), -1)
	if len(releases) == 0 {
		t.Fatal(`CHANGELOG.md has no "## [X.Y.Z] - AAAA-MM-JJ" section`)
	}
	if releases[0][1] != version {
		t.Errorf("VERSION is %s but the newest section of CHANGELOG.md is %s", version, releases[0][1])
	}
	date := releases[0][2]

	// AppStream: the first <release> is the current version, with the same date.
	appstream := appstreamRelease.FindStringSubmatch(read(t, filepath.Join(root, "metainfo", "io.github.gillesmagneville.imageprintlayout.metainfo.xml")))
	switch {
	case appstream == nil:
		t.Error("the AppStream file has no <release version=... date=...>")
	case appstream[1] != version:
		t.Errorf("VERSION is %s but the first AppStream <release> is %s", version, appstream[1])
	case appstream[2] != date:
		t.Errorf("the AppStream release is dated %s but CHANGELOG.md says %s", appstream[2], date)
	}

	// Manual pages, one per language of the program: .TH NAME SECTION "date"
	// "imageprintlayout X.Y.Z".
	for _, page := range manualPages {
		man := manPageHeader.FindStringSubmatch(read(t, filepath.Join(root, filepath.FromSlash(page))))
		switch {
		case man == nil:
			t.Errorf(`%s has no .TH header of the form: .TH IMAGEPRINTLAYOUT 1 "date" "imageprintlayout X.Y.Z" ...`, page)
		case man[2] != version:
			t.Errorf("VERSION is %s but %s says %s", version, page, man[2])
		case man[1] != date:
			t.Errorf("%s is dated %s but CHANGELOG.md says %s", page, man[1], date)
		}
	}
}

// manualPages are the manual pages of the program: the English one, which is
// the fallback, then one per other language.
var manualPages = []string{
	"docs/imageprintlayout.1",
	"docs/fr/imageprintlayout.1",
	"docs/es/imageprintlayout.1",
	"docs/de/imageprintlayout.1",
}
