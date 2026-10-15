// Authored before baseline selection: cancelled/failed predecessors must not
// hide unverified changes; reject unrelated branches, missing/expired artifacts,
// non-ancestors and current run; API errors must fail instead of skipping tests.
const {test}=require('node:test');
const assert=require('node:assert/strict');
const {baseline}=require('./baseline.cjs');
function fixture(overrides={}) {
 const runs=[{id:3,head_sha:'c'.repeat(40)},{id:2,head_sha:'b'.repeat(40)},{id:1,head_sha:'a'.repeat(40)}].map(r=>({...r,conclusion:r.id===1?'success':'cancelled',event:'push',head_branch:'main',path:'.github/workflows/build.yml'}));
 const api={runs:async()=>runs,jobs:async(id)=>[{name:'Test and Linux amd64 binary',conclusion:id===1?'success':'cancelled'}],artifacts:async()=>[{name:'wire-board-linux-amd64',expired:false,size_in_bytes:10}],ancestor:async()=>true,...overrides};return api;
}
test('uses the last verified binary rather than the previous cancelled push',async()=>assert.equal(await baseline(fixture(),3),'a'.repeat(40)));
test('missing baseline requests bootstrap coverage',async()=>assert.equal(await baseline(fixture({artifacts:async()=>[]}),3),null));
test('expired artifact cannot certify a baseline',async()=>assert.equal(await baseline(fixture({artifacts:async()=>[{name:'wire-board-linux-amd64',expired:true,size_in_bytes:10}]}),3),null));
test('non-ancestor cannot hide changes',async()=>assert.equal(await baseline(fixture({ancestor:async()=>false}),3),null));
test('foreign branch cannot certify a baseline',async()=>assert.equal(await baseline(fixture({runs:async()=>[{id:1,head_branch:'feature',event:'push',path:'.github/workflows/build.yml'}]}),3),null));
test('API errors propagate',async()=>assert.rejects(baseline(fixture({jobs:async()=>{throw Error('unavailable')}}),3),/unavailable/));
