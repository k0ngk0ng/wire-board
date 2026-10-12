// Resolves a release to an existing, verified binary. No build/test dispatches.
async function resolveRelease({github, context, inputs={}, sleep=(ms)=>new Promise(r=>setTimeout(r,ms)), attempts=20, info=()=>{}}) {
  const manual=context.eventName==='workflow_dispatch';
  const version=manual?inputs.version:context.ref.replace(/^refs\/tags\//,'');
  if(!/^v\d+\.\d+\.\d+$/.test(version||'')) throw Error('Expected a stable vX.Y.Z version tag');
  if(manual && context.ref!==`refs/tags/${version}`) throw Error('Dispatch publication on the version tag, never main');
  if(!manual && (context.eventName!=='push'||context.ref!==`refs/tags/${version}`)) throw Error('Expected a tag push');
  if(manual && (!/^\d+$/.test(inputs.source_run||'')||! /^[0-9a-f]{40}$/.test(inputs.source_sha||''))) throw Error('Invalid manual source inputs');
  const repo=context.repo;
  let tagSHA;
  try { tagSHA=(await github.rest.repos.getCommit({...repo,ref:`refs/tags/${version}`})).data.sha; }
  catch(error) { if(!manual||error.status!==404) throw error; }
  const sha=manual?inputs.source_sha:tagSHA;
  if(!/^[0-9a-f]{40}$/.test(sha||'')||(tagSHA && tagSHA!==sha)) throw Error('Existing version tag does not match the source commit');
  let selected;
  for(let attempt=0;attempt<attempts;attempt++) {
    const runs=manual?[(await github.rest.actions.getWorkflowRun({...repo,run_id:Number(inputs.source_run)})).data]:
      (await github.rest.actions.listWorkflowRuns({...repo,workflow_id:'build.yml',branch:'main',event:'push',head_sha:sha,per_page:100})).data.workflow_runs;
    for(const run of runs) {
      const branchOK=run?.head_branch==='main'||(manual && run?.head_branch===version);
      if(!run||run.head_sha!==sha||!branchOK||run.event!=='push'||run.path!=='.github/workflows/build.yml') {
        if(manual) throw Error('Source must be a matching main build or explicitly selected legacy tag build');
        continue;
      }
      const jobs=await github.paginate(github.rest.actions.listJobsForWorkflowRun,{...repo,run_id:run.id,per_page:100});
      if(jobs.some(j=>['failure','timed_out','action_required'].includes(j.conclusion))) throw Error('Source has a failed check; refusing publication');
      const binary=jobs.find(j=>j.name==='Test and Linux amd64 binary');
      if(binary?.conclusion!=='success') {
        if(manual||run.status==='completed') throw Error('Source binary build did not pass');
        continue;
      }
      const artifacts=await github.paginate(github.rest.actions.listWorkflowRunArtifacts,{...repo,run_id:run.id,per_page:100});
      if(!artifacts.some(a=>a.name==='wire-board-linux-amd64'&&!a.expired&&a.size_in_bytes>0)) throw Error('Verified binary artifact is missing or expired');
      selected=run;break;
    }
    if(selected) break;
    if(attempt+1<attempts) {info(`Waiting for the main binary build of ${sha} (${attempt+1}/${attempts})`);await sleep(20000);}
  }
  if(!selected) throw Error('No matching successful main binary build; push this commit to main and wait for its build');
  const releases=await github.paginate(github.rest.repos.listReleases,{...repo,per_page:100});
  const release=releases.find(r=>r.tag_name===version);
  if(release && (!release.draft||(!tagSHA && release.target_commitish!==sha))) throw Error('Release must be unpublished and target this exact commit');
  // For an existing tag, its resolved SHA is authoritative; GitHub ignores
  // target_commitish for content selection. Supplying an old workflow-changing
  // commit here can unnecessarily require privileges GITHUB_TOKEN cannot hold.
  if(!release) {
    if(!tagSHA) throw Error('Create the matching draft or version tag before manual publication');
    await github.rest.repos.createRelease({...repo,tag_name:version,name:`Wire Board ${version}`,draft:true,generate_release_notes:true});
  }
  return {run:String(selected.id),sha,version};
}
module.exports={resolveRelease};
