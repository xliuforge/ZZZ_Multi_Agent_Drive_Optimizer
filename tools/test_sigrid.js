const fs = require('fs');
const vm = require('vm');
const assert = require('assert/strict');
const path = require('path');
process.chdir(path.resolve(__dirname, '..'));
const html = fs.readFileSync('web/index.html', 'utf8').replaceAll('__SCANNER_AVAILABLE__', 'false');
const script = [...html.matchAll(/<script(?:\s[^>]*)?>([\s\S]*?)<\/script>/g)].map(m => m[1]).find(s => s.includes('const STAT_LABELS'));
const nodes = new Map();
const node = selector => { if (!nodes.has(selector)) nodes.set(selector, {value: '', options: []}); return nodes.get(selector); };
const context = vm.createContext({window: {addEventListener() {}}, localStorage: {getItem() {return null;}}, document: {querySelector: node}, console});
vm.runInContext(script.slice(0, script.lastIndexOf('(async function init()')), context);
const characters = JSON.parse(fs.readFileSync('web/data/characters.json', 'utf8'));
const engines = JSON.parse(fs.readFileSync('web/data/wengines.json', 'utf8'));
context.characters = characters; context.engines = engines;
vm.runInContext('CHARACTERS=characters; WENGINES=engines; state={discs:[],setEffects:[]};', context);
const c = characters.find(c => c.name === '希格莉德');
const e = engines.find(e => e.character === c.name);
const panelCR = [5, 9.8, 9.8, 14.6, 14.6, 19.4, 19.4];
const attack = [863, 863, 888, 888, 913, 913, 938];
for (let level = 0; level < 7; level++) {
  const panel = context.characterStats(c, level);
  assert.ok(Math.abs(5 + (panel.CRIT_RATE || 0) - panelCR[level]) < 1e-9);
  assert.equal(context.characterPanelBase(c, level).atk, attack[level]);
  assert.equal(context.characterCombatStats(c, level).CRIT_RATE, [33,39,44,50,55,61,66][level]);
  // Recalculation must rebuild both core and engine effects, without retaining stale bonuses.
  const plan = {characterName: c.name, ui: {coreLevel: ['0','A','B','C','D','E','F'][level], wEngineName: e.name, wEngineRank: 1}, request: {combatExtraStats: {CRIT_RATE: 999}}};
  const req = context.requestForMultiPlan(plan, new Set());
  assert.equal(req.combatExtraStats.CRIT_RATE, [33,39,44,50,55,61,66][level]);
  assert.equal(req.combatExtraStats.CRIT_DMG, 64);
  assert.equal(req.extraStats.BASE_ATK, attack[level] - 863 + 713);
  assert.equal(req.extraStats.CRIT_DMG, 48);
}
for (let rank = 1; rank <= 5; rank++) {
  assert.equal(context.wEngineCombatStats(e, rank).CRIT_DMG, [64,73.6,83.2,92.8,102.4][rank-1]);
}
const noEngine = context.requestForMultiPlan({characterName:c.name,ui:{coreLevel:'F'},request:{}},new Set());
assert.equal(noEngine.combatExtraStats.CRIT_RATE,66);
assert.equal(noEngine.combatExtraStats.CRIT_DMG,undefined);
assert.equal(Object.keys(context.characterCombatStats(characters.find(c=>c.name==='艾莲'),6)).length,0);
assert.ok(context.statMapText({ICE_RES_IGNORE:20}).includes('20%'));
console.log('SIGRID_OK: core 0–F, engine 1–5, saved-plan refresh, no-engine and other-character cases');
