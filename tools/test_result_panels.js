const fs = require('fs');
const vm = require('vm');
const assert = require('assert/strict');
const path = require('path');
process.chdir(path.resolve(__dirname, '..'));
const html = fs.readFileSync('web/index.html','utf8').replaceAll('__SCANNER_AVAILABLE__','false');
const script = [...html.matchAll(/<script(?:\s[^>]*)?>([\s\S]*?)<\/script>/g)].map(m=>m[1]).find(s=>s.includes('const STAT_LABELS'));
const nodes=new Map();
const node=key=>{if(!nodes.has(key))nodes.set(key,{value:'',options:[],selectedOptions:[]});return nodes.get(key);};
const context=vm.createContext({console,window:{addEventListener(){}},localStorage:{getItem(){return null;}},document:{querySelector:node,querySelectorAll(){return [];}}});
vm.runInContext(script.slice(0,script.lastIndexOf('(async function init()')),context);
context.characters=JSON.parse(fs.readFileSync('web/data/characters.json','utf8'));
context.engines=JSON.parse(fs.readFileSync('web/data/wengines.json','utf8'));
vm.runInContext('CHARACTERS=characters;WENGINES=engines;state={discs:[],setEffects:[]};',context);
const base={rank:1,finalHp:12897,finalAttack:1900,finalDefense:629,panelImpact:95,panelCritRate:70,panelCritDmg:150,panelAnomalyMastery:90,panelEnergyRegen:1.2,panelSheerForce:1859,panelAdrenalineRegen:2,sheerForce:9999,stats:{ANOMALY_PROFICIENCY:89,PEN_RATIO:0,PEN_FLAT:0},combatStats:{CRIT_RATE:99,CRIT_DMG:999},setSummary:{},discs:[]};
const rowsFor=(r,role)=>JSON.parse(JSON.stringify(context.resultPanelRows(r,role)));
const rupture={...base,roleSystem:'RUPTURE'};
const rows=rowsFor(rupture,'ATTACK'); // Result snapshot must win over the currently selected role.
assert.deepEqual(rows.map(r=>r[0]),['生命值','攻击力','防御力','冲击力','暴击率','暴击伤害','异常掌控','异常精通','贯穿力','闪能自动累积']);
assert.equal(rows.find(r=>r[0]==='贯穿力')[1],1859);
assert.equal(rows.find(r=>r[0]==='闪能自动累积')[1],2);
assert.equal(rows.find(r=>r[0]==='暴击率')[1],70);
const zero=rowsFor({...rupture,panelAdrenalineRegen:0});
assert.equal(zero.find(r=>r[0]==='闪能自动累积')[1],0);
const old={...rupture};delete old.panelSheerForce;delete old.panelAdrenalineRegen;
assert.ok(context.allResultAttributesHtml(old).includes('—'));
assert.ok(!context.allResultAttributesHtml(old).includes('9999'));
const sharp={...base,roleSystem:'ARMORER',stats:{...base.stats,LACERATION_DMG:150,SHARPNESS_REGEN:1.5}};
const sharpRows=rowsFor(sharp);
assert.ok(sharpRows.some(r=>r[0]==='锐暴伤害'&&r[1]===150));
assert.ok(sharpRows.some(r=>r[0]==='锐能自动累积'&&r[1]===1.5));
assert.ok(!sharpRows.some(r=>['攻击力','贯穿力','能量自动回复','闪能自动累积'].includes(r[0])));
for(const c of context.characters){
 const req=context.requestForMultiPlan({characterName:c.name,ui:{coreLevel:'F'},request:{}},new Set());
 const r={...base,roleSystem:c.role,stats:req.extraStats};
 if(c.role==='RUPTURE'){
  assert.equal(req.extraStats.ADRENALINE_REGEN??0,c.name==='真斗'?0:2);
 }else if(c.role!=='ARMORER'){
  const names=rowsFor(r).map(x=>x[0]);
  assert.ok(names.includes('能量自动回复')&&!names.includes('贯穿力')&&!names.includes('锐暴伤害'));
 }
}
const withDamage=rowsFor({...base,stats:{...base.stats,ICE_DMG:30}});
assert.ok(withDamage.some(r=>r[0]==='冰属性伤害'&&r[1]===30));
// Exercise the actual templates for single results, near misses and assigned builds.
vm.runInContext("floorSummary=()=>'';occupiedPolicySummary=()=>'';selectedMainLockSummary=()=>'';lastOptimizeRole='RUPTURE';",context);
context.renderResults({results:[rupture]});
assert.ok(node('#resultsContainer').innerHTML.includes('配装后面板'));
assert.ok(node('#resultsContainer').innerHTML.includes('闪能自动累积'));
context.renderResults({results:[],nearMisses:[rupture]});
assert.ok(node('#resultsContainer').innerHTML.includes('配装后面板'));
const multi=context.assignedBuildHtml({request:{roleSystem:'RUPTURE'},ui:{}},rupture);
assert.ok(multi.includes('配装后面板')&&multi.includes('闪能自动累积'));
assert.ok(multi.includes(context.allResultAttributesHtml(rupture)));
const css=[...html.matchAll(/<style>([\s\S]*?)<\/style>/g)].map(m=>m[1]).join('\n');
fs.mkdirSync('.tmp',{recursive:true});
fs.writeFileSync('.tmp/result-panels-preview.html',`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><style>${css}body{padding:24px;max-width:1100px;margin:auto}h2{margin:18px 0 4px}</style><body><h2>星徽·比利 · 配装示例</h2>${context.resultPanelHtml(rupture)}<h2>克拉蕾 · 配装示例</h2>${context.resultPanelHtml(sharp)}<h2>普通职业 · 配装示例</h2>${context.resultPanelHtml({...base,roleSystem:'ATTACK'})}</body></html>`);
console.log('RESULT_PANELS_OK: 59 roles, resource zero, panel/combat separation, single/near-miss/multi rendering');
