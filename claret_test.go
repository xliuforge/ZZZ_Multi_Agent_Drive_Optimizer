package main

import (
	"context"
	"math"
	"testing"
)

func TestClaretSearchPrefersDefenseAndKeepsHighCritTarget(t *testing.T) {
	req := claretTestRequest()
	req.SetPattern, req.Required4Set, req.Required2Set = "4+2", "荆棘玫瑰", "雷暴重金属"
	req.Discs = []Disc{
		testDisc("荆棘玫瑰", 1, sv("HP_FLAT", 2200)),
		testDisc("荆棘玫瑰", 2, sv("ATK_FLAT", 316)),
		testDisc("荆棘玫瑰", 3, sv("DEF_FLAT", 184)),
		testDisc("荆棘玫瑰", 4, sv("CRIT_RATE", 24)),
		testDisc("雷暴重金属", 5, sv("ELECTRIC_DMG", 30)),
		testDisc("雷暴重金属", 6, sv("DEF_PERCENT", 48)),
	}
	attackDisc := testDisc("雷暴重金属", 6, sv("ATK_PERCENT", 30))
	attackDisc.ID += "-attack"
	req.Discs = append(req.Discs, attackDisc)
	req.ExtraStats["CRIT_RATE"] += 60
	req.Mode, req.TargetCritRate = "STRICT_TARGETS", 135.3
	resp := optimize(context.Background(), req)
	if len(resp.Results) != 2 {
		t.Fatalf("search: %+v", resp)
	}
	best := resp.Results[0]
	if best.PanelCritRate != 135.3 || best.CritRate != 200 || best.CombatStats["ELEMENT_DMG"] != 15 {
		t.Fatalf("high crit/rose: %+v", best)
	}
	if best.Discs[5].MainStat.Type != "DEF_PERCENT" || best.DamageIndex <= resp.Results[1].DamageIndex {
		t.Fatal("search preferred attack over defense")
	}
	defScore := discRoughScore(req.Discs[5], nil, "ARMORER_DEF", 0, "", "", "ARMORER")
	atkScore := discRoughScore(attackDisc, nil, "ARMORER_DEF", 0, "", "", "ARMORER")
	if defScore <= atkScore {
		t.Fatal("candidate pruning discarded defense main stat")
	}
}

func TestLacerationMultiCrit(t *testing.T) {
	for _, tc := range []struct{ rate, want float64 }{{-10, 1}, {0, 1}, {50, 1.75}, {100, 2.5}, {150, 4.375}, {170, 5.125}, {200, 6.25}, {250, 6.25}} {
		if got := calcLacerationMultiplier(tc.rate, 150); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("rate %v: got %v, want %v", tc.rate, got, tc.want)
		}
	}
}

func claretTestRequest() OptimizeRequest {
	return OptimizeRequest{CharacterName: "克拉蕾·弗林特", CharacterElement: "ELECTRIC", RoleSystem: "ARMORER", Mode: "ARMORER_DEF", BaseDEF: 441, BaseHP: 5651, BaseCritRate: 5, BaseCritDmg: 50,
		ExtraStats:       map[string]float64{"CRIT_RATE": 28.8, "BASE_DEF": 431, "DEF_PERCENT": 48, "LACERATION_DMG": 150, "SHARPNESS_REGEN": 1.5},
		CombatExtraStats: map[string]float64{"CRIT_RATE": 55, "ELECTRIC_DMG": 15, "ELECTRIC_SHARP_DMG": 10}}
}

func TestClaretDefenseAndInitialCritConversion(t *testing.T) {
	req := claretTestRequest()
	base, ok := evaluateBuild(nil, req, nil)
	if !ok || base.PanelCritRate != 51.3 || base.CritRate != 106.3 || base.FinalDefense != 1290 {
		t.Fatalf("unexpected Claret panel/combat: %+v", base)
	}
	want := round(1290*calcLacerationMultiplier(106.3, 150)*1.25, 3)
	if base.DamageIndex != want {
		t.Fatalf("damage %v want %v", base.DamageIndex, want)
	}
	req.CombatExtraStats["CRIT_DMG"] = 100
	req.CombatExtraStats["ATK_PERCENT"] = 200
	req.BaseATK = 9999
	ignored, _ := evaluateBuild(nil, req, nil)
	if ignored.Score != base.Score || ignored.CritRate != base.CritRate {
		t.Fatal("combat CD or attack changed Claret output")
	}
	req.ExtraStats["CRIT_DMG"] = 100
	converted, _ := evaluateBuild(nil, req, nil)
	if converted.PanelCritRate != 86.3 || converted.CritRate != 141.3 || converted.DamageIndex <= base.DamageIndex {
		t.Fatalf("initial conversion incorrect: %+v", converted)
	}
	req.ExtraStats["CRIT_RATE"] = 250
	cap, _ := evaluateBuild(nil, req, nil)
	req.ExtraStats["CRIT_DMG"] += 100
	over, _ := evaluateBuild(nil, req, nil)
	if cap.CritRate != 200 || cap.CritMultiplier != 6.25 || cap.Score != over.Score || over.PanelCritRate <= 200 {
		t.Fatal("second crit cap or overcap score incorrect")
	}
	// An ordinary attack agent must not acquire Claret's conversion.
	req.RoleSystem = "ATTACK"
	if claretInitialCritBonus(req, 150) != 0 {
		t.Fatal("conversion leaked to other roles")
	}
}

func TestThornedRoseUsesInitialDefense(t *testing.T) {
	for _, tc := range []struct{ defense, crit float64 }{{999, 0}, {1000, 8}, {1799, 8}, {1800, 16}} {
		combat := map[string]float64{"DEF_PERCENT": 900}
		applyThornedRoseCombatBonus(combat, map[string]float64{}, map[string]int{"荆棘玫瑰": 4}, tc.defense)
		if combat["CRIT_RATE"] != tc.crit || combat["ELEMENT_DMG"] != 15 {
			t.Fatalf("def %v: %+v", tc.defense, combat)
		}
	}
	combat := map[string]float64{}
	applyThornedRoseCombatBonus(combat, nil, map[string]int{"荆棘玫瑰": 2}, 2000)
	if len(combat) != 0 {
		t.Fatal("two-piece triggered four-piece effects")
	}
}
