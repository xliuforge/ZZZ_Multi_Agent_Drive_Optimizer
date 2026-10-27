const fs = require('fs');
const vm = require('vm');
const assert = require('assert/strict');
const path = require('path');
process.chdir(path.resolve(__dirname, '..'));
const html = fs.readFileSync('web/index.html', 'utf8').replaceAll('__SCANNER_AVAILABLE__', 'true');
const script = [...html.matchAll(/<script(?:\s[^>]*)?>([\s\S]*?)<\/script>/g)].map(m => m[1]).find(s => s.includes('const STAT_LABELS'));
const statusNode = {textContent:''};
const timers = [];
const context = vm.createContext({window:{addEventListener(){}},localStorage:{getItem(){return null;}},document:{querySelector(selector){return selector === '#scannerUpdateStatus' ? statusNode : null;}},setTimeout(fn,ms){timers.push({fn,ms});},console});
vm.runInContext(script.slice(0,script.lastIndexOf('(async function init()')),context);
(async()=>{
  for (const state of ['checking','downloading','current','updated','deferred','skipped','error']) {
    timers.length=0;
    context.reply={state,message:'Scanner <b>状态</b>',version:'1.0.49'};
    vm.runInContext('fetchJSON=async()=>reply;',context);
    await context.refreshScannerUpdateStatus();
    assert.equal(statusNode.textContent,context.reply.message);
    assert.equal(timers.length,['checking','downloading'].includes(state)?1:0);
    if(timers.length)assert.equal(timers[0].ms,3000);
  }
  vm.runInContext('fetchJSON=async()=>{throw Error("offline")};',context);
  await context.refreshScannerUpdateStatus();
  assert.ok(statusNode.textContent.includes('本地扫描器'));
  console.log('SCANNER_UPDATE_UI_OK: progress polling, terminal states, plain-text messages and offline fallback');
})().catch(err=>{console.error(err);process.exitCode=1;});
