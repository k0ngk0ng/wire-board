// Independent comparison against the preserved original TypeScript generator.
// All helper binaries/cache/temp files stay within the sourced repository env.
import fs from 'node:fs';
import path from 'node:path';
import {spawnSync} from 'node:child_process';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import ts from '../../web/node_modules/typescript/lib/typescript.js';
const root=path.resolve(import.meta.dirname,'../..'),out=path.join(root,'.local/pairs');fs.mkdirSync(out,{recursive:true});
const source=fs.readFileSync(path.join(root,'games/pairs/tests/source-engine.ts'),'utf8');
const compiled=ts.transpile(source,{target:ts.ScriptTarget.ES2022,module:ts.ModuleKind.ES2022});
const original=await import('data:text/javascript;base64,'+Buffer.from(compiled).toString('base64'));
const helper=path.join(out,'source-parity.go');
fs.writeFileSync(helper,`package main
import("encoding/json";"os";p "github.com/k0ngk0ng/wire-board/games/pairs/server")
func main(){out:=[]any{};for n:=1;n<=70;n++{for seed:=uint32(1);seed<=3;seed++{level:=p.GenerateLevel(n,seed*7919);shuffled,relaxed:=p.Reshuffle(level.Board,seed);out=append(out,map[string]any{"n":n,"seed":seed,"board":level.Board,"shuffled":shuffled,"relaxed":relaxed})}};json.NewEncoder(os.Stdout).Encode(out)}
`);
const r=spawnSync('go',['run',helper],{cwd:root,encoding:'utf8',maxBuffer:16*1024*1024});if(r.status!==0)throw Error(r.stderr||String(r.error));
const cases=JSON.parse(r.stdout);
for(const c of cases){const level=original.generateLevel(c.n,c.seed*7919);assert.deepEqual(c.board,level.board,`map parity ${c.n}/${c.seed}`);assert.ok(original.verifySolution(c.board,level.solution));const shuffled=original.reshuffle(level.board,c.seed);assert.deepEqual(c.shuffled,shuffled.board);assert.equal(c.relaxed,shuffled.relaxed);}
fs.writeFileSync(path.join(out,'source-parity.json'),JSON.stringify({passed:true,cases:cases.length,sourceSHA256:createHash('sha256').update(source).digest('hex'),scope:'All 70 source levels, three seeds each; exact generated board, original complete removal solution and reshuffle parity'},null,2)+'\n');console.log('Verified original generator and reshuffle parity:',cases.length,'cases');
