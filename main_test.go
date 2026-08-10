package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedInventoryAssets(t *testing.T) {
	for _, path := range []string{
		"web/index.html",
		"web/assets/asset-map.js",
		"web/assets/drive-disc-interop.js",
		"web/assets/drive-discs/drive-disc-01.png",
		"web/assets/drive-discs/drive-disc-29.png",
		"web/assets/drive-discs/drive-disc-30.png",
		"web/assets/agents/agent-01.png",
		"web/assets/agents/agent-57.png",
		"web/assets/agents/Q_AVATAR_SOURCE_MANIFEST.json",
		"web/assets/ASSET_SOURCES.md",
	} {
		data, err := webFiles.ReadFile(path)
		if err != nil {
			t.Fatalf("embedded asset %q: %v", path, err)
		}
		if len(data) == 0 {
			t.Fatalf("embedded asset %q is empty", path)
		}
	}
}

func TestStrictTargetPriorityIsLexicographic(t *testing.T) {
	results := []OptimizeResult{
		{Score: 1, DamageIndex: 100, StrictTargetGaps: []float64{0, 12}},
		{Score: 999999, DamageIndex: 999999, StrictTargetGaps: []float64{1, 0}},
	}
	sortResults(results, "STRICT_TARGETS")
	if results[0].StrictTargetGaps[0] != 0 {
		t.Fatalf("lower-priority score overrode priority 1: %#v", results)
	}

	req := OptimizeRequest{
		BaseATK:           1000,
		TargetCritRate:    80,
		TargetFinalAttack: 4000,
		TargetPriorities: map[string]int{
			"ATK":       1,
			"CRIT_RATE": 2,
		},
	}
	_, _, gaps := strictTargetPenalty(map[string]float64{}, 80, 50, 3900, 0, 0, 0, req)
	if len(gaps) != 6 || gaps[0] <= 0 || gaps[1] != 0 {
		t.Fatalf("priority gaps = %#v; want attack gap at level 1 and exact crit at level 2", gaps)
	}
}

func TestCharacterTargetsStoredSeparately(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "inventory.json")
	if err := saveState(statePath, defaultState()); err != nil {
		t.Fatal(err)
	}
	targets := CharacterTargetsFile{Plans: []json.RawMessage{json.RawMessage(`{"id":"plan-1","characterName":"蕾米埃尔"}`)}}
	if err := saveCharacterTargets(statePath, targets); err != nil {
		t.Fatal(err)
	}
	stateJSON, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stateJSON, []byte("multiCharacterPlans")) || bytes.Contains(stateJSON, []byte("蕾米埃尔")) {
		t.Fatalf("inventory state contains character targets: %s", stateJSON)
	}
	loaded, err := loadCharacterTargets(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Plans) != 1 || !bytes.Contains(loaded.Plans[0], []byte("蕾米埃尔")) {
		t.Fatalf("separate character targets = %#v", loaded.Plans)
	}
	if got := characterTargetsPath(statePath); got != filepath.Join(filepath.Dir(statePath), "inventory-character-targets.json") {
		t.Fatalf("character targets path = %s", got)
	}
}

func TestPortableJSONPathsAndLegacyMigration(t *testing.T) {
	portable := t.TempDir()
	legacy := t.TempDir()
	t.Setenv("ZZZ_APP_DATA_ROOT", portable)
	t.Setenv("ZZZ_LEGACY_CONFIG_ROOT", legacy)

	statePath, err := defaultStoragePath()
	if err != nil {
		t.Fatal(err)
	}
	if statePath != filepath.Join(portable, "state.json") {
		t.Fatalf("portable state path = %s", statePath)
	}
	configPath, err := storageConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if configPath != filepath.Join(portable, "storage-config.json") {
		t.Fatalf("portable config path = %s", configPath)
	}
	outputPath, err := scannerOutputRoot()
	if err != nil {
		t.Fatal(err)
	}
	if outputPath != filepath.Join(portable, "scanner-outputs") {
		t.Fatalf("portable Scanner output path = %s", outputPath)
	}

	legacyState := filepath.Join(legacy, "state.json")
	if err := saveState(legacyState, AppState{Version: appVersion, Discs: []Disc{{ID: "legacy-disc"}}}); err != nil {
		t.Fatal(err)
	}
	legacyTargets := CharacterTargetsFile{Plans: []json.RawMessage{json.RawMessage(`{"id":"legacy-plan"}`)}}
	if err := saveCharacterTargets(legacyState, legacyTargets); err != nil {
		t.Fatal(err)
	}
	if err := migrateLegacyStorageFiles(); err != nil {
		t.Fatal(err)
	}
	migrated, err := loadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrated.Discs) != 1 || migrated.Discs[0].ID != "legacy-disc" {
		t.Fatalf("legacy inventory was not migrated: %#v", migrated.Discs)
	}
	targets, err := loadCharacterTargets(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets.Plans) != 1 || !bytes.Contains(targets.Plans[0], []byte("legacy-plan")) {
		t.Fatalf("legacy character targets were not migrated: %#v", targets.Plans)
	}
	if _, err := os.Stat(legacyState); err != nil {
		t.Fatalf("legacy source should remain recoverable: %v", err)
	}
}

func TestInteropFieldsSurviveStateRoundTrip(t *testing.T) {
	raw := []byte(`{
		"id":"scanner-abc","setName":"流光咏叹","setId":"astral_voice","slot":1,
		"rarity":"S","level":15,"maxLevel":15,"locked":false,"discarded":false,
		"equippedBy":"","note":"","createdAt":"2026-07-28T00:00:00Z","updatedAt":"2026-07-28T00:00:00Z",
		"source":{"type":"zzz-scanner"},"reservedForAgentId":"agent-a","futureField":{"keep":true},
		"mainStat":{"type":"HP_FLAT","stat":"hpFlat","mode":"flat","value":2200,"label":"生命值","rawValue":2200},
		"subStats":[{"type":"CRIT_RATE","stat":"critRate","mode":"pct","value":4.8,"label":"暴击率","rawValue":"4.8%"}]
	}`)
	var disc Disc
	if err := json.Unmarshal(raw, &disc); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(disc)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range [][]byte{
		[]byte(`"setId":"astral_voice"`), []byte(`"maxLevel":15`),
		[]byte(`"reservedForAgentId":"agent-a"`), []byte(`"futureField":{"keep":true}`),
		[]byte(`"stat":"hpFlat"`), []byte(`"rawValue":"4.8%"`),
	} {
		if !bytes.Contains(encoded, marker) {
			t.Fatalf("round trip lost interoperability field %s: %s", marker, encoded)
		}
	}
}

func TestBundledScannerIntegrity(t *testing.T) {
	root, err := findScannerBundle()
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := verifyScannerBundle(root)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "1.0.45" || manifest.ReleaseTag != "scanner-1.0.45" {
		t.Fatalf("unexpected bundled scanner metadata: %#v", manifest)
	}
	for _, required := range []string{
		"ZZZ-Scanner.Next.exe",
		"Resources/models/PP-OCRv5_mobile_rec_infer.onnx",
		"Data/drive_discs.json",
		"onnxruntime.dll",
	} {
		if manifest.Files[required] == "" {
			t.Fatalf("scanner manifest is missing %s", required)
		}
	}
}

func TestScannerBundlePathCannotEscapeRoot(t *testing.T) {
	if _, err := scannerBundleFile(t.TempDir(), "../outside.exe"); err == nil {
		t.Fatal("scanner bundle path traversal should be rejected")
	}
}

func TestLatestScannerAssetSelectionAndManifestValidation(t *testing.T) {
	release := githubLatestRelease{Assets: []githubReleaseAsset{
		{Name: "scanner-manifest-1.2.3.json", DownloadURL: "https://github.com/ZztIsolation/ZZZ-Scanner.Next/releases/download/scanner-1.2.3/scanner-manifest-1.2.3.json"},
		{Name: "ZZZ-Scanner.Next-win-x64-self-contained.zip", Size: 123, DownloadURL: "https://github.com/ZztIsolation/ZZZ-Scanner.Next/releases/download/scanner-1.2.3/ZZZ-Scanner.Next-win-x64-self-contained.zip"},
	}}
	manifestAsset, packageAsset, err := selectLatestScannerAssets(release)
	if err != nil || manifestAsset.Name == "" || packageAsset.Size != 123 {
		t.Fatalf("official Scanner assets not selected: manifest=%#v package=%#v err=%v", manifestAsset, packageAsset, err)
	}
	pkg, err := selectSelfContainedScannerPackage(officialScannerManifest{SchemaVersion: 3, ScannerVersion: "1.2.3", Packages: []officialScannerPackage{{
		ID: "win-x64-self-contained", SHA256: strings.Repeat("a", 64), Size: 123, ExpandedSize: 456, Entry: "ZZZ-Scanner.Next.exe", Files: []officialScannerFile{{Path: "ZZZ-Scanner.Next.exe", Size: 10, SHA256: strings.Repeat("b", 64)}},
	}}})
	if err != nil || pkg.ID != "win-x64-self-contained" {
		t.Fatalf("self-contained package not accepted: %#v err=%v", pkg, err)
	}
	release.Assets[1].DownloadURL = "https://example.com/untrusted.zip"
	if _, _, err := selectLatestScannerAssets(release); err == nil {
		t.Fatal("non-official Scanner asset URL should be rejected")
	}
}

func TestScannerZipTraversalIsRejected(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "bad.zip")
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create("../outside.exe")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte("bad"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	pkg := officialScannerPackage{ExpandedSize: 3, Files: []officialScannerFile{{Path: "../outside.exe", Size: 3, SHA256: strings.Repeat("0", 64)}}}
	if err := extractScannerPackage(zipPath, filepath.Join(t.TempDir(), "out"), pkg); err == nil {
		t.Fatal("Scanner ZIP path traversal should be rejected")
	}
}

func TestStandaloneScannerCanBeDiscoveredWithoutBundleManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ZZZ-Scanner.Next.exe"), []byte("local-scanner"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest, err := verifyOrDescribeScannerBundle(root)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != "本地独立版" || manifest.Files["ZZZ-Scanner.Next.exe"] == "" {
		t.Fatalf("standalone Scanner metadata is incomplete: %#v", manifest)
	}
}

func TestDualReleaseRendering(t *testing.T) {
	index, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	versionA := renderIndexPage(index, "A")
	versionB := renderIndexPage(index, "B")
	if !bytes.Contains(versionA, []byte("v2.0.0")) || !bytes.Contains(versionB, []byte("v2.0.0")) {
		t.Fatal("v2.0.0 release label was not rendered")
	}
	if bytes.Contains(versionA, []byte(`id="startScannerBtn"`)) || bytes.Contains(versionA, []byte(`>打开驱动盘扫描器</button>`)) {
		t.Fatal("V1.05A must not render the scanner button")
	}
	if !bytes.Contains(versionA, []byte("const SCANNER_AVAILABLE=false")) {
		t.Fatal("V1.05A must disable scanner JavaScript")
	}
	if !scannerIncludedForEdition("B") || scannerIncludedForEdition("A") {
		t.Fatal("scanner edition selection is incorrect")
	}
	for _, marker := range []string{
		`id="startScannerBtn"`,
		`class="scannerLaunchButton"`,
		`>打开驱动盘扫描器</button>`,
		`点击“检测窗口”→“开始扫描”`,
		`/api/scanner/start`,
		`startBundledScanner`,
		`background: #16a34a`,
	} {
		if !bytes.Contains(versionB, []byte(marker)) {
			t.Fatalf("scanner integration marker missing: %s", marker)
		}
	}
	inputSection := bytes.Index(versionB, []byte(`<section class="card" id="input">`))
	button := bytes.Index(versionB, []byte(`id="startScannerBtn"`))
	tip := bytes.Index(versionB, []byte(`<div class="tipText">`))
	if inputSection < 0 || button <= inputSection || tip <= button {
		t.Fatalf("scanner button should be the first control in the drive-disc input section: section=%d button=%d tip=%d", inputSection, button, tip)
	}
}

func TestSingleCharacterResultsAreEmbeddedInOptimizer(t *testing.T) {
	index, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		`<h2>3. 角色配装与结果</h2>`,
		`id="currentCharacterResults"`,
		`id="currentResultCharacterName"`,
		`最多显示前 20 套`,
		`location.hash='#currentCharacterResults'`,
	} {
		if !bytes.Contains(index, []byte(marker)) {
			t.Fatalf("embedded single-character result marker missing: %s", marker)
		}
	}
	for _, obsolete := range []string{`<section class="card" id="results">`, `<h2>4. 配装结果</h2>`, `href="#results"`} {
		if bytes.Contains(index, []byte(obsolete)) {
			t.Fatalf("obsolete separate result section remains: %s", obsolete)
		}
	}
}

func TestReleaseOrderedDropdownsAndReadableDataDetails(t *testing.T) {
	index, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		"const DRIVE_SET_RELEASE_ORDER = Object.fromEntries(BUILTIN_SET_NAMES.map((name,index)=>[name,index+1]));",
		"function sortSetNames(names){return Array.from(names).sort((a,b)=>releaseOrderOfDriveSet(b)-releaseOrderOfDriveSet(a)||a.localeCompare(b,'zh-CN'));}",
		"function releaseOrderedRoles()",
		"fillRoleControl({selectNewest:true});",
		"releaseOrderOfName(b.character)-releaseOrderOfName(a.character)",
		`id="characterInfo" class="tipText dataDetail"`,
		`id="wEngineInfo" class="tipText dataDetail"`,
		".tipText.dataDetail strong { color: #fff; font-size: 16px; }",
		".tipText.dataDetail .fine { color: #d9e0ef; font-size: 13px; }",
	} {
		if !bytes.Contains(index, []byte(marker)) {
			t.Fatalf("release-order/readability marker missing: %s", marker)
		}
	}
	if bytes.Contains(index, []byte("$('#roleSystem').value='ATTACK';")) {
		t.Fatal("optimizer reset must not force the old Attack/Ye Shunguang default")
	}
}

func TestV105InterfaceEmphasisAndRemielleAvatar(t *testing.T) {
	index, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		"h2 { margin: 0 0 14px; font-size: 24px; line-height: 1.35; font-weight: 850; letter-spacing: .01em; }",
		".tipText.prominentTip",
		".tipText.dataDetail .dataNumber",
		"function highlightDataNumbers(value)",
		`<div id="driveSetInfo" class="tipText hidden"></div>`,
		"if(!entries.length){el.textContent=''; el.classList.add('hidden'); return;}",
	} {
		if !bytes.Contains(index, []byte(marker)) {
			t.Fatalf("V1.05 interface marker missing: %s", marker)
		}
	}
	for _, removed := range []string{
		"本地保存库存，支持强攻/命破/异常/击破/防护/辅助多职业配装。",
		"选择套装后会显示 3.0 新驱动盘的 2 件套、4 件套与本工具计算口径。",
		"选择「呼啸沙龙」或「拂晓行纪」后，会在此显示 3.0 套装效果与本工具计算口径。",
	} {
		if bytes.Contains(index, []byte(removed)) {
			t.Fatalf("removed V1.05 interface copy is still present: %s", removed)
		}
	}
	assetMap, err := webFiles.ReadFile("web/assets/asset-map.js")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(assetMap, []byte(`'蕾米埃尔': '/assets/agents/agent-58.png'`)) {
		t.Fatal("Remielle chibi avatar mapping is missing")
	}
	if _, err := webFiles.ReadFile("web/assets/agents/agent-58.png"); err != nil {
		t.Fatalf("Remielle chibi avatar asset: %v", err)
	}
}

func anomalyPanelDiscs() []Disc {
	return []Disc{
		testDisc("谶羽之誓", 1, sv("HP_FLAT", 2200)),
		testDisc("谶羽之誓", 2, sv("ATK_FLAT", 316)),
		testDisc("谶羽之誓", 3, sv("DEF_FLAT", 184)),
		testDisc("谶羽之誓", 4, sv("ANOMALY_PROFICIENCY", 92)),
		testDisc("自由蓝调", 5, sv("ATK_PERCENT", 30)),
		testDisc("自由蓝调", 6, sv("ANOMALY_MASTERY", 30)),
	}
}

func anomalyPanelRequest() OptimizeRequest {
	return OptimizeRequest{
		RoleSystem:       "ANOMALY",
		Mode:             "ANOMALY_AP",
		CharacterName:    "蕾米埃尔·丹",
		CharacterElement: "LUMIFLUX",
		SetPattern:       "4+2",
		Required4Set:     "谶羽之誓",
		Required2Set:     "自由蓝调",
		BaseATK:          823,
		BaseCritRate:     5,
		BaseCritDmg:      50,
		ExtraStats: map[string]float64{
			"BASE_ATK":            743,
			"ATK_PERCENT":         36,
			"ANOMALY_PROFICIENCY": 224,
		},
		CombatExtraStats: map[string]float64{
			"ANOMALY_PROFICIENCY": 96,
			"ATK_PERCENT":         20,
		},
		WantedWeights: roleEffectiveWeights("ANOMALY", "ANOMALY_AP", nil),
	}
}

func TestOptimizeAcceptsMultipleTwoPieceCandidates(t *testing.T) {
	discs := anomalyPanelDiscs()
	chaos5 := testDisc("混沌爵士", 5, sv("ATK_PERCENT", 30))
	chaos5.SubStats = []StatValue{sv("ANOMALY_PROFICIENCY", 9)}
	chaos6 := testDisc("混沌爵士", 6, sv("ANOMALY_MASTERY", 30))
	chaos6.SubStats = []StatValue{sv("ANOMALY_PROFICIENCY", 9)}
	discs = append(discs, chaos5, chaos6)

	req := anomalyPanelRequest()
	req.Discs = discs
	req.Required2Sets = []string{"自由蓝调", "混沌爵士", "自由蓝调"}
	req.TopN = 10
	resp := optimize(context.Background(), req)
	if len(resp.Results) == 0 {
		t.Fatalf("multi 2-piece search returned no results: %s", resp.Message)
	}
	counts := map[string]int{}
	for _, disc := range resp.Results[0].Discs {
		counts[canonicalSetName(disc.SetName)]++
	}
	if counts["谶羽之誓"] != 4 || counts["混沌爵士"] != 2 {
		t.Fatalf("best merged result sets = %#v; want 谶羽之誓 4 + 混沌爵士 2", counts)
	}
}

func TestAnomalyResultUsesAgentPanelValues(t *testing.T) {
	req := anomalyPanelRequest()
	res, ok := evaluateBuild(anomalyPanelDiscs(), req, nil)
	if !ok {
		t.Fatal("anomaly panel fixture did not evaluate")
	}
	if res.Stats["ANOMALY_PROFICIENCY"] != 376 {
		t.Fatalf("panel AP = %.0f; want 376", res.Stats["ANOMALY_PROFICIENCY"])
	}
	if res.CombatStats["ANOMALY_PROFICIENCY"] != 522 {
		t.Fatalf("combat AP = %.0f; want 522", res.CombatStats["ANOMALY_PROFICIENCY"])
	}
	if res.FinalAttack != 2915 || res.CombatFinalAttack != 3228 {
		t.Fatalf("panel/combat attack = %.0f/%.0f; want 2915/3228", res.FinalAttack, res.CombatFinalAttack)
	}
	if !strings.Contains(res.Reason, "面板异常精通 376.0") || !strings.Contains(res.Reason, "面板攻击 2915") {
		t.Fatalf("anomaly reason does not use panel values: %s", res.Reason)
	}
	if strings.Contains(res.Reason, "522.0") || strings.Contains(res.Reason, "攻击力 86.0%") {
		t.Fatalf("anomaly reason leaked combat/component values: %s", res.Reason)
	}
}

func TestAnomalyTargetCannotBeSatisfiedByCombatBonus(t *testing.T) {
	req := anomalyPanelRequest()
	req.TargetAnomalyProficiency = 450
	if _, ok := evaluateBuild(anomalyPanelDiscs(), req, nil); ok {
		t.Fatal("combat AP must not satisfy a panel-facing AP target")
	}
}

func TestAnomalyAttackModeSortsByFinalPanelAttack(t *testing.T) {
	results := []OptimizeResult{
		{FinalAttack: 2800, Stats: map[string]float64{"ATK_PERCENT": 90, "ANOMALY_PROFICIENCY": 400}},
		{FinalAttack: 3000, Stats: map[string]float64{"ATK_PERCENT": 80, "ANOMALY_PROFICIENCY": 400}},
	}
	sortResults(results, "ANOMALY_ATK")
	if results[0].FinalAttack != 3000 {
		t.Fatalf("ANOMALY_ATK sorted by a component instead of final panel attack: %#v", results)
	}
}

func TestAnomalyResultMarkupUsesPanelStats(t *testing.T) {
	index, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"const panelStats=res.stats||{}", "面板异常精通", "面板攻击"} {
		if !bytes.Contains(index, []byte(marker)) {
			t.Fatalf("anomaly result panel marker missing: %s", marker)
		}
	}
	if bytes.Contains(index, []byte("const st=res.combatStats||res.stats||{}")) {
		t.Fatal("anomaly result panel still reads combatStats")
	}
}

func remielleScreenshotDiscs() []Disc {
	return []Disc{
		testDisc("谶羽之誓", 1, sv("HP_FLAT", 2200), sv("ATK_PERCENT", 3), sv("ANOMALY_PROFICIENCY", 36), sv("ATK_FLAT", 38), sv("PEN_FLAT", 9)),
		testDisc("静听嘉音", 2, sv("ATK_FLAT", 316), sv("HP_PERCENT", 6), sv("ANOMALY_PROFICIENCY", 27), sv("DEF_PERCENT", 9.6), sv("ATK_PERCENT", 6)),
		testDisc("谶羽之誓", 3, sv("DEF_FLAT", 184), sv("PEN_FLAT", 9), sv("CRIT_DMG", 4.8), sv("ANOMALY_PROFICIENCY", 27), sv("ATK_PERCENT", 9)),
		testDisc("静听嘉音", 4, sv("ANOMALY_PROFICIENCY", 92), sv("ATK_PERCENT", 9), sv("CRIT_DMG", 4.8), sv("PEN_FLAT", 18), sv("DEF_FLAT", 30)),
		testDisc("谶羽之誓", 5, sv("ATK_PERCENT", 30), sv("ATK_FLAT", 38), sv("PEN_FLAT", 18), sv("ANOMALY_PROFICIENCY", 27), sv("CRIT_DMG", 4.8)),
		testDisc("谶羽之誓", 6, sv("ATK_PERCENT", 30), sv("ANOMALY_PROFICIENCY", 27), sv("ATK_FLAT", 38), sv("CRIT_RATE", 4.8), sv("DEF_FLAT", 30)),
	}
}

func remielleScreenshotRequest() OptimizeRequest {
	return OptimizeRequest{
		Discs:                    remielleScreenshotDiscs(),
		RoleSystem:               "ANOMALY",
		Mode:                     "ANOMALY_AP",
		CharacterName:            "蕾米埃尔·丹",
		CharacterElement:         "LUMIFLUX",
		SetPattern:               "4+2",
		Required4Set:             "谶羽之誓",
		Required2Set:             "静听嘉音",
		BaseHP:                   7482,
		BaseATK:                  748,
		BaseDEF:                  600,
		BaseCritRate:             5,
		BaseCritDmg:              50,
		BaseAnomalyMastery:       115,
		BaseEnergyRegen:          1.2,
		TargetFinalAttack:        4078,
		TargetAnomalyProficiency: 436,
		TopN:                     20,
		TopKPerSlot:              80,
		MaxCombinations:          2000000,
		ExtraStats: map[string]float64{
			"BASE_ATK":            75 + 743,
			"ATK_PERCENT":         36,
			"ANOMALY_PROFICIENCY": 116 + 54,
		},
		CombatExtraStats: map[string]float64{
			"ANOMALY_PROFICIENCY": 96,
		},
		WantedWeights: roleEffectiveWeights("ANOMALY", "ANOMALY_AP", nil),
	}
}

func TestRemielleScreenshotPanelCalibration(t *testing.T) {
	req := remielleScreenshotRequest()
	res, ok := evaluateBuild(req.Discs, req, nil)
	if !ok {
		t.Fatal("蕾米埃尔实机六盘未通过 evaluateBuild")
	}
	if res.FinalHP != 10131 || res.FinalAttack != 4078 || res.FinalDefense != 901 {
		t.Fatalf("蕾米埃尔面板生命/攻击/防御 = %.0f/%.0f/%.0f; want 10131/4078/901", res.FinalHP, res.FinalAttack, res.FinalDefense)
	}
	if !almostEqual(res.PanelCritRate, 9.8) || !almostEqual(res.PanelCritDmg, 64.4) {
		t.Fatalf("蕾米埃尔面板暴击/暴伤 = %.1f/%.1f; want 9.8/64.4", res.PanelCritRate, res.PanelCritDmg)
	}
	if res.Stats["ANOMALY_PROFICIENCY"] != 436 || res.GameEffectiveWords != 31 {
		t.Fatalf("蕾米埃尔异常精通/有效词条 = %.0f/%.0f; want 436/31", res.Stats["ANOMALY_PROFICIENCY"], res.GameEffectiveWords)
	}
	if res.Stats["ATK_PERCENT"] != 133 || res.Stats["ATK_FLAT"] != 430 {
		t.Fatalf("蕾米埃尔汇总攻击力%%/固定攻击 = %.0f/%.0f; want 133/430", res.Stats["ATK_PERCENT"], res.Stats["ATK_FLAT"])
	}
}

func TestRemielleScreenshotBuildIsOptimizable(t *testing.T) {
	resp := optimize(context.Background(), remielleScreenshotRequest())
	if len(resp.Results) != 1 {
		t.Fatalf("蕾米埃尔实机六盘应返回 1 个方案，got %d; message=%s counts=%#v", len(resp.Results), resp.Message, resp.CandidateCounts)
	}
	if resp.Results[0].FinalAttack != 4078 || resp.Results[0].Stats["ANOMALY_PROFICIENCY"] != 436 {
		t.Fatalf("优化结果面板 = attack %.0f / AP %.0f; want 4078 / 436", resp.Results[0].FinalAttack, resp.Results[0].Stats["ANOMALY_PROFICIENCY"])
	}
}

func remielleCoreFScreenshotDiscs() []Disc {
	return []Disc{
		testDisc("谶羽之誓", 1, sv("HP_FLAT", 2200), sv("ATK_PERCENT", 3), sv("ANOMALY_PROFICIENCY", 36), sv("ATK_FLAT", 38), sv("PEN_FLAT", 9)),
		testDisc("谶羽之誓", 2, sv("ATK_FLAT", 316), sv("CRIT_RATE", 2.4), sv("ATK_PERCENT", 9), sv("ANOMALY_PROFICIENCY", 18), sv("PEN_FLAT", 18)),
		testDisc("谶羽之誓", 3, sv("DEF_FLAT", 184), sv("PEN_FLAT", 9), sv("CRIT_DMG", 4.8), sv("ANOMALY_PROFICIENCY", 27), sv("ATK_PERCENT", 9)),
		testDisc("自由蓝调", 4, sv("ANOMALY_PROFICIENCY", 92), sv("PEN_FLAT", 18), sv("CRIT_RATE", 7.2), sv("ATK_PERCENT", 6), sv("ATK_FLAT", 19)),
		testDisc("自由蓝调", 5, sv("ATK_PERCENT", 30), sv("CRIT_DMG", 4.8), sv("PEN_FLAT", 9), sv("ANOMALY_PROFICIENCY", 27), sv("DEF_FLAT", 45)),
		testDisc("谶羽之誓", 6, sv("ATK_PERCENT", 30), sv("ANOMALY_PROFICIENCY", 27), sv("ATK_FLAT", 38), sv("CRIT_RATE", 4.8), sv("DEF_FLAT", 30)),
	}
}

func remielleCoreFScreenshotRequest() OptimizeRequest {
	return OptimizeRequest{
		Discs:                    remielleCoreFScreenshotDiscs(),
		RoleSystem:               "ANOMALY",
		Mode:                     "ANOMALY_AP",
		CharacterName:            "蕾米埃尔·丹",
		CharacterElement:         "LUMIFLUX",
		SetPattern:               "4+2",
		Required4Set:             "谶羽之誓",
		Required2Set:             "自由蓝调",
		BaseHP:                   7482,
		BaseATK:                  748,
		BaseDEF:                  600,
		BaseCritRate:             5,
		BaseCritDmg:              50,
		BaseAnomalyMastery:       115,
		BaseEnergyRegen:          1.2,
		TargetFinalAttack:        3903,
		TargetAnomalyProficiency: 457,
		TopN:                     20,
		TopKPerSlot:              80,
		MaxCombinations:          2000000,
		ExtraStats: map[string]float64{
			"BASE_ATK":            75 + 743,
			"ATK_PERCENT":         36,
			"ANOMALY_PROFICIENCY": 116 + 54,
		},
		CombatExtraStats: map[string]float64{
			"ANOMALY_PROFICIENCY": 96,
		},
		WantedWeights: roleEffectiveWeights("ANOMALY", "ANOMALY_AP", nil),
	}
}

func TestRemielleCoreFScreenshotPanelCalibration(t *testing.T) {
	req := remielleCoreFScreenshotRequest()
	res, ok := evaluateBuild(req.Discs, req, nil)
	if !ok {
		t.Fatal("蕾米埃尔核心技 F 实机六盘未通过 evaluateBuild")
	}
	if res.FinalHP != 9682 || res.FinalAttack != 3903 || res.FinalDefense != 859 {
		t.Fatalf("蕾米埃尔 F 面板生命/攻击/防御 = %.0f/%.0f/%.0f; want 9682/3903/859", res.FinalHP, res.FinalAttack, res.FinalDefense)
	}
	if !almostEqual(res.PanelCritRate, 19.4) || !almostEqual(res.PanelCritDmg, 59.6) {
		t.Fatalf("蕾米埃尔 F 面板暴击/暴伤 = %.1f/%.1f; want 19.4/59.6", res.PanelCritRate, res.PanelCritDmg)
	}
	if res.Stats["ANOMALY_PROFICIENCY"] != 457 || res.GameEffectiveWords != 29 {
		t.Fatalf("蕾米埃尔 F 异常精通/有效词条 = %.0f/%.0f; want 457/29", res.Stats["ANOMALY_PROFICIENCY"], res.GameEffectiveWords)
	}
	if res.Stats["ATK_PERCENT"] != 123 || res.Stats["ATK_FLAT"] != 411 || res.Stats["PEN_FLAT"] != 63 {
		t.Fatalf("蕾米埃尔 F 攻击%%/固定攻击/穿透值 = %.0f/%.0f/%.0f; want 123/411/63", res.Stats["ATK_PERCENT"], res.Stats["ATK_FLAT"], res.Stats["PEN_FLAT"])
	}
}

func TestRemielleCoreFScreenshotBuildIsOptimizable(t *testing.T) {
	resp := optimize(context.Background(), remielleCoreFScreenshotRequest())
	if len(resp.Results) != 1 {
		t.Fatalf("蕾米埃尔 F 实机六盘应返回 1 个方案，got %d; message=%s counts=%#v", len(resp.Results), resp.Message, resp.CandidateCounts)
	}
	result := resp.Results[0]
	if result.FinalAttack != 3903 || result.Stats["ANOMALY_PROFICIENCY"] != 457 || result.GameEffectiveWords != 29 {
		t.Fatalf("蕾米埃尔 F 优化结果 = attack %.0f / AP %.0f / words %.0f; want 3903 / 457 / 29", result.FinalAttack, result.Stats["ANOMALY_PROFICIENCY"], result.GameEffectiveWords)
	}
}

func TestRemielleCoreFBaselineIsNotDoubleCounted(t *testing.T) {
	index, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	marker := "name:'蕾米埃尔',fullName:'蕾米埃尔·丹',rank:'S',element:'LUMIFLUX',faction:'达识结社',role:'ANOMALY',hp:7482,atk:748,def:600,impact:83,baseAtkBonus:75,baseAnomalyProficiency:116"
	if !bytes.Contains(index, []byte(marker)) {
		t.Fatal("蕾米埃尔必须以未升级底数 748/116 建模，再由 F 节点加到 823/170")
	}
	if bytes.Contains(index, []byte("name:'蕾米埃尔',fullName:'蕾米埃尔·丹',rank:'S',element:'LUMIFLUX',faction:'达识结社',role:'ANOMALY',hp:7482,atk:823")) {
		t.Fatal("蕾米埃尔不得把核心技 F 总面板 823 当作未升级攻击底数")
	}
	for _, coreMarker := range []string{
		"function defaultCoreLevelForCharacter(c){return c?.coreDefault||'F';}",
		"function coreAdvancedRatio(levelIndex=selectedCoreLevelIndex()){return [1,3,5].filter(step=>levelIndex>=step).length/3;}",
		"function coreBaseRatio(levelIndex=selectedCoreLevelIndex()){return [2,4,6].filter(step=>levelIndex>=step).length/3;}",
	} {
		if !bytes.Contains(index, []byte(coreMarker)) {
			t.Fatalf("蕾米埃尔 F 核心技分层依赖的前端规则缺失: %s", coreMarker)
		}
	}
}

func TestDualReleaseRoutes(t *testing.T) {
	versionA, err := newAppMux(false)
	if err != nil {
		t.Fatal(err)
	}
	requestA := httptest.NewRequest(http.MethodGet, "/api/scanner/start", nil)
	responseA := httptest.NewRecorder()
	versionA.ServeHTTP(responseA, requestA)
	if responseA.Code != http.StatusNotFound {
		t.Fatalf("V1.05A scanner route status = %d; want 404", responseA.Code)
	}

	versionB, err := newAppMux(true)
	if err != nil {
		t.Fatal(err)
	}
	requestB := httptest.NewRequest(http.MethodGet, "/api/scanner/start", nil)
	responseB := httptest.NewRecorder()
	versionB.ServeHTTP(responseB, requestB)
	if responseB.Code != http.StatusMethodNotAllowed {
		t.Fatalf("V1.05B scanner route status = %d; want 405", responseB.Code)
	}
}

func TestVersion121AgentRosterAndDefenseSupport(t *testing.T) {
	index, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"凯撒", "赛斯", "本", "潘引壶", "照", "希希芙", "普罗米娅", "诺姆", "蕾米埃尔"} {
		if !bytes.Contains(index, []byte("name:'"+name+"'")) {
			t.Fatalf("v1.21 agent roster is missing %s", name)
		}
	}
	if bytes.Contains(index, []byte("name:'普尔克拉'")) {
		t.Fatal("duplicate agent 普尔克拉 should be removed; only 波可娜 is retained")
	}
	if !bytes.Contains(index, []byte("name:'照',rank:'S',element:'ICE',faction:'坎卜斯黑枝',role:'DEFENSE'")) {
		t.Fatal("照 should be classified as DEFENSE")
	}
	if bytes.Count(index, []byte("name:'星见雅'")) != 1 || !bytes.Contains(index, []byte("name:'星见雅',role:'ANOMALY'")) {
		t.Fatal("星见雅 should have exactly one ANOMALY record")
	}
	if bytes.Contains(index, []byte("特殊异常→强攻")) || bytes.Contains(index, []byte("specialNote")) {
		t.Fatal("obsolete special Attack entry for 星见雅 should be removed")
	}
	weights := roleEffectiveWeights("DEFENSE", "UTILITY_BALANCE", nil)
	if weights["DEF_PERCENT"] != 1 || weights["HP_PERCENT"] <= 0 || weights["ATK_PERCENT"] <= 0 {
		t.Fatalf("defense weights are incomplete: %#v", weights)
	}
	if !roleIsUtility("DEFENSE") {
		t.Fatal("DEFENSE should use utility target-window planning")
	}
	if got := calcFinalDefense(724, 0, 48, 184); got != 1255 {
		t.Fatalf("final defense = %.0f; want 1255", got)
	}
}

func TestVersion31RemielleAndDriveDiscSupport(t *testing.T) {
	index, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		"fullName:'蕾米埃尔·丹'", "element:'LUMIFLUX'", "faction:'达识结社'",
		"name:'空羽复归之诗'", "'谶羽之誓'", "'荆棘玫瑰'", "LUMIFLUX:'流明'",
	} {
		if !bytes.Contains(index, []byte(marker)) {
			t.Fatalf("v1.21 Remielle marker missing: %s", marker)
		}
	}
	if got, ok := ocrDefaultMainStatValue("LUMIFLUX_DMG"); !ok || got != 30 {
		t.Fatalf("LUMIFLUX_DMG default main stat = %v, %v; want 30, true", got, ok)
	}
	if slot, ok := uniqueOCRSlotForMainStat("LUMIFLUX_DMG"); !ok || slot != 5 {
		t.Fatalf("LUMIFLUX_DMG slot = %d, %v; want 5, true", slot, ok)
	}
	if got := statTypeFromOCRLabel("流明属性伤害加成 30%", true); got != "LUMIFLUX_DMG" {
		t.Fatalf("OCR stat type = %q; want LUMIFLUX_DMG", got)
	}
	if got := elementDamageStatKey("LUMIFLUX"); got != "LUMIFLUX_DMG" {
		t.Fatalf("Lumiflux element damage key = %q", got)
	}

	panel := map[string]float64{}
	applyTwoPiecePanelBonuses(panel, map[string]int{"谶羽之誓": 2, "荆棘玫瑰": 2})
	if panel["ANOMALY_PROFICIENCY"] != 30 || panel["DEF_PERCENT"] != 16 {
		t.Fatalf("3.1 two-piece bonuses = %#v", panel)
	}
	combat := map[string]float64{}
	applyFourPieceCombatBonuses(combat, map[string]int{"谶羽之誓": 4})
	applyConditionalFourPieceCombatBonuses(combat, map[string]int{"谶羽之誓": 4}, "LUMIFLUX")
	if combat["ANOMALY_PROFICIENCY"] != 50 || combat["ANOMALY_DMG_BONUS"] != 15 {
		t.Fatalf("Feathered Fate combat bonuses = %#v", combat)
	}
}

func TestInventoryCardViewportAndDetailLayout(t *testing.T) {
	index, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		"grid-auto-rows: 192px",
		"max-height: 798px",
		"overflow-y: auto",
		"width: 42px; height: 42px",
		"inventoryDetailSubStats",
		"inventoryDetailPrimaryStat",
		"font-size: 24px",
		"<strong>副属性</strong>",
		"Array.from({length:4}",
	} {
		if !bytes.Contains(index, []byte(marker)) {
			t.Fatalf("inventory UI regression marker missing: %s", marker)
		}
	}
}

func TestWindMainStatSupport(t *testing.T) {
	if got, ok := ocrDefaultMainStatValue("WIND_DMG"); !ok || got != 30 {
		t.Fatalf("WIND_DMG default main stat = %v, %v; want 30, true", got, ok)
	}
	if slot, ok := uniqueOCRSlotForMainStat("WIND_DMG"); !ok || slot != 5 {
		t.Fatalf("WIND_DMG slot = %d, %v; want 5, true", slot, ok)
	}
	if got := statTypeFromOCRLabel("风属性伤害加成 30%", true); got != "WIND_DMG" {
		t.Fatalf("OCR stat type = %q; want WIND_DMG", got)
	}
	if got := statCNName("WIND_DMG"); got != "风属性伤害" {
		t.Fatalf("Chinese stat name = %q; want 风属性伤害", got)
	}
}

func TestVersion30DriveDiscBonuses(t *testing.T) {
	panel := map[string]float64{}
	applyTwoPiecePanelBonuses(panel, map[string]int{"呼啸沙龙": 2, "拂晓行纪": 2})
	if panel["WIND_DMG"] != 10 || panel["ETHER_DMG"] != 10 {
		t.Fatalf("two-piece bonuses = %#v; want WIND_DMG=10 and ETHER_DMG=10", panel)
	}

	etherPanel := map[string]float64{}
	applyConditionalFourPiecePanelBonuses(etherPanel, map[string]int{"拂晓行纪": 4}, "ETHER")
	if etherPanel["CRIT_DMG"] != 0 {
		t.Fatalf("Dawn 4pc should not affect panel stats in v1.18, got %#v", etherPanel)
	}
	nonEtherPanel := map[string]float64{}
	applyConditionalFourPiecePanelBonuses(nonEtherPanel, map[string]int{"拂晓行纪": 4}, "WIND")
	if nonEtherPanel["CRIT_DMG"] != 0 {
		t.Fatalf("Dawn 4pc non-Ether bonus = %#v; want no panel CRIT_DMG", nonEtherPanel)
	}

	combat := map[string]float64{}
	applyFourPieceCombatBonuses(combat, map[string]int{"呼啸沙龙": 4, "拂晓行纪": 4})
	applyConditionalFourPieceCombatBonuses(combat, map[string]int{"拂晓行纪": 4}, "ETHER")
	if combat["ANOMALY_PROFICIENCY"] != 50 || combat["ELEMENT_DMG"] != 18 || combat["ATK_PERCENT"] != 10 || combat["CRIT_DMG"] != 30 {
		t.Fatalf("4pc combat bonuses = %#v; want AP=50, ELEMENT_DMG=18, ATK_PERCENT=10, CRIT_DMG=30", combat)
	}
	nonEtherCombat := map[string]float64{}
	applyFourPieceCombatBonuses(nonEtherCombat, map[string]int{"拂晓行纪": 4})
	applyConditionalFourPieceCombatBonuses(nonEtherCombat, map[string]int{"拂晓行纪": 4}, "WIND")
	if nonEtherCombat["CRIT_DMG"] != 0 || nonEtherCombat["ATK_PERCENT"] != 10 {
		t.Fatalf("Dawn 4pc non-Ether combat = %#v; want only triggered ATK_PERCENT=10", nonEtherCombat)
	}
}

func TestVelinaCoreConversion(t *testing.T) {
	panel := map[string]float64{"ENERGY_REGEN": 20}
	combat := cloneStatMap(panel)
	req := OptimizeRequest{
		CharacterName:      "维琳娜·艾嘉德",
		BaseEnergyRegen:    1.2,
		BaseAnomalyMastery: 112,
	}
	initial, finalMastery := applyCharacterCombatBonuses(combat, panel, req)
	if !almostEqual(initial, 1.44) {
		t.Fatalf("initial energy regen = %.12f; want 1.44", initial)
	}
	if !almostEqual(combat["VELINA_CORE_DMG_BONUS"], 5.04) {
		t.Fatalf("Velina core damage bonus = %.12f; want 5.04", combat["VELINA_CORE_DMG_BONUS"])
	}
	if !almostEqual(combat["ANOMALY_MASTERY_FLAT"], 12) {
		t.Fatalf("Velina flat anomaly mastery = %.12f; want 12", combat["ANOMALY_MASTERY_FLAT"])
	}
	if !almostEqual(finalMastery, 124) {
		t.Fatalf("final anomaly mastery = %.12f; want 124", finalMastery)
	}

	cappedPanel := map[string]float64{"ENERGY_REGEN": 200}
	cappedCombat := cloneStatMap(cappedPanel)
	_, cappedMastery := applyCharacterCombatBonuses(cappedCombat, cappedPanel, req)
	if cappedCombat["VELINA_CORE_DMG_BONUS"] != 35 || cappedCombat["ANOMALY_MASTERY_FLAT"] != 84 || cappedMastery != 196 {
		t.Fatalf("capped Velina conversion = damage %.3f, flat AM %.3f, final AM %.3f; want 35, 84, 196",
			cappedCombat["VELINA_CORE_DMG_BONUS"], cappedCombat["ANOMALY_MASTERY_FLAT"], cappedMastery)
	}
}

func TestElementAwareDamageBonus(t *testing.T) {
	stats := map[string]float64{
		"WIND_DMG":              10,
		"ETHER_DMG":             99,
		"ELEMENT_DMG":           18,
		"VELINA_CORE_DMG_BONUS": 5.04,
	}
	if got := combatDamageBonusPercent(stats, "WIND"); !almostEqual(got, 33.04) {
		t.Fatalf("Wind damage bonus = %.12f; want 33.04", got)
	}
	if got := combatDamageBonusPercent(stats, "ETHER"); !almostEqual(got, 122.04) {
		t.Fatalf("Ether damage bonus = %.12f; want 122.04", got)
	}
}

func sv(t string, v float64) StatValue { return StatValue{Type: t, Value: v} }

func testDisc(set string, slot int, main StatValue, subs ...StatValue) Disc {
	return Disc{ID: newID(), SetName: set, Slot: slot, Rarity: "S", Level: 15, MainStat: main, SubStats: subs}
}

func mustEvalWords(t *testing.T, name, role string, discs []Disc, required4, required2 string) float64 {
	t.Helper()
	res, ok := evaluateBuild(discs, OptimizeRequest{
		RoleSystem:    role,
		CharacterName: name,
		SetPattern:    "4+2",
		Required4Set:  required4,
		Required2Set:  required2,
		BaseCritRate:  5,
		BaseCritDmg:   50,
		WantedWeights: roleEffectiveWeights(role, "", nil),
	}, nil)
	if !ok {
		t.Fatalf("build for %s did not evaluate", name)
	}
	return res.GameEffectiveWords
}

func yixuanScreenshotDiscs() []Disc {
	return []Disc{
		testDisc("云岿如我", 1, sv("HP_FLAT", 2200), sv("HP_PERCENT", 6), sv("CRIT_RATE", 2.4), sv("ATK_PERCENT", 3), sv("CRIT_DMG", 19.2)),
		testDisc("折枝剑歌", 2, sv("ATK_FLAT", 316), sv("CRIT_RATE", 2.4), sv("CRIT_DMG", 14.4), sv("HP_PERCENT", 9), sv("ATK_PERCENT", 3)),
		testDisc("云岿如我", 3, sv("DEF_FLAT", 184), sv("DEF_PERCENT", 9.6), sv("CRIT_RATE", 4.8), sv("HP_PERCENT", 6), sv("CRIT_DMG", 14.4)),
		testDisc("折枝剑歌", 4, sv("CRIT_RATE", 24), sv("CRIT_DMG", 14.4), sv("HP_PERCENT", 9), sv("PEN_FLAT", 18), sv("ATK_FLAT", 19)),
		testDisc("云岿如我", 5, sv("ETHER_DMG", 30), sv("ATK_FLAT", 57), sv("CRIT_DMG", 9.6), sv("CRIT_RATE", 2.4), sv("HP_FLAT", 224)),
		testDisc("云岿如我", 6, sv("HP_PERCENT", 30), sv("CRIT_DMG", 14.4), sv("ATK_PERCENT", 6), sv("ATK_FLAT", 38), sv("CRIT_RATE", 4.8)),
	}
}

func TestProPanelCalibrationMatchesYixuanScreenshot(t *testing.T) {
	res, ok := evaluateBuild(yixuanScreenshotDiscs(), OptimizeRequest{
		RoleSystem:       "RUPTURE",
		CharacterName:    "仪玄",
		CharacterElement: "ETHER",
		SetPattern:       "4+2",
		Required4Set:     "云岿如我",
		Required2Set:     "折枝剑歌",
		BaseHP:           7953,
		BaseATK:          872,
		BaseCritRate:     5,
		BaseCritDmg:      50,
		HPToSheerRatio:   0.1,
		ExtraStats: map[string]float64{
			"BASE_HP":    420,
			"BASE_ATK":   743,
			"HP_PERCENT": 30,
			"CRIT_RATE":  14.4,
		},
		CombatExtraStats: map[string]float64{"CRIT_RATE": 20},
		WantedWeights:    roleEffectiveWeights("RUPTURE", "", nil),
	}, nil)
	if !ok {
		t.Fatal("仪玄截图配装未通过 evaluateBuild")
	}
	if res.FinalHP != 19170 || res.FinalAttack != 2238 {
		t.Fatalf("仪玄面板生命/攻击 = %.0f/%.0f; want 19170/2238", res.FinalHP, res.FinalAttack)
	}
	if !almostEqual(res.PanelCritRate, 60.2) || !almostEqual(res.PanelCritDmg, 152.4) {
		t.Fatalf("仪玄面板暴击/暴伤 = %.1f/%.1f; want 60.2/152.4", res.PanelCritRate, res.PanelCritDmg)
	}
	if res.SheerForce != 2588 {
		t.Fatalf("仪玄贯穿力 = %.0f; want 2588", res.SheerForce)
	}
	if res.GameEffectiveWords != 35 {
		t.Fatalf("仪玄游戏有效词条 = %.1f; want 35", res.GameEffectiveWords)
	}
	if res.Stats["ATK_PERCENT"] != 12 {
		t.Fatalf("不计为仪玄游戏有效词条的攻击力%%仍应进入面板汇总，got %.1f; want 12", res.Stats["ATK_PERCENT"])
	}
	if res.EffectiveWords <= res.GameEffectiveWords {
		t.Fatalf("总副词条 %.1f 应大于游戏有效词条 %.1f，以证明两种口径已拆分", res.EffectiveWords, res.GameEffectiveWords)
	}
}

func TestGameEffectiveWordsMatchUploadedPanelScreenshots(t *testing.T) {
	nangong := []Disc{
		testDisc("法厄同之歌", 1, sv("HP_FLAT", 2200), sv("ATK_FLAT", 19), sv("HP_PERCENT", 6), sv("DEF_PERCENT", 4.8), sv("ANOMALY_PROFICIENCY", 36)),
		testDisc("自由蓝调", 2, sv("ATK_FLAT", 316), sv("DEF_PERCENT", 9.6), sv("PEN_FLAT", 9), sv("HP_FLAT", 224), sv("ANOMALY_PROFICIENCY", 36)),
		testDisc("法厄同之歌", 3, sv("DEF_FLAT", 184), sv("DEF_PERCENT", 9.6), sv("CRIT_DMG", 9.6), sv("ANOMALY_PROFICIENCY", 36), sv("HP_FLAT", 112)),
		testDisc("法厄同之歌", 4, sv("ANOMALY_PROFICIENCY", 92), sv("CRIT_DMG", 4.8), sv("CRIT_RATE", 4.8), sv("ATK_PERCENT", 9), sv("ATK_FLAT", 57)),
		testDisc("法厄同之歌", 5, sv("ETHER_DMG", 30), sv("ANOMALY_PROFICIENCY", 18), sv("ATK_PERCENT", 12), sv("DEF_FLAT", 15), sv("ATK_FLAT", 19)),
		testDisc("自由蓝调", 6, sv("ANOMALY_MASTERY", 30), sv("ANOMALY_PROFICIENCY", 18), sv("ATK_PERCENT", 12), sv("DEF_FLAT", 15), sv("PEN_FLAT", 9)),
	}
	if got := mustEvalWords(t, "南宫羽", "STUN", nangong, "法厄同之歌", "自由蓝调"); got != 27 {
		t.Fatalf("南宫羽有效词条 = %.1f; want 27", got)
	}

	aria := []Disc{
		testDisc("荧光蛛眼", 1, sv("HP_FLAT", 2200), sv("ANOMALY_PROFICIENCY", 36), sv("CRIT_DMG", 9.6), sv("DEF_PERCENT", 4.8), sv("DEF_FLAT", 15)),
		testDisc("法厄同之歌", 2, sv("ATK_FLAT", 316), sv("ATK_PERCENT", 6), sv("ANOMALY_PROFICIENCY", 36), sv("CRIT_DMG", 4.8), sv("PEN_FLAT", 18)),
		testDisc("荧光蛛眼", 3, sv("DEF_FLAT", 184), sv("HP_FLAT", 224), sv("ATK_PERCENT", 6), sv("ATK_FLAT", 19), sv("ANOMALY_PROFICIENCY", 36)),
		testDisc("荧光蛛眼", 4, sv("ANOMALY_PROFICIENCY", 92), sv("HP_FLAT", 112), sv("ATK_FLAT", 19), sv("CRIT_DMG", 19.2), sv("ATK_PERCENT", 9)),
		testDisc("荧光蛛眼", 5, sv("ATK_PERCENT", 30), sv("ANOMALY_PROFICIENCY", 27), sv("HP_PERCENT", 6), sv("CRIT_DMG", 4.8), sv("DEF_FLAT", 15)),
		testDisc("法厄同之歌", 6, sv("ANOMALY_MASTERY", 30), sv("CRIT_DMG", 14.4), sv("DEF_FLAT", 15), sv("ANOMALY_PROFICIENCY", 27), sv("PEN_FLAT", 18)),
	}
	if got := mustEvalWords(t, "爱芮", "ANOMALY", aria, "荧光蛛眼", "法厄同之歌"); got != 25 {
		t.Fatalf("爱芮有效词条 = %.1f; want 25", got)
	}

	velina := []Disc{
		testDisc("月光骑士颂", 1, sv("HP_FLAT", 2200), sv("DEF_PERCENT", 14.4), sv("ATK_PERCENT", 6), sv("CRIT_DMG", 9.6), sv("ANOMALY_PROFICIENCY", 18)),
		testDisc("呼啸沙龙", 2, sv("ATK_FLAT", 316), sv("ANOMALY_PROFICIENCY", 36), sv("DEF_PERCENT", 4.8), sv("HP_FLAT", 112), sv("ATK_PERCENT", 6)),
		testDisc("呼啸沙龙", 3, sv("DEF_FLAT", 184), sv("ATK_FLAT", 19), sv("ANOMALY_PROFICIENCY", 18), sv("HP_PERCENT", 3), sv("ATK_PERCENT", 12)),
		testDisc("月光骑士颂", 4, sv("ANOMALY_PROFICIENCY", 92), sv("ATK_PERCENT", 9), sv("DEF_FLAT", 45), sv("PEN_FLAT", 9), sv("ATK_FLAT", 38)),
		testDisc("呼啸沙龙", 5, sv("ATK_PERCENT", 30), sv("ANOMALY_PROFICIENCY", 18), sv("ATK_PERCENT", 6), sv("ATK_FLAT", 57), sv("HP_FLAT", 112)),
		testDisc("呼啸沙龙", 6, sv("ENERGY_REGEN", 60), sv("ANOMALY_PROFICIENCY", 27), sv("CRIT_DMG", 4.8), sv("DEF_PERCENT", 9.6), sv("ATK_PERCENT", 3)),
	}
	if got := mustEvalWords(t, "维琳娜", "ANOMALY", velina, "呼啸沙龙", "月光骑士颂"); got != 27 {
		t.Fatalf("维琳娜有效词条 = %.1f; want 27", got)
	}

	yixuan := yixuanScreenshotDiscs()
	if got := mustEvalWords(t, "仪玄", "RUPTURE", yixuan, "云岿如我", "折枝剑歌"); got != 35 {
		t.Fatalf("仪玄有效词条 = %.1f; want 35", got)
	}
}

func TestAllTargetPrioritiesDefaultToOneAndMultiResultShowsAllStats(t *testing.T) {
	index, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"priorityCrit", "priorityCritDmg", "priorityAtk", "priorityHp", "priorityDef", "priorityAp"} {
		start := bytes.Index(index, []byte(`<select id="`+id+`">`))
		if start < 0 {
			t.Fatalf("priority selector missing: %s", id)
		}
		endOffset := bytes.Index(index[start:], []byte(`</select>`))
		if endOffset < 0 {
			t.Fatalf("priority selector is not closed: %s", id)
		}
		selector := index[start : start+endOffset]
		if !bytes.Contains(selector, []byte(`<option value="1">1（最高）</option>`)) || bytes.Contains(selector, []byte(` selected`)) {
			t.Fatalf("priority selector %s does not default to 1: %s", id, selector)
		}
	}
	for _, marker := range []string{
		`<div class="assignedBuildTitle" style="margin-top:12px">全部属性</div>`,
		`function allResultAttributesHtml(res){`,
		`['面板生命',finalHp,'']`,
		`allResultAttributesHtml(result)`,
		`const extras=[...new Set([...Object.keys(res.stats||{}),...Object.keys(res.combatStats||{})])]`,
		`priorityCritDmg:p.CRIT_DMG||1`,
		`priorityAp:p.ANOMALY_PROFICIENCY||1`,
		`{priorityCrit:1,priorityCritDmg:1,priorityAtk:1,priorityHp:1,priorityDef:1,priorityAp:1}`,
	} {
		if !bytes.Contains(index, []byte(marker)) {
			t.Fatalf("default-priority/full-stat marker missing: %s", marker)
		}
	}

	req := OptimizeRequest{TargetCritRate: 80, TargetCritDmg: 170, TargetFinalAttack: 4000, TargetFinalHP: 10000, TargetFinalDefense: 1000, TargetAnomalyProficiency: 400}
	_, _, gaps := strictTargetPenalty(map[string]float64{}, 5, 50, 1000, 1000, 100, 0, req)
	if gaps[0] <= 0 {
		t.Fatalf("default priority 1 gap is empty: %#v", gaps)
	}
	for priority := 1; priority < len(gaps); priority++ {
		if gaps[priority] != 0 {
			t.Fatalf("unset priorities should all default to level 1: %#v", gaps)
		}
	}
}

func TestSingleCharacterMultiRecalculationControls(t *testing.T) {
	index, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		`data-multi-action="recalculate"`,
		`async function recalculateMultiPlan(plan){`,
		`for(let i=0;i<targetIndex;i++)`,
		`availableDiscsForPlan(plan,reserved)`,
		`for(let i=targetIndex+1;i<ordered.length;i++)`,
		`multiRunResults.delete(ordered[i].plan.id)`,
		`button.dataset.multiAction==='recalculate'`,
	} {
		if !bytes.Contains(index, []byte(marker)) {
			t.Fatalf("single-character recalculation marker missing: %s", marker)
		}
	}
}

func TestTargetInputsStartBlankWithoutRoleDefaults(t *testing.T) {
	index, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"targetCritRate", "targetCritDmg", "targetFinalAttack", "targetFinalHP", "targetFinalDefense", "targetAnomalyProficiency"} {
		marker := []byte(`id="` + id + `"`)
		start := bytes.Index(index, marker)
		if start < 0 {
			t.Fatalf("target input missing: %s", id)
		}
		end := bytes.IndexByte(index[start:], '>')
		if end < 0 || !bytes.Contains(index[start:start+end], []byte(`value=""`)) {
			t.Fatalf("target input %s does not start blank", id)
		}
	}
	for _, removed := range []string{"异常默认启用攻击力和异常精通", "value==='0')$('#targetCritRate').value='80'", "activeDefaults.has(field)"} {
		if bytes.Contains(index, []byte(removed)) {
			t.Fatalf("removed role target default is still present: %s", removed)
		}
	}
	for _, marker := range []string{"for(const label of $$('[data-target-field]'))", "$('#targetCritRate').value=''", "填写数值后才参与筛选"} {
		if !bytes.Contains(index, []byte(marker)) {
			t.Fatalf("blank target behavior marker missing: %s", marker)
		}
	}
}

func TestStrictTargetReasonShowsDifferenceDirection(t *testing.T) {
	req := OptimizeRequest{
		BaseATK:           1000,
		TargetCritRate:    80,
		TargetFinalAttack: 4000,
		TargetPriorities:  map[string]int{"CRIT_RATE": 1, "ATK": 1},
	}
	_, lowParts, _ := strictTargetPenalty(map[string]float64{}, 80, 50, 3900, 0, 0, 0, req)
	lowText := strings.Join(lowParts, "；")
	if !strings.Contains(lowText, "攻击力 3900.0/4000.0（约差 ") {
		t.Fatalf("low target reason does not say shortfall: %s", lowText)
	}
	if !strings.Contains(lowText, "暴击率 80.0/80.0（正好达到）") {
		t.Fatalf("exact target reason is unclear: %s", lowText)
	}

	_, highParts, _ := strictTargetPenalty(map[string]float64{}, 85, 50, 4100, 0, 0, 0, req)
	highText := strings.Join(highParts, "；")
	if !strings.Contains(highText, "暴击率 85.0/80.0（约高出 ") || !strings.Contains(highText, "攻击力 4100.0/4000.0（约高出 ") {
		t.Fatalf("high target reason does not say overflow: %s", highText)
	}
}

func TestCustomEffectiveWordStats(t *testing.T) {
	req := OptimizeRequest{EffectiveWordStats: []string{"CRIT_RATE", "CRIT_DMG", "ATK_PERCENT", "ATK_FLAT", "UNKNOWN"}}
	set := gameEffectiveStatSet(req)
	for _, statType := range []string{"CRIT_RATE", "CRIT_DMG", "ATK_PERCENT", "ATK_FLAT"} {
		if !set[statType] {
			t.Fatalf("selected effective stat missing: %s in %#v", statType, set)
		}
	}
	if set["UNKNOWN"] || len(set) != 4 {
		t.Fatalf("unsupported effective stats were not filtered: %#v", set)
	}
	if got := statWords(StatValue{Type: "CRIT_RATE", Value: 7.2}); math.Abs(got-3) > 1e-9 {
		t.Fatalf("crit-rate word conversion = %.3f; want 3", got)
	}
	if got := statWords(StatValue{Type: "ATK_FLAT", Value: 38}); math.Abs(got-2) > 1e-9 {
		t.Fatalf("flat-attack word conversion = %.3f; want 2", got)
	}
	if got := gameEffectiveStatSet(OptimizeRequest{EffectiveWordStats: []string{}}); len(got) != 0 {
		t.Fatalf("explicit empty effective stat selection should count nothing: %#v", got)
	}
}

func TestEffectiveWordCheckboxesPersistPerCharacterPlan(t *testing.T) {
	index, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		`id="effectiveCrit" type="checkbox"`,
		`id="effectiveCritDmg" type="checkbox"`,
		`id="effectiveAtk" type="checkbox"`,
		`id="effectiveHp" type="checkbox"`,
		`id="effectiveDef" type="checkbox"`,
		`id="effectiveAp" type="checkbox"`,
		`effectiveAtk:['ATK_PERCENT','ATK_FLAT']`,
		`effectiveHp:['HP_PERCENT','HP_FLAT']`,
		`effectiveDef:['DEF_PERCENT','DEF_FLAT']`,
		`effectiveWordStats:selectedEffectiveWordStats()`,
		`request.effectiveWordStats=effectiveWordStats`,
		`applyEffectiveWordStats(u.effectiveWordStats??plan.request?.effectiveWordStats??[])`,
		`applyEffectiveWordStats([])`,
	} {
		if !bytes.Contains(index, []byte(marker)) {
			t.Fatalf("custom effective-word UI marker missing: %s", marker)
		}
	}
	if bytes.Contains(index, []byte(`id="effectiveCrit" type="checkbox" checked`)) {
		t.Fatal("effective-word checkbox should not be selected by default")
	}
}
