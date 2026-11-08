// Before host opt-in: the flow is dropped, other games become horizontal, or
// ten seats are omitted. Actual scrolling/visibility is covered in-browser.
import test from 'node:test';
import assert from 'node:assert/strict';
import {build} from 'esbuild';
import {createRequire} from 'node:module';
import {mkdirSync,writeFileSync,readFileSync} from 'node:fs';
import {fileURLToPath} from 'node:url';
import path from 'node:path';
const root=fileURLToPath(new URL('../../',import.meta.url));
const out=path.join(root,'.local/stoneage/verification/player-layout');mkdirSync(out,{recursive:true});
const result=await build({stdin:{contents:`
import React from 'react';import {renderToStaticMarkup} from 'react-dom/server';
import {ModulePlayers} from './src/game-api/host.tsx';
export function render(horizontal){globalThis.__playerFixture={id:'fixture',name:'布局',playerLayout:horizontal?{flow:'horizontal',density:'compact'}:undefined,PlayerSummary:()=>React.createElement('span',null,'生命 100')};const room={id:'r',kind:'fixture',you:0,seats:Array.from({length:10},(_,i)=>({id:'p'+i,name:'玩家'+i,ready:true})),game:{data:{},actors:[0],attention:[0]}};return renderToStaticMarkup(React.createElement(ModulePlayers,{room}));}
`,resolveDir:path.join(root,'web'),loader:'tsx'},bundle:true,platform:'node',format:'cjs',jsx:'automatic',write:false,loader:{'.css':'empty'},plugins:[{name:'fixture-registry',setup(b){b.onResolve({filter:/^\.\.\/profiles$/},()=>({path:'profiles',namespace:'players'}));b.onLoad({filter:/profiles/,namespace:'players'},()=>({contents:'export const PlayerName=({user})=>user.name;',loader:'js'}));b.onResolve({filter:/^\.\/registry$/},a=>a.importer.endsWith('/game-api/host.tsx')?{path:'registry',namespace:'players'}:null);b.onLoad({filter:/registry/,namespace:'players'},()=>({contents:'export const gameModule=()=>globalThis.__playerFixture;export const gameName=()=>"fixture";',loader:'js'}));}}]});
const compiled=path.join(out,'fixture.cjs');writeFileSync(compiled,result.outputFiles[0].contents);const {render}=createRequire(import.meta.url)(compiled);
test('horizontal player strip is explicitly opted into and preserves every seat',()=>{const html=render(true);assert.match(html,/data-player-flow="horizontal"/);assert.equal((html.match(/data-player-seat=/g)||[]).length,10);assert.match(html,/data-player-density="compact"/);writeFileSync(path.join(out,'render.json'),JSON.stringify({passed:true,html},null,2));});
test('modules without horizontal flow retain their normal host layout',()=>{assert.doesNotMatch(render(false),/data-player-flow=/);});
