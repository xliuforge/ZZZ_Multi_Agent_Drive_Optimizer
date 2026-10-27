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
const near = (actual, expected) => assert.ok(Math.abs(actual-expected)<1e-8, `${actual} != ${expected}`);
for (const name of ['洛克茜','菲欧妮']) {
  const c = characters.find(c => c.name === name), e = engines.find(e => e.character === name);
  assert.ok(c && e);
  for (let level = 0; level < 7; level++) {
    for (let rank = 1; rank <= 5; rank++) {
      const plan = {characterName: name, ui: {coreLevel: ['0','A','B','C','D','E','F'][level], wEngineName: e.name, wEngineRank: rank}, request: {extraStats:{CRIT_RATE:999},combatExtraStats:{CRIT_RATE:999,ANOMALY_PROFICIENCY:999}}};
      const req = context.requestForMultiPlan(plan, new Set());
      assert.equal(req.characterName,c.fullName);
      assert.equal(req.roleSystem,c.role);
      assert.equal(req.extraStats.BASE_ATK,713+Math.floor(level/2)*25);
      if (name === '洛克茜') {
        near(req.extraStats.CRIT_RATE||0,[0,4.8,4.8,9.6,9.6,14.4,14.4][level]);
        assert.equal(req.extraStats.ENERGY_REGEN,60);
        assert.equal(req.combatExtraStats.ROXY_ATK_CAP,480+level*80);
        assert.equal(req.combatExtraStats.ROXY_IMPACT_CAP,[40,46,52,58,64,70,76.8][level]);
        assert.equal(req.combatExtraStats.CRIT_RATE,24+(rank-1)*2);
        assert.equal(req.combatExtraStats.OTHER_SQUAD_DMG_BONUS,20+(rank-1)*3);
        assert.equal(req.combatExtraStats.ELEMENT_DMG,undefined);
      } else {
        assert.equal(req.extraStats.BASE_ANOMALY_MASTERY||0,[0,12,12,24,24,36,36][level]);
        assert.equal(req.extraStats.ANOMALY_MASTERY,30);
        assert.equal(req.extraStats.ANOMALY_PROFICIENCY,118);
        assert.equal(req.combatExtraStats.ANOMALY_PROFICIENCY,43+level*7+90+(rank-1)*9);
        assert.equal(req.combatExtraStats.ANOMALY_CRIT_DMG,[12.5,14.6,16.7,18.8,20.8,22.9,25][level]);
        assert.equal(req.combatExtraStats.FIRE_DMG,40+(rank-1)*4);
        assert.equal(req.combatExtraStats.CRIT_RATE,undefined);
      }
    }
  }
  const noEngine=context.requestForMultiPlan({characterName:name,ui:{coreLevel:'0'},request:{}},new Set());
  assert.equal(noEngine.extraStats.BASE_ATK,undefined);
  assert.equal(noEngine.combatExtraStats.CRIT_RATE,undefined);
  const assets=fs.readFileSync('web/assets/asset-map.js','utf8');
  const match=assets.match(new RegExp(`'${name}': '(/assets/agents/agent-\\d+\\.png)'`));
  assert.ok(match && fs.existsSync('web'+match[1]));
}
assert.ok(context.statMapText({ANOMALY_CRIT_RATE:100}).includes('100%'));
assert.ok(context.resultStatsHtml({combatStats:{ROXY_ATK_CAP:960,FINAL_IMPACT:176.8}}).includes('实战冲击力'));
assert.ok(!context.resultStatsHtml({combatStats:{ROXY_ATK_CAP:960}}).includes('ROXY_ATK_CAP'));
console.log('NEW_AGENTS_OK: both agents, core 0–F × engine 1–5, saved-plan refresh, no-engine, avatars and reference labels');
