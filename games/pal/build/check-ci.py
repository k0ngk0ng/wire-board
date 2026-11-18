import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import shutil
import subprocess
import sys
import tarfile
import urllib.request


ROOT = Path(__file__).resolve().parents[3]
LOCAL = ROOT / '.local/pal'
PROOF = LOCAL / 'ci'
MANIFEST = ROOT / 'games/pal/assets/source-manifest.json'
NATIVE_OUTPUT = LOCAL / 'dense-battle-candidate/campaign'
PRODUCTION_OUTPUT = LOCAL / 'server-package-ci'
PUBLISHED_OUTPUT = LOCAL / 'ci-published-rule-input/pal'
TOOLCHAINS = {
    'Linux-x86_64': [
        ('WebAssembly/wasi-sdk', 'wasi-sdk-34', 'wasi-sdk-34.0-x86_64-linux', 192383077, 'b761e3a0721dbae9c09a0059e5fdb2bf917d1b4a8a7b430fb3b5aafb0984b2c4'),
        ('WebAssembly/binaryen', 'version_133', 'binaryen-version_133-x86_64-linux', 111200018, '2dc9c7813f5375db93d96ead4b78222fcc3e2677bbb832297af4797782a37489'),
    ],
    'Darwin-arm64': [
        ('WebAssembly/wasi-sdk', 'wasi-sdk-34', 'wasi-sdk-34.0-arm64-macos', 180322002, '9c59398106b417f8f14913380fdf0097a8cc0ff4af9eb3ce0065a859e88d49e9'),
        ('WebAssembly/binaryen', 'version_133', 'binaryen-version_133-arm64-macos', 7810532, 'ad66da82ac13f163e424b1643f16c6dfcccc98b5966296b43e52d3cab04f84a8'),
    ],
}
CHECKS = {
    './games/pal/resources': ['TestArchiveBoundaries', 'Test98TablesRejectIncompleteData', 'TestOriginal98Tables', 'TestOriginal98Decompression', 'TestOriginal98RuleBundle', 'TestOriginal98Pixels', 'TestOriginalVisualExport', 'TestOriginalRNGRuleAndVisualArchives', 'TestOriginalRNGAllFramesAndSavedPixels'],
    './games/pal/engine': ['TestNativeOriginalOpeningSaveRestore', 'TestNativeCampaignReachesOriginalInn', 'TestNativeTimedBatchMatchesOriginalSteps', 'TestOriginalOwnedContinuation', 'TestOriginalFieldMenusSaveAndSystemBoundary', 'TestOriginalFieldPredictionProjection', 'TestOriginalLocalReplayCheckpointAndNativeOperations'],
    './games/pal/tests': ['TestOriginalHiddenSearchEventInventoryPreserved', 'TestOriginalYuhangHiddenItemsSearchSaveAndNoDuplication', 'TestOriginalYuhangVillageHiddenItemsAndHerbsSave', 'TestOriginalCampaignHammerAndIslandLandingSave'],
    './internal/server': ['TestPALControllerProfilesHTTP', 'TestPALOriginalSoloPrivateSaveRestartHTTP', 'TestPALOriginalManualCheckpointReloadAndOldPageHTTP', 'TestPALAuthorityArchiveRestartAndManualCheckpointHTTP', 'TestPALLocalFirstNativeQueueReplayPrivacyAndResume', 'TestPALWorkerContentPolicy'],
}


def toolchains():
    return {target: [{'file': name + '.tar.gz', 'bytes': size, 'sha256': sha, 'url': f'https://github.com/{repo}/releases/download/{tag}/{name}.tar.gz'} for repo, tag, name, size, sha in tools] for target, tools in TOOLCHAINS.items()}


def sha(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def checked_download(folder, item):
    archive = folder / item['file']
    temporary = archive.with_suffix('.partial')
    if not archive.exists():
        try:
            request = urllib.request.Request(item['url'], headers={'User-Agent': 'wire-board-pal-ci'})
            with urllib.request.urlopen(request, timeout=60) as response, temporary.open('xb') as output:
                written = 0
                while data := response.read(1024 * 1024):
                    written += len(data)
                    if written > item['bytes']:
                        raise ValueError('PAL CI tool exceeds its recorded length')
                    output.write(data)
            if temporary.stat().st_size != item['bytes'] or sha(temporary) != item['sha256']:
                raise ValueError('PAL CI tool archive differs: ' + item['file'])
            temporary.rename(archive)
        finally:
            temporary.unlink(missing_ok=True)
    if archive.is_symlink() or archive.stat().st_size != item['bytes'] or sha(archive) != item['sha256']:
        raise ValueError('PAL CI tool archive differs: ' + item['file'])
    return archive


def prepare_tools(target):
    folder = LOCAL / 'ci-toolchains'
    downloads = folder / 'downloads'
    downloads.mkdir(parents=True, exist_ok=True)
    tools = toolchains()[target]
    extracted = folder / 'installed'
    extracted.mkdir()
    for item in tools:
        archive = checked_download(downloads, item)
        with tarfile.open(archive, 'r:gz') as tar:
            tar.extractall(extracted, filter='data')
    sdk, = extracted.glob('wasi-sdk-*/bin/clang')
    binaryen = extracted / 'binaryen-version_133/bin/wasm-opt'
    assert sdk.is_file() and binaryen.is_file()
    return sdk, binaryen


def command(args, env, label):
    print('PAL CI:', label, flush=True)
    path = PROOF / (label + '.log')
    diagnostics = PROOF / (label + '.stderr.log')
    with path.open('w') as log, diagnostics.open('w') as errors:
        status = subprocess.run(args, cwd=ROOT, env=env, stdout=log, stderr=errors, check=False).returncode
    if status != 0:
        for stream in [path, diagnostics]:
            print('\n'.join(stream.read_text(errors='replace').splitlines()[-35:]), file=sys.stderr)
        raise ValueError(f'{label} failed; see {path.relative_to(ROOT)}')
    return path


def passed_events(log, names):
    results = {}
    for line in log.read_text(errors='replace').splitlines():
        try:
            row = json.loads(line)
        except ValueError:
            continue
        if row.get('Test') in names and row.get('Action') in ('pass','fail','skip'):
            results[row['Test']] = row['Action']
    if results != dict.fromkeys(names, 'pass'):
        raise ValueError('PAL CI required tests did not all run and pass: ' + str(results))
    return sorted(results)


def plan():
    return {'profile':'original-98','historicalArtifactMode':'local-research','nativeOutputDirectory':NATIVE_OUTPUT.relative_to(ROOT).as_posix(),'validateProductionBundle':True,'requiredRuntime':'verified-server-publication','preparedOriginalCount':len(json.loads(MANIFEST.read_bytes())['files']),'toolchains':toolchains(),'checks':[{'package':pkg,'tests':tests} for pkg,tests in CHECKS.items()]}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--plan', action='store_true')
    args = parser.parse_args()
    specification = plan()
    if args.plan:
        print(json.dumps(specification,indent=2))
        return
    required_path = LOCAL / 'native' / 'rules.wasm'
    if any(path.exists() for path in [PROOF,required_path,NATIVE_OUTPUT,PRODUCTION_OUTPUT,PUBLISHED_OUTPUT,LOCAL/'authority-prepared']):
        parser.error('use a clean checkout so PAL CI does not overwrite local cores or evidence')
    (ROOT/'.local/tmp').mkdir(parents=True,exist_ok=True)
    PROOF.mkdir(parents=True)
    env = {k:v for k,v in os.environ.items() if not k.startswith('PAL_')}
    env.update(TMPDIR=str(ROOT/'.local/tmp'), GOTMPDIR=str(ROOT/'.local/tmp'), PAL_FRESH_CI='1', PAL_AUTHORITY_HTTP='1', PAL_CHECKPOINT_E2E='1', GOTELEMETRY='off')
    report = {'passed':False,'profile':'original-98','toolchainPlatform':platform.system()+'-'+platform.machine(),'historicalArtifactMode':specification['historicalArtifactMode'],'checks':[]}
    receipt = PROOF / 'result.json'
    try:
        command([sys.executable,'-B','scripts/ci/pal-release-e2e.py'],env,'ci-output-regression')
        sdk, optimizer = prepare_tools(report['toolchainPlatform'])
        env.update(PAL_WASI_SDK=str(sdk), PAL_WASM_OPT=str(optimizer))
        command([sys.executable,'-B','games/pal/assets/fetch.py'],env,'source-data')
        command([sys.executable,'-B','games/pal/build/native.py','--out',str(NATIVE_OUTPUT)],env,'native-build')
        command([sys.executable,'-B','games/pal/build/authority.py'],env,'authority-build')
        report['sourceCoreSHA256'] = sha(NATIVE_OUTPUT/'rules.wasm')
        report['authoritySHA256'] = sha(LOCAL/'authority-prepared/authority.zip')
        report['sourceManifestSHA256'] = sha(MANIFEST)
        rebuilt_log = command([sys.executable,'-B','games/pal/build/server-package.py','--core',str(NATIVE_OUTPUT/'rules.wasm'),'--out',str(PRODUCTION_OUTPUT)],env,'source-bundle')
        rebuilt = json.loads(rebuilt_log.read_bytes())
        if not rebuilt['passed'] or rebuilt['identity']['buildID'] != report['sourceCoreSHA256']:
            raise ValueError('PAL CI source core failed to open its own rebuilt rule bundle')
        production_log = command([sys.executable,'-B','games/pal/build/prepare-server.py','--out',str(PUBLISHED_OUTPUT)],env,'production-bundle')
        distribution = json.loads(production_log.read_bytes())
        production = distribution['runtime']
        published = json.loads((ROOT/'games/pal/assets/server-release.json').read_bytes())
        pin = published['catalogs'][published['current']]
        core = PUBLISHED_OUTPUT/'cores'/(pin['currentCore']+'.wasm')
        if not distribution['passed'] or distribution['assetID'] != published['current'] or not production['passed'] or production['identity']['buildID'] != pin['currentCore'] or production['identity']['resourceID'] != rebuilt['identity']['resourceID'] or production['identity']['profile'] != 'original-98' or production['identity']['purpose'] != 'campaign' or not all(production[key] for key in ['singlePlayer','supportsSave','supportsCheckpoint']):
            raise ValueError('PAL CI did not select the source-bound production publication')
        if sha(PUBLISHED_OUTPUT/'authority.zip') != report['authoritySHA256'] or sha(core) != pin['currentCore']:
            raise ValueError('PAL CI production package differs from the rebuilt rules or published core')
        required_path.parent.mkdir(parents=True,exist_ok=True)
        shutil.copyfile(core,required_path)
        report['nativeSHA256'] = sha(required_path)
        if report['nativeSHA256'] != pin['currentCore']:
            raise ValueError('PAL CI test core differs from the published original campaign core')
        report['serverPublicationID'] = published['current']
        report['productionBundle'] = production
        report['nativeOutputDirectory'] = NATIVE_OUTPUT.relative_to(ROOT).as_posix()
        for package,names in CHECKS.items():
            label = package.rsplit('/',1)[-1]
            if label == 'server':
                label = 'platform-http'
            pattern = '^(?:'+'|'.join(re.escape(name) for name in names)+')$'
            log = command(['go','test','-json','-count=1','-timeout=8m',package,'-run',pattern],env,label)
            verified = passed_events(log,names)
            report['checks'].append({'package':package,'passed':verified,'logSHA256':sha(log)})
        for name, evidence in [('local-native-e2e','native-browser-replay'),('local-simulation-e2e','local-simulation-e2e'),('field-correction.test','field-correction')]:
            log = command(['node', 'games/pal/tests/'+name+'.mjs'],env,name)
            shutil.copyfile(LOCAL/'verification/local-first'/(evidence+'.json'), PROOF/(evidence+'.json'))
            report['checks'].append({'package':'browser-native','passed':{name:'pass'},'logSHA256':sha(log)})
        report['passed'] = True
    finally:
        receipt.write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({'passed':report['passed'],'tests':sum(len(c['passed']) for c in report['checks']),'nativeSHA256':report['nativeSHA256']}))


if __name__ == '__main__':
    main()
