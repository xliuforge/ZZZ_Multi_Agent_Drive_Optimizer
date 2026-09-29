package main

import (
	"math"
	"strings"
)

func roleIsArmorer(role string) bool { return strings.EqualFold(strings.TrimSpace(role), "ARMORER") }

// Sharp damage uses defense and laceration damage, including a second crit
// above 100% CR. Ordinary crit damage only contributes through Claret's core.
func calcLacerationMultiplier(critRate, laceration float64) float64 {
	first := math.Max(0, math.Min(1, critRate/100))
	second := math.Max(0, math.Min(1, (critRate-100)/100))
	return (1 + first*laceration/100) * (1 + second*laceration/100)
}

func claretInitialCritBonus(req OptimizeRequest, panelCritDmg float64) float64 {
	if roleIsArmorer(req.RoleSystem) && agentNameContains(normalizedAgentName(req.CharacterName), "克拉蕾") {
		return math.Max(0, panelCritDmg) * 0.35
	}
	return 0
}

func applyThornedRoseCombatBonus(combat, panel map[string]float64, sets map[string]int, baseDEF float64) {
	if sets["荆棘玫瑰"] < 4 {
		return
	}
	combat["ELEMENT_DMG"] += 15
	initialDEF := calcFinalDefense(baseDEF, panel["BASE_DEF"], panel["DEF_PERCENT"], panel["DEF_FLAT"])
	if initialDEF >= 1800 {
		combat["CRIT_RATE"] += 16
	} else if initialDEF >= 1000 {
		combat["CRIT_RATE"] += 8
	}
}
