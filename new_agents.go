package main

import (
	"math"
	"strings"
)

func isRoxy(req OptimizeRequest) bool {
	return strings.EqualFold(req.RoleSystem, "STUN") && agentNameContains(normalizedAgentName(req.CharacterName), "洛克茜")
}

func isPhoenix(req OptimizeRequest) bool {
	return strings.EqualFold(req.RoleSystem, "ANOMALY") && agentNameContains(normalizedAgentName(req.CharacterName), "菲欧妮")
}

// Conversions read initial panel values. Triggered energy/mastery buffs must not
// feed back into these passives. Core parameters are refreshed by the client.
func applyNewAgentCombatBonuses(combat, panel map[string]float64, req OptimizeRequest, energy float64) {
	if isRoxy(req) {
		steps := math.Max(0, math.Floor((energy-1.2+1e-9)/0.01))
		combat["ATK_FLAT"] += math.Min(combat["ROXY_ATK_CAP"], steps*5)
		impact := math.Min(combat["ROXY_IMPACT_CAP"], steps*0.4)
		combat["ROXY_IMPACT_FLAT"] = impact
		combat["FINAL_IMPACT"] = req.BaseImpact*(1+combat["IMPACT"]/100) + impact
		rate := math.Max(0, req.BaseCritRate+req.ExtraCritRate+combat["CRIT_RATE"])
		combat["ROXY_SQUAD_CRIT_DMG"] = math.Min(combat["ROXY_CRIT_DMG_CAP"], rate*0.4)
		combat["ROXY_SQUAD_LACERATION_DMG"] = math.Min(combat["ROXY_LACERATION_CAP"], rate*0.2)
		// Roxy herself deals Wind damage and benefits from the squad crit buff.
		combat["CRIT_DMG"] += combat["ROXY_SQUAD_CRIT_DMG"]
	}
	if isPhoenix(req) {
		mastery := calcPanelAnomalyMastery(req.BaseAnomalyMastery, panel)
		combat["ANOMALY_CRIT_RATE"] = math.Min(100, 30+math.Max(0, mastery-155)*0.7)
		combat["ANOMALY_CRIT_MULTIPLIER"] = calcCritMultiplier(combat["ANOMALY_CRIT_RATE"], combat["ANOMALY_CRIT_DMG"])
	}
}

// An anomaly proxy for comparing gear, without skill multipliers or enemy data.
// Ordinary CR/CD never affect Phoenix's anomaly crits.
func phoenixDamageIndex(combat map[string]float64, attack, damageBonus float64) float64 {
	return attack * math.Max(0, combat["ANOMALY_PROFICIENCY"]) / 100 * combat["ANOMALY_CRIT_MULTIPLIER"] * (1 + (damageBonus+combat["ANOMALY_DMG_BONUS"])/100)
}
