package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// EnvReleaseRepo overrides the GitHub repo the updater pulls releases from.
// Format: "owner/repo" (e.g. "tqrcisio/golang-boilerplate").
const EnvReleaseRepo = "BOILERPLATE_RELEASE_REPO"

// EnvManifestURL overrides the manifest URL entirely. Useful when hosting the
// manifest somewhere other than GitHub Releases.
const EnvManifestURL = "BOILERPLATE_MANIFEST_URL"

// DefaultReleaseRepo is the fallback repository used when EnvReleaseRepo is
// unset. Forks should override this at build time via:
//
//	-ldflags "-X github.com/tqrcisio/golang-boilerplate/internal/updater.DefaultReleaseRepo=owner/repo"
var DefaultReleaseRepo = "tqrcisio/golang-boilerplate"

// LatestManifest is the shape of manifest.json published as a release asset.
// CI uploads this file on every release; the boilerplate fetches it from
// https://github.com/{repo}/releases/latest/download/manifest.json which
// GitHub redirects to the latest release's asset.
type LatestManifest struct {
	Version            string    `json:"version"`
	ReleasedAt         time.Time `json:"released_at"`
	DownloadURL        string    `json:"download_url"`
	SHA256URL          string    `json:"sha256_url"`
	UpdaterDownloadURL string    `json:"updater_download_url,omitempty"`
	UpdaterSHA256URL   string    `json:"updater_sha256_url,omitempty"`
}

func releaseRepo() string {
	if r := os.Getenv(EnvReleaseRepo); r != "" {
		return r
	}
	return DefaultReleaseRepo
}

// manifestURL returns the URL that serves the latest manifest.json. Falls back
// to GitHub's "latest release asset" redirect when EnvManifestURL is unset.
func manifestURL() string {
	if u := os.Getenv(EnvManifestURL); u != "" {
		return u
	}
	return fmt.Sprintf("https://github.com/%s/releases/latest/download/manifest.json", releaseRepo())
}

// releaseAssetURL returns the canonical GitHub Releases URL for a specific
// version asset (e.g. updater.exe for v0.1.0).
func releaseAssetURL(version, asset string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", releaseRepo(), version, asset)
}

// FetchLatest reads manifest.json from the configured manifest URL.
func FetchLatest(ctx context.Context, client *http.Client) (LatestManifest, error) {
	url := manifestURL()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return LatestManifest{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return LatestManifest{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return LatestManifest{}, fmt.Errorf("manifest.json: status %d: %s", resp.StatusCode, string(body))
	}
	var m LatestManifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return LatestManifest{}, fmt.Errorf("manifest.json decode: %w", err)
	}
	if m.Version == "" || m.DownloadURL == "" || m.SHA256URL == "" {
		return LatestManifest{}, errors.New("manifest.json missing required fields")
	}
	return m, nil
}

// CompareVersions returns:
//
//	-1 if a  < b
//	 0 if a == b
//	+1 if a  > b
//
// Accepts versions formatted as "vX.Y.Z" (semver, leading "v" optional).
// "dev" is treated as lower than any released version.
func CompareVersions(a, b string) int {
	if a == b {
		return 0
	}
	if a == "dev" {
		return -1
	}
	if b == "dev" {
		return 1
	}
	pa := parseSemver(a)
	pb := parseSemver(b)
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

func parseSemver(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	var out [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		n, _ := strconv.Atoi(strings.SplitN(parts[i], "-", 2)[0])
		out[i] = n
	}
	return out
}
