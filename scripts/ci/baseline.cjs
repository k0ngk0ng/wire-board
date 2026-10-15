// The binary job is the validation boundary. In the scoped workflow it runs
// only after every selected check; earlier workflows used this same job name.
async function baseline(api, currentRun, publishedSHA = '') {
  for await (const run of await api.runs()) {
    if (run.id === Number(currentRun) || run.head_branch !== 'main' || run.event !== 'push' || run.path !== '.github/workflows/build.yml') continue;
    // Legacy workflows published their binary before optional global matrices
    // ended. Only an already published release can certify that legacy case.
    if (run.conclusion !== 'success' && run.head_sha !== publishedSHA) continue;
    const jobs = await api.jobs(run.id);
    if (!jobs.some(j => j.name === 'Test and Linux amd64 binary' && j.conclusion === 'success')) continue;
    const artifacts = await api.artifacts(run.id);
    if (!artifacts.some(a => a.name === 'wire-board-linux-amd64' && !a.expired && a.size_in_bytes > 0)) continue;
    if (await api.ancestor(run.head_sha)) return run.head_sha;
  }
  return null;
}
module.exports = {baseline};
