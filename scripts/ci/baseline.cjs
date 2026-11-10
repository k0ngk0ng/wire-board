// The binary job is the validation boundary. In the scoped workflow it runs
// only after every selected check; earlier workflows used this same job name.
async function baseline(api, currentRun, publishedSHA = '') {
  for await (const run of await api.runs()) {
    if (run.id === Number(currentRun) || run.head_branch !== 'main' || run.event !== 'push' || run.path !== '.github/workflows/build.yml') continue;
    // Legacy workflows published their binary before optional global matrices
    // ended. Only an already published release can certify that legacy case.
    if (run.conclusion !== 'success' && run.head_sha !== publishedSHA) continue;
    const jobs = await api.jobs(run.id);
    const aggregate = jobs.some(j => j.name === 'Selected checks passed' && j.conclusion === 'success');
    const binary = jobs.some(j => j.name === 'Test and Linux amd64 binary' && j.conclusion === 'success');
    if (!aggregate && !binary) continue;
    const artifacts = await api.artifacts(run.id);
    const artifactName = aggregate ? 'verification-plan' : 'wire-board-linux-amd64';
    if (!artifacts.some(a => a.name === artifactName && !a.expired && a.size_in_bytes > 0)) continue;
    if (await api.ancestor(run.head_sha)) return run.head_sha;
  }
  return null;
}
module.exports = {baseline};
