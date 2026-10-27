package main

import (
	"math"
	"testing"
)

func TestRoxyInitialEnergyConversionAndSquadBuff(t *testing.T) {
	for _, tc := range []struct{ energy, attack, impact float64 }{{1.2, 0, 0}, {1.209, 0, 0}, {1.21, 5, 0.4}, {2.64, 720, 57.6}, {3.12, 960, 76.8}, {9, 960, 76.8}} {
		req := OptimizeRequest{CharacterName: "洛克茜·伊芙莉塔·普莱斯", RoleSystem: "STUN", BaseEnergyRegen: 1.2, BaseImpact: 100, BaseCritRate: 5, BaseCritDmg: 50,
			ExtraStats:       map[string]float64{"ENERGY_REGEN": (tc.energy/1.2 - 1) * 100, "CRIT_RATE": 14.4},
			CombatExtraStats: map[string]float64{"ROXY_ATK_CAP": 960, "ROXY_IMPACT_CAP": 76.8, "ROXY_CRIT_DMG_CAP": 40, "ROXY_LACERATION_CAP": 20, "CRIT_RATE": 24, "ENERGY_REGEN": 999}}
		res, ok := evaluateBuild(nil, req, nil)
		if !ok || math.Abs(res.CombatStats["ATK_FLAT"]-tc.attack) > 1e-6 || math.Abs(res.CombatStats["FINAL_IMPACT"]-(100+tc.impact)) > 1e-6 {
			t.Fatalf("energy %.3f: %+v", tc.energy, res)
		}
		if res.PanelCritRate != 19.4 || res.PanelImpact != 100 || res.CritDmg != 67.36 || res.CombatStats["ROXY_SQUAD_LACERATION_DMG"] != 8.68 {
			t.Fatalf("panel/combat separation: %+v", res)
		}
	}
}

func TestRoxyEveryCoreCapAndNoBuffLeak(t *testing.T) {
	for i, impact := range []float64{40, 46, 52, 58, 64, 70, 76.8} {
		req := OptimizeRequest{CharacterName: "洛克茜", RoleSystem: "STUN", BaseCritRate: 150}
		combat := map[string]float64{"ROXY_ATK_CAP": float64(480 + i*80), "ROXY_IMPACT_CAP": impact, "ROXY_CRIT_DMG_CAP": float64(22 + i*3), "ROXY_LACERATION_CAP": 10 + float64(i)*1.5}
		applyNewAgentCombatBonuses(combat, nil, req, 10)
		if combat["ATK_FLAT"] != float64(480+i*80) || combat["ROXY_IMPACT_FLAT"] != impact || combat["CRIT_DMG"] != float64(22+i*3) {
			t.Fatalf("core %d: %+v", i, combat)
		}
		req.CharacterName = "青衣"
		other := map[string]float64{}
		applyNewAgentCombatBonuses(other, nil, req, 10)
		if len(other) != 0 {
			t.Fatal("Roxy passive leaked to another agent")
		}
	}
}

func TestPhoenixAnomalyCritUsesInitialMasteryNotNormalCrit(t *testing.T) {
	for _, tc := range []struct{ mastery, rate float64 }{{116, 30}, {155, 30}, {200, 61.5}, {254, 99.3}, {255, 100}, {300, 100}} {
		req := OptimizeRequest{CharacterName: "菲欧妮·蕾法爱菈", RoleSystem: "ANOMALY", Mode: "ANOMALY_AP", BaseATK: 797, BaseCritRate: 5, BaseCritDmg: 50, BaseAnomalyMastery: tc.mastery,
			ExtraStats:       map[string]float64{"ANOMALY_PROFICIENCY": 118},
			CombatExtraStats: map[string]float64{"ANOMALY_PROFICIENCY": 85, "ANOMALY_CRIT_DMG": 25, "ANOMALY_MASTERY": 999, "CRIT_RATE": 999, "CRIT_DMG": 999}}
		res, ok := evaluateBuild(nil, req, nil)
		want := round(797*2.03*(1+tc.rate/100*0.25), 3)
		if !ok || res.CombatStats["ANOMALY_CRIT_RATE"] != tc.rate || res.DamageIndex != want || res.Stats["ANOMALY_PROFICIENCY"] != 118 {
			t.Fatalf("mastery %.0f: %+v, want index %v", tc.mastery, res, want)
		}
		req.CombatExtraStats["CRIT_RATE"], req.CombatExtraStats["CRIT_DMG"] = 0, 0
		plain, _ := evaluateBuild(nil, req, nil)
		if plain.DamageIndex != res.DamageIndex || plain.Score != res.Score {
			t.Fatal("normal crit changed Phoenix anomaly ranking")
		}
	}
}
