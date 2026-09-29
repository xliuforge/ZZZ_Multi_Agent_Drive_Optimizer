package main

import (
	"encoding/json"
	"testing"
)

func TestRupturePanelSeparatesTriggeredStatsAndEnergy(t *testing.T) {
	req := OptimizeRequest{CharacterName: "星徽·比利", RoleSystem: "RUPTURE", BaseHP: 8497, BaseATK: 784, BaseDEF: 445, BaseCritRate: 5, BaseCritDmg: 50, HPToSheerRatio: 0.1,
		ExtraStats:       map[string]float64{"BASE_ATK": 75, "CRIT_RATE": 14.4, "ADRENALINE_REGEN": 2, "ENERGY_REGEN": 60},
		CombatExtraStats: map[string]float64{"ATK_PERCENT": 100, "HP_PERCENT": 100, "SHEER_FORCE_FLAT": 123, "CRIT_RATE": 30},
	}
	res, ok := evaluateBuild(nil, req, nil)
	if !ok || res.PanelSheerForce != 1106 || res.SheerForce != 2337 || res.PanelCritRate != 19.4 || res.CritRate != 49.4 {
		t.Fatalf("panel leaked triggered effects: %+v", res)
	}
	if res.PanelAdrenalineRegen != 2 || res.PanelEnergyRegen != 0 || res.RoleSystem != "RUPTURE" {
		t.Fatalf("wrong resource/role: %+v", res)
	}
	// Zero is a known value for Manato, not a missing field or the default 2.
	delete(req.ExtraStats, "ADRENALINE_REGEN")
	req.CharacterName = "真斗"
	zero, _ := evaluateBuild(nil, req, nil)
	data, err := json.Marshal(zero)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["panelAdrenalineRegen"] != float64(0) {
		t.Fatal("zero regeneration missing from response")
	}
}

func TestRuptureResourceBaseData(t *testing.T) {
	data, err := webFiles.ReadFile("web/data/characters.json")
	if err != nil {
		t.Fatal(err)
	}
	var characters []struct {
		Name  string   `json:"name"`
		Role  string   `json:"role"`
		Regen *float64 `json:"baseAdrenalineRegen"`
	}
	if err := json.Unmarshal(data, &characters); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, c := range characters {
		if c.Role != "RUPTURE" {
			continue
		}
		count++
		want := 2.0
		if c.Name == "真斗" {
			want = 0
		}
		if c.Regen == nil || *c.Regen != want {
			t.Errorf("%s regeneration missing or incorrect", c.Name)
		}
	}
	if count != 5 {
		t.Fatalf("checked %d rupture characters, want 5", count)
	}
}
