package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var scannerInstallMu sync.Mutex
var scannerProcessProbe = scannerProcessRunning

type scannerUpdateStatus struct {
	State   string `json:"state"`
	Message string `json:"message"`
	Version string `json:"version,omitempty"`
}

var scannerUpdates struct {
	sync.RWMutex
	status scannerUpdateStatus
}

func reportScannerUpdate(state, message, version string) {
	scannerUpdates.Lock()
	scannerUpdates.status = scannerUpdateStatus{state, message, version}
	scannerUpdates.Unlock()
	log.Printf("Scanner: %s", message)
}

func scannerUpdateBusy() bool {
	scannerUpdates.RLock()
	defer scannerUpdates.RUnlock()
	return scannerUpdates.status.State == "checking" || scannerUpdates.status.State == "downloading"
}

func handleScannerStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	scannerUpdates.RLock()
	status := scannerUpdates.status
	scannerUpdates.RUnlock()
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status)
}

func startScannerUpdateCheck() {
	reportScannerUpdate("checking", "正在检查 Scanner 更新…", "")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		root, err := scannerInstallRoot()
		if err == nil {
			err = updateLocalScanner(ctx, root, scannerHTTPClient(), reportScannerUpdate)
		}
		if err != nil {
			reportScannerUpdate("error", "Scanner 自动更新未完成，不影响配装；已有本地版本可继续使用："+err.Error(), "")
		}
	}()
}

// Only numeric official versions are ordered; unknown custom builds must not be
// silently replaced. The managed install can coexist with a discovered sibling.
func compareScannerVersions(a, b string) (int, error) {
	parse := func(raw string) ([]uint64, error) {
		raw = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(raw), "scanner-"), "v")
		parts := strings.Split(raw, ".")
		if len(parts) < 2 || len(parts) > 4 {
			return nil, errors.New("Scanner 版本号无法识别")
		}
		out := make([]uint64, 4)
		for i, p := range parts {
			n, err := strconv.ParseUint(p, 10, 32)
			if err != nil {
				return nil, errors.New("Scanner 版本号无法识别")
			}
			out[i] = n
		}
		return out, nil
	}
	x, err := parse(a)
	if err != nil {
		return 0, err
	}
	y, err := parse(b)
	if err != nil {
		return 0, err
	}
	for i := range x {
		if x[i] < y[i] {
			return -1, nil
		}
		if x[i] > y[i] {
			return 1, nil
		}
	}
	return 0, nil
}

func scannerBackupRoot(root string) string { return root + "-backups" }

// Recover an interrupted directory switch before using the install. Backups are
// never deleted here; they may contain custom configuration or scan artifacts.
func recoverScannerInstall(root string) error {
	if _, err := os.Lstat(root); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	entries, err := os.ReadDir(scannerBackupRoot(root))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() > entries[j].Name() })
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		backup := filepath.Join(scannerBackupRoot(root), entry.Name())
		if _, err := verifyInstalledScannerBundle(backup); err != nil {
			continue
		}
		return os.Rename(backup, root)
	}
	return nil
}

func updateLocalScanner(ctx context.Context, root string, client *http.Client, report func(string, string, string)) error {
	if !scannerInstallMu.TryLock() {
		return errors.New("其他 Scanner 安装正在进行")
	}
	defer scannerInstallMu.Unlock()
	unlock, err := lockScannerInstall(root)
	if err != nil {
		return err
	}
	defer unlock()
	if err := recoverScannerInstall(root); err != nil {
		return err
	}
	var installed scannerBundleManifest
	if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 {
		info, err := os.Lstat(root)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("Scanner 安装目录不能是链接")
		}
		installed, err = verifyInstalledScannerBundle(root)
		if err != nil {
			return fmt.Errorf("保留现有目录，请检查本地 Scanner：%w", err)
		}
		if installed.Source != "https://github.com/ZztIsolation/ZZZ-Scanner.Next" || installed.ReleaseTag == "" {
			report("skipped", "检测到独立安装的 Scanner，保留本地版本", installed.Version)
			return nil
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	var release githubLatestRelease
	checkCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	err = fetchScannerJSON(checkCtx, client, scannerLatestReleaseAPI, 4*1024*1024, &release)
	cancel()
	if err != nil {
		return fmt.Errorf("无法检查官方新版：%w", err)
	}
	if installed.Version != "" {
		cmp, err := compareScannerVersions(release.TagName, installed.Version)
		if err != nil {
			return err
		}
		if cmp <= 0 {
			report("current", "Scanner "+installed.Version+" 已是最新版本", installed.Version)
			return nil
		}
	}
	manifestAsset, packageAsset, err := selectLatestScannerAssets(release)
	if err != nil {
		return err
	}
	var official officialScannerManifest
	manifestCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	err = fetchScannerJSON(manifestCtx, client, manifestAsset.DownloadURL, 4*1024*1024, &official)
	cancel()
	if err != nil {
		return fmt.Errorf("无法读取官方清单：%w", err)
	}
	if cmp, err := compareScannerVersions(release.TagName, official.ScannerVersion); err != nil || cmp != 0 {
		return errors.New("官方 Release 与 manifest 版本不一致")
	}
	pkg, err := selectSelfContainedScannerPackage(official)
	if err != nil {
		return err
	}
	report("downloading", "正在下载并校验 Scanner "+official.ScannerVersion+"…", installed.Version)
	if err := os.MkdirAll(filepath.Dir(root), 0755); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(filepath.Dir(root), ".scanner-install-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	zipPath := filepath.Join(staging, "scanner.zip")
	if err := downloadScannerPackage(ctx, client, packageAsset, pkg, zipPath); err != nil {
		return err
	}
	content := filepath.Join(staging, "content")
	if err := os.Mkdir(content, 0755); err != nil {
		return err
	}
	if err := extractScannerPackage(zipPath, content, pkg); err != nil {
		return err
	}
	files := map[string]string{}
	for _, file := range pkg.Files {
		files[filepath.ToSlash(file.Path)] = strings.ToLower(file.SHA256)
	}
	bundle := scannerBundleManifest{Version: official.ScannerVersion, ReleaseTag: release.TagName, Source: "https://github.com/ZztIsolation/ZZZ-Scanner.Next", Package: packageAsset.Name, Files: files}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(content, "SCANNER_BUNDLE.json"), data, 0644); err != nil {
		return err
	}
	if _, err := verifyInstalledScannerBundle(content); err != nil {
		return fmt.Errorf("新包缺少可启动的主程序，保留旧版：%w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	scannerRuntime.Lock()
	defer scannerRuntime.Unlock()
	externalRunning, err := scannerProcessProbe()
	if err != nil {
		return fmt.Errorf("无法确认 Scanner 是否正在运行，保留旧版：%w", err)
	}
	if scannerRuntime.cmd != nil || externalRunning {
		report("deferred", "Scanner 正在运行，将在下次启动配装器时更新", installed.Version)
		return nil
	}
	if err := activateScannerUpdate(root, content, installed); err != nil {
		return err
	}
	report("updated", "Scanner 已更新至 "+official.ScannerVersion, official.ScannerVersion)
	return nil
}

// Keep extra local settings/logs, but never overwrite newly verified package
// files. Modified packaged files remain in the complete old-directory backup.
func preserveScannerExtras(root, content string, old scannerBundleManifest) error {
	managed := map[string]bool{"scanner_bundle.json": true}
	for name := range old.Files {
		managed[strings.ToLower(filepath.ToSlash(name))] = true
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("本地 Scanner 含链接，保留现有目录：%s", path)
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if managed[strings.ToLower(filepath.ToSlash(rel))] {
			return nil
		}
		target, err := scannerBundleFile(content, rel)
		if err != nil {
			return err
		}
		if _, err := os.Stat(target); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func activateScannerUpdate(root, content string, old scannerBundleManifest) error {
	backup := ""
	if info, err := os.Lstat(root); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("Scanner 安装目录异常，未替换")
		}
		if err := preserveScannerExtras(root, content, old); err != nil {
			return err
		}
		if err := os.MkdirAll(scannerBackupRoot(root), 0755); err != nil {
			return err
		}
		backup, err = os.MkdirTemp(scannerBackupRoot(root), time.Now().UTC().Format("20060102T150405.000000000")+"-*")
		if err != nil {
			return err
		}
		if err := os.Remove(backup); err != nil {
			return err
		}
		if err := os.Rename(root, backup); err != nil {
			return fmt.Errorf("Scanner 可能正在使用，保留旧版，下次启动重试：%w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(content, root); err != nil {
		if backup != "" {
			if rollbackErr := os.Rename(backup, root); rollbackErr != nil {
				return fmt.Errorf("更新失败：%v；旧版保存在 %s，恢复失败：%w", err, backup, rollbackErr)
			}
		}
		return fmt.Errorf("Scanner 更新未启用，已保留旧版：%w", err)
	}
	return nil
}
