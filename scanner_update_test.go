package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

type scannerTestTransport func(*http.Request) (*http.Response, error)

func (f scannerTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func scannerTestHash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func scannerTestInstall(t *testing.T, root, version string) scannerBundleManifest {
	t.Helper()
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for name, contents := range map[string]string{"ZZZ-Scanner.Next.exe": "old-exe", "Data/rules.json": "old-rules"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
		files[name] = scannerTestHash([]byte(contents))
	}
	bundle := scannerBundleManifest{Version: version, ReleaseTag: "scanner-" + version, Source: "https://github.com/ZztIsolation/ZZZ-Scanner.Next", Files: files}
	data, _ := json.Marshal(bundle)
	if err := os.WriteFile(filepath.Join(root, "SCANNER_BUNDLE.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	return bundle
}

func scannerTestClient(t *testing.T, version string, corrupt bool) (*http.Client, *int) {
	t.Helper()
	originalProbe := scannerProcessProbe
	scannerProcessProbe = func() (bool, error) { return false, nil }
	t.Cleanup(func() { scannerProcessProbe = originalProbe })
	var archive bytes.Buffer
	w := zip.NewWriter(&archive)
	files := []officialScannerFile{}
	var expanded int64
	for _, item := range [][2]string{{"ZZZ-Scanner.Next.exe", "new-exe"}, {"Data/rules.json", "new-rules"}} {
		entry, err := w.Create(item[0])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(item[1])); err != nil {
			t.Fatal(err)
		}
		files = append(files, officialScannerFile{Path: item[0], Size: int64(len(item[1])), SHA256: scannerTestHash([]byte(item[1]))})
		expanded += int64(len(item[1]))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	pkgBytes := append([]byte(nil), archive.Bytes()...)
	pkg := officialScannerPackage{ID: "win-x64-self-contained", Entry: "ZZZ-Scanner.Next.exe", Size: int64(len(pkgBytes)), ExpandedSize: expanded, Files: files, SHA256: scannerTestHash(pkgBytes)}
	base := "https://github.com/ZztIsolation/ZZZ-Scanner.Next/releases/download/scanner-" + version + "/"
	release := githubLatestRelease{TagName: "scanner-" + version, Assets: []githubReleaseAsset{
		{Name: "scanner-manifest-" + version + ".json", DownloadURL: base + "manifest.json"},
		{Name: "ZZZ-Scanner.Next-win-x64-self-contained.zip", DownloadURL: base + "scanner.zip", Size: pkg.Size},
	}}
	releaseBytes, _ := json.Marshal(release)
	manifestBytes, _ := json.Marshal(officialScannerManifest{SchemaVersion: 3, ScannerVersion: version, Packages: []officialScannerPackage{pkg}})
	if corrupt {
		pkgBytes[0] ^= 1
	}
	calls := new(int)
	client := &http.Client{Transport: scannerTestTransport(func(r *http.Request) (*http.Response, error) {
		*calls++
		var data []byte
		switch r.URL.String() {
		case scannerLatestReleaseAPI:
			data = releaseBytes
		case base + "manifest.json":
			data = manifestBytes
		case base + "scanner.zip":
			data = pkgBytes
		default:
			return nil, errors.New("unexpected network request")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data)), Header: make(http.Header)}, nil
	})}
	return client, calls
}

func TestScannerUpdateInstallsAndRetainsBackup(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "first-install", true: "upgrade"}[existing], func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "scanner")
			if existing {
				scannerTestInstall(t, root, "1.0.9")
				if err := os.WriteFile(filepath.Join(root, "preferences.json"), []byte("my-settings"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			client, calls := scannerTestClient(t, "1.0.49", false)
			state := ""
			err := updateLocalScanner(context.Background(), root, client, func(s, m, v string) { state = s })
			if err != nil || state != "updated" || *calls != 3 {
				t.Fatalf("update: state=%s calls=%d err=%v", state, *calls, err)
			}
			bundle, err := verifyScannerBundle(root)
			if err != nil || bundle.Version != "1.0.49" {
				t.Fatalf("new package: %+v %v", bundle, err)
			}
			if existing {
				data, err := os.ReadFile(filepath.Join(root, "preferences.json"))
				if err != nil || string(data) != "my-settings" {
					t.Fatal("lost local settings", err)
				}
				backups, err := os.ReadDir(scannerBackupRoot(root))
				if err != nil || len(backups) != 1 {
					t.Fatal("missing old backup", err)
				}
				old, err := verifyScannerBundle(filepath.Join(scannerBackupRoot(root), backups[0].Name()))
				if err != nil || old.Version != "1.0.9" {
					t.Fatal("old backup was changed", err)
				}
			}
		})
	}
}

func TestScannerUpdateSkipsCurrentAndNewerVersions(t *testing.T) {
	for _, version := range []string{"1.0.49", "1.0.100"} {
		root := filepath.Join(t.TempDir(), "scanner")
		scannerTestInstall(t, root, version)
		client, calls := scannerTestClient(t, "1.0.49", false)
		if err := updateLocalScanner(context.Background(), root, client, func(s, m, v string) {}); err != nil {
			t.Fatal(err)
		}
		if *calls != 1 {
			t.Fatal("downloaded an equal or older release")
		}
		bundle, err := verifyScannerBundle(root)
		if err != nil || bundle.Version != version {
			t.Fatal("existing version changed", err)
		}
	}
}

func TestScannerUpdateFailureLeavesExistingInstall(t *testing.T) {
	for _, failure := range []string{"corrupt", "offline", "cancelled"} {
		t.Run(failure, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "scanner")
			scannerTestInstall(t, root, "1.0.9")
			client, _ := scannerTestClient(t, "1.0.49", failure == "corrupt")
			ctx := context.Background()
			if failure == "offline" {
				client.Transport = scannerTestTransport(func(r *http.Request) (*http.Response, error) { return nil, errors.New("offline") })
			}
			if failure == "cancelled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if err := updateLocalScanner(ctx, root, client, func(s, m, v string) {}); err == nil {
				t.Fatal("bad update unexpectedly succeeded")
			}
			bundle, err := verifyScannerBundle(root)
			if err != nil || bundle.Version != "1.0.9" {
				t.Fatal("failed update changed old install", err)
			}
			staging, _ := filepath.Glob(filepath.Join(filepath.Dir(root), ".scanner-install-*"))
			if len(staging) != 0 {
				t.Fatal("temporary download was not cleaned up")
			}
		})
	}
}

func TestScannerUpdateDefersWhileScannerRunning(t *testing.T) {
	root := filepath.Join(t.TempDir(), "scanner")
	scannerTestInstall(t, root, "1.0.9")
	client, _ := scannerTestClient(t, "1.0.49", false)
	scannerRuntime.Lock()
	scannerRuntime.cmd = &exec.Cmd{}
	scannerRuntime.Unlock()
	t.Cleanup(func() { scannerRuntime.Lock(); scannerRuntime.cmd = nil; scannerRuntime.Unlock() })
	state := ""
	if err := updateLocalScanner(context.Background(), root, client, func(s, m, v string) { state = s }); err != nil {
		t.Fatal(err)
	}
	if state != "deferred" {
		t.Fatal("did not defer active scanner update")
	}
	bundle, err := verifyScannerBundle(root)
	if err != nil || bundle.Version != "1.0.9" {
		t.Fatal("replaced running scanner", err)
	}
}

func TestScannerUpdateDefersForExternalScanner(t *testing.T) {
	root := filepath.Join(t.TempDir(), "scanner")
	scannerTestInstall(t, root, "1.0.9")
	client, _ := scannerTestClient(t, "1.0.49", false)
	scannerProcessProbe = func() (bool, error) { return true, nil }
	state := ""
	if err := updateLocalScanner(context.Background(), root, client, func(s, m, v string) { state = s }); err != nil {
		t.Fatal(err)
	}
	bundle, err := verifyScannerBundle(root)
	if state != "deferred" || err != nil || bundle.Version != "1.0.9" {
		t.Fatal("external Scanner was replaced", err)
	}
}

func TestScannerUpdateRollbackAndCrashRecovery(t *testing.T) {
	root := filepath.Join(t.TempDir(), "scanner")
	old := scannerTestInstall(t, root, "1.0.9")
	if err := activateScannerUpdate(root, filepath.Join(filepath.Dir(root), "missing-content"), old); err == nil {
		t.Fatal("missing new directory activated")
	}
	if _, err := verifyScannerBundle(root); err != nil {
		t.Fatal("failed activation did not restore old install", err)
	}
	backup := filepath.Join(scannerBackupRoot(root), "interrupted")
	if err := os.Rename(root, backup); err != nil {
		t.Fatal(err)
	}
	if err := recoverScannerInstall(root); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyScannerBundle(root); err != nil {
		t.Fatal("crash recovery failed", err)
	}
}

func TestScannerUpdaterKeepsCustomInstall(t *testing.T) {
	root := filepath.Join(t.TempDir(), "scanner")
	old := scannerTestInstall(t, root, "1.0.9")
	old.Source = "local"
	data, _ := json.Marshal(old)
	if err := os.WriteFile(filepath.Join(root, "SCANNER_BUNDLE.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	client, calls := scannerTestClient(t, "1.0.49", false)
	state := ""
	if err := updateLocalScanner(context.Background(), root, client, func(s, m, v string) { state = s }); err != nil {
		t.Fatal(err)
	}
	if state != "skipped" || *calls != 0 {
		t.Fatal("custom installation was automatically replaced")
	}
}

func TestScannerUpdateStatusEndpoint(t *testing.T) {
	reportScannerUpdate("checking", "正在检查", "1.0.49")
	t.Cleanup(func() { reportScannerUpdate("", "", "") })
	mux, err := newAppMux(true)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/scanner/status", nil))
	var status scannerUpdateStatus
	decodeErr := json.Unmarshal(w.Body.Bytes(), &status)
	if w.Code != 200 || decodeErr != nil || status.State != "checking" || status.Version != "1.0.49" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/scanner/status", nil))
	if w.Code != 405 {
		t.Fatal("status accepted mutation")
	}
}

func TestScannerInstallLockExcludesOtherInstances(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows named mutex")
	}
	root := filepath.Join(t.TempDir(), "scanner")
	unlock, err := lockScannerInstall(root)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := lockScannerInstall(root); err == nil {
		second()
		unlock()
		t.Fatal("second installer acquired same lock")
	}
	unlock()
	unlock, err = lockScannerInstall(root)
	if err != nil {
		t.Fatal("lock did not release", err)
	}
	unlock()
}
