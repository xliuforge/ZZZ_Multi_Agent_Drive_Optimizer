package main

import (
	"encoding/json"
	"testing"
)

func TestSigridPanelAndCombatSeparation(t *testing.T) {
	data, err := webFiles.ReadFile("web/data/characters.json")
	if err != nil {
		t.Fatal(err)
	}
	var characters []struct {
		Name              string               `json:"name"`
		ATK               float64              `json:"atk"`
		Extra             map[string]float64   `json:"extra"`
		BaseAtkBonus      float64              `json:"baseAtkBonus"`
		CoreCombatByLevel []map[string]float64 `json:"coreCombatByLevel"`
	}
	if err := json.Unmarshal(data, &characters); err != nil {
		t.Fatal(err)
	}
	for _, c := range characters {
		if c.Name != "希格莉德" {
			continue
		}
		req := OptimizeRequest{BaseATK: c.ATK, BaseCritRate: 5, BaseCritDmg: 50, RoleSystem: "ATTACK", Mode: "MAX_CD",
			ExtraStats:       map[string]float64{"BASE_ATK": c.BaseAtkBonus + 713, "CRIT_RATE": c.Extra["CRIT_RATE"], "CRIT_DMG": 48},
			CombatExtraStats: map[string]float64{"CRIT_RATE": c.CoreCombatByLevel[6]["CRIT_RATE"], "CRIT_DMG": 64, "ICE_RES_IGNORE": 20},
		}
		res, ok := evaluateBuild(nil, req, nil)
		if !ok {
			t.Fatal("Sigrid build rejected")
		}
		if res.PanelCritRate != 19.4 || res.CritRate != 85.4 || res.PanelCritDmg != 98 || res.CritDmg != 162 || res.FinalAttack != 1651 {
			t.Fatalf("unexpected panel/combat stats: %+v", res)
		}
		// Resistance ignore is metadata, not a generic damage bonus.
		delete(req.CombatExtraStats, "ICE_RES_IGNORE")
		withoutIgnore, _ := evaluateBuild(nil, req, nil)
		if withoutIgnore.DamageIndex != res.DamageIndex {
			t.Fatal("ice resistance ignore changed generic damage")
		}
		req.ExtraStats["CRIT_RATE"] = 29 // 34% panel + 66% triggered = 100%.
		capped, _ := evaluateBuild(nil, req, nil)
		req.ExtraStats["CRIT_RATE"] = 40
		overflow, _ := evaluateBuild(nil, req, nil)
		if capped.CritRate != 100 || overflow.CritRate != 100 || capped.DamageIndex != overflow.DamageIndex {
			t.Fatal("crit overflow incorrectly changes damage")
		}
		// Capping after removing over-target panel crit must still leave 100%
		// combat crit at a 34% target. The only loss is the explicit fit penalty.
		req.Mode = ""
		req.TargetCritRate = 34
		req.ExtraStats["CRIT_RATE"] = 30
		nearTarget, ok := evaluateBuild(nil, req, nil)
		_, _, penalty := critTargetPenalty(35, 34)
		wantScore := round(capped.OutputScore*(1-penalty)+98*2, 4)
		if !ok || !almostEqual(nearTarget.Score, wantScore) {
			t.Fatalf("score with triggered crit and panel overflow = %v; want %v", nearTarget.Score, wantScore)
		}
		return
	}
	t.Fatal("missing Sigrid")
}
