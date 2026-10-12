// Failure cases specified before implementation: main checks rerun on a tag;
// mismatched SHA or workflow supplies an artifact; failed/cancelled binary job;
// expired/missing artifact; known failed checks; annotated tag points elsewhere;
// published release overwritten; a main build is not ready when a tag arrives.
const test = require('node:test');
const assert = require('node:assert/strict');
const {resolveRelease} = require('./release-source.cjs');
const SHA='a'.repeat(40), OTHER='b'.repeat(40);
function fixture(options={}) {
  const runs=options.runs || [{id:11,head_sha:SHA,head_branch:'main',event:'push',path:'.github/workflows/build.yml',status:'in_progress',conclusion:null}];
  const releases=options.releases || [];
  let polls=0;
  const actions={
    listWorkflowRuns:async()=>({data:{workflow_runs:++polls===1 && options.delay?[]:runs}}),
    getWorkflowRun:async({run_id})=>({data:runs.find(r=>r.id===run_id)}),
    listJobsForWorkflowRun:()=>{}, listWorkflowRunArtifacts:()=>{},
  };
  const repos={
    getCommit:async()=>({data:{sha:options.tagSHA||SHA}}),
    listReleases:()=>{},
    createRelease:async(args)=>{releases.push({...args,id:77,tag_name:args.tag_name});return {data:releases.at(-1)}}
  };
  const github={rest:{actions,repos},paginate:async(method)=>{
    if(method===actions.listJobsForWorkflowRun)return options.jobs||[{name:'Test and Linux amd64 binary',conclusion:'success',status:'completed'}];
    if(method===actions.listWorkflowRunArtifacts)return options.artifacts||[{name:'wire-board-linux-amd64',expired:false,size_in_bytes:100}];
    if(method===repos.listReleases)return releases;
    throw Error('unexpected API');
  }};
  const context={repo:{owner:'owner',repo:'repo'},eventName:options.manual?'workflow_dispatch':'push',ref:'refs/tags/v1.8.0',sha:SHA};
  return {github,context,releases,sleep:async()=>{},attempts:2,info:()=>{},inputs:options.manual?{source_run:'11',source_sha:SHA,version:'v1.8.0'}:{}};
}
test('tag selects exact main binary while other checks run, without compiling',async()=>{
 const f=fixture();const r=await resolveRelease(f);assert.deepEqual(r,{run:'11',sha:SHA,version:'v1.8.0'});assert.equal(f.releases[0].draft,true);
});
test('waits for a main build created shortly after a tag',async()=>{
 assert.equal((await resolveRelease(fixture({delay:true}))).run,'11');
});
for(const [name,options] of Object.entries({
 'wrong commit':{runs:[{id:11,head_sha:OTHER,head_branch:'main',event:'push',path:'.github/workflows/build.yml'}]},
 'foreign workflow':{runs:[{id:11,head_sha:SHA,head_branch:'main',event:'push',path:'.github/workflows/other.yml'}]},
 'pull request':{runs:[{id:11,head_sha:SHA,head_branch:'main',event:'pull_request',path:'.github/workflows/build.yml'}]},
 'automatic tag artifact':{runs:[{id:11,head_sha:SHA,head_branch:'v1.8.0',event:'push',path:'.github/workflows/build.yml'}]},
 'failed binary':{jobs:[{name:'Test and Linux amd64 binary',conclusion:'failure'}]},
 'cancelled binary':{jobs:[{name:'Test and Linux amd64 binary',conclusion:'cancelled'}]},
 'known failed rules':{jobs:[{name:'Test and Linux amd64 binary',conclusion:'success'},{name:'rules',conclusion:'failure'}]},
 'expired artifact':{artifacts:[{name:'wire-board-linux-amd64',expired:true,size_in_bytes:100}]},
 'missing artifact':{artifacts:[]},
 'published release':{releases:[{id:2,tag_name:'v1.8.0',draft:false,target_commitish:SHA}]},
 'wrong draft target':{releases:[{id:2,tag_name:'v1.8.0',draft:true,target_commitish:OTHER}]},
})) test('rejects '+name,async()=>{await assert.rejects(resolveRelease(fixture(options)))});
test('manual migration accepts the successful legacy tag build',async()=>{
 const f=fixture({manual:true,runs:[{id:11,head_sha:SHA,head_branch:'v1.8.0',event:'push',path:'.github/workflows/build.yml'}]});assert.equal((await resolveRelease(f)).sha,SHA);
});
test('manual source cannot disagree with the existing tag',async()=>{
 await assert.rejects(resolveRelease(fixture({manual:true,tagSHA:OTHER})));
});
