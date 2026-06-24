// Package updater checks GitHub Releases for a newer client build and (on
// explicit user confirmation) downloads and launches the installer.
//
// It never auto-applies: CheckLatest only reports; Download/LaunchInstaller run
// only when the UI asks. The current version comes from internal/buildinfo
// (ldflags). No secrets are involved and nothing sensitive is logged.
package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Default GitHub repo to query. Overridable via env for forks/testing
// (VPNCLIENT_UPDATE_REPO=owner/name).
const defaultRepo = "Alexzxcv/vpn-client-windows"

// Release is the subset of the GitHub Releases API we use.
type Release struct {
	TagName    string  `json:"tag_name"`
	Name       string  `json:"name"`
	Body       string  `json:"body"`
	HTMLURL    string  `json:"html_url"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

// Asset is a downloadable release artifact.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Result is the outcome of a check, surfaced to the UI.
type Result struct {
	// CurrentVersion is this build's version (e.g. "v0.1.0" or "dev").
	CurrentVersion string `json:"current_version"`
	// LatestVersion is the newest published release tag (empty if none).
	LatestVersion string `json:"latest_version,omitempty"`
	// UpdateAvailable is true when LatestVersion is strictly newer than current.
	UpdateAvailable bool `json:"update_available"`
	// Notes is the release body (markdown), for display.
	Notes string `json:"notes,omitempty"`
	// ReleaseURL points at the GitHub release page.
	ReleaseURL string `json:"release_url,omitempty"`
	// AssetURL is the chosen installer/zip download URL (empty if none matched).
	AssetURL string `json:"asset_url,omitempty"`
	// AssetName is the chosen asset filename.
	AssetName string `json:"asset_name,omitempty"`
}

// Updater checks releases and downloads assets.
type Updater struct {
	current string
	repo    string
	http    *http.Client
}

// New builds an Updater. current is the running version (buildinfo.Version). hc
// may be nil for a default client with a sane timeout.
func New(current string, hc *http.Client) *Updater {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	repo := defaultRepo
	if env := strings.TrimSpace(os.Getenv("VPNCLIENT_UPDATE_REPO")); env != "" {
		repo = env
	}
	return &Updater{current: current, repo: repo, http: hc}
}

// CurrentVersion returns the running version string.
func (u *Updater) CurrentVersion() string { return u.current }

// CheckLatest queries the latest GitHub release and compares it to the current
// version. It returns a Result describing whether an update is available and the
// best asset to download (the Windows installer if present, else a zip).
func (u *Updater) CheckLatest(ctx context.Context) (Result, error) {
	res := Result{CurrentVersion: u.current}

	rel, err := u.latestRelease(ctx)
	if err != nil {
		return res, err
	}
	res.LatestVersion = rel.TagName
	res.Notes = rel.Body
	res.ReleaseURL = rel.HTMLURL

	if a, ok := pickAsset(rel.Assets); ok {
		res.AssetURL = a.BrowserDownloadURL
		res.AssetName = a.Name
	}

	res.UpdateAvailable = IsNewer(u.current, rel.TagName)
	return res, nil
}

// latestRelease fetches the newest non-draft release. The GitHub "latest"
// endpoint already excludes drafts and prereleases.
func (u *Updater) latestRelease(ctx context.Context) (Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", u.repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, fmt.Errorf("updater: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := u.http.Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("updater: fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return Release{}, fmt.Errorf("updater: no releases published")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return Release{}, fmt.Errorf("updater: github status %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Release{}, fmt.Errorf("updater: decode release: %w", err)
	}
	return rel, nil
}

// pickAsset chooses the best Windows asset: prefer the installer (.exe), then a
// .zip. Returns ok=false if neither is present.
func pickAsset(assets []Asset) (Asset, bool) {
	var zip *Asset
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if strings.HasSuffix(name, ".exe") {
			return assets[i], true
		}
		if strings.HasSuffix(name, ".zip") && zip == nil {
			zip = &assets[i]
		}
	}
	if zip != nil {
		return *zip, true
	}
	return Asset{}, false
}

// Download fetches assetURL into the OS temp dir and returns the local path. The
// filename is taken from assetName (sanitised). The caller is responsible for
// launching/removing it.
func (u *Updater) Download(ctx context.Context, assetURL, assetName string) (string, error) {
	if assetURL == "" {
		return "", fmt.Errorf("updater: empty asset url")
	}
	name := filepath.Base(filepath.Clean("/" + assetName))
	if name == "" || name == "." || name == "/" {
		name = "vpnclient-update"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return "", fmt.Errorf("updater: build download request: %w", err)
	}
	resp, err := u.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("updater: download asset: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("updater: download status %d", resp.StatusCode)
	}

	dir, err := os.MkdirTemp("", "vpnclient-update-*")
	if err != nil {
		return "", fmt.Errorf("updater: temp dir: %w", err)
	}
	dst := filepath.Join(dir, name)
	f, err := os.Create(dst)
	if err != nil {
		return "", fmt.Errorf("updater: create file: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return "", fmt.Errorf("updater: write file: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("updater: close file: %w", err)
	}
	return dst, nil
}

// IsNewer reports whether candidate is a strictly newer version than current.
// "dev" (or any unparseable current) is treated as the lowest version so a dev
// build always sees a published release as newer. Tags may carry a leading "v".
func IsNewer(current, candidate string) bool {
	cand, okC := parseSemver(candidate)
	if !okC {
		return false // can't trust an unparseable remote tag
	}
	cur, okCur := parseSemver(current)
	if !okCur {
		return true // dev/unknown local build: any real release is newer
	}
	return compare(cand, cur) > 0
}

type semver struct{ major, minor, patch int }

// parseSemver parses "v1.2.3" / "1.2.3" / "1.2" / "1" into a semver. Pre-release
// suffixes ("-rc1") are ignored for comparison purposes.
func parseSemver(s string) (semver, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return semver{}, false
	}
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return semver{}, false
	}
	var out semver
	dst := []*int{&out.major, &out.minor, &out.patch}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return semver{}, false
		}
		*dst[i] = n
	}
	return out, true
}

func compare(a, b semver) int {
	switch {
	case a.major != b.major:
		return sign(a.major - b.major)
	case a.minor != b.minor:
		return sign(a.minor - b.minor)
	case a.patch != b.patch:
		return sign(a.patch - b.patch)
	default:
		return 0
	}
}

func sign(n int) int {
	if n > 0 {
		return 1
	}
	if n < 0 {
		return -1
	}
	return 0
}
