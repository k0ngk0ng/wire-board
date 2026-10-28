"""Run source-bound provider/catalog/template validation; no game-state oracle injection."""
from pathlib import Path
import hashlib,json,os,subprocess
ROOT=Path(__file__).resolve().parents[2];E=ROOT/'docs/research/nes-contra/evidence'
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def files():
 p=ROOT/'games/contra/engine';out=sorted(p.glob('bot_visual*.go'));out += [p/n for n in ['presentation_sprites.go','presentation_sprites_data.json','world_presentation.go','world_display.go','bot_vision.go','frame.go']];out += [p/'testdata/bot-visual-frame-raster.json.gz',ROOT/'games/contra/tests/testdata/background-original.json.gz'];out += [ROOT/'games/contra/web'/n for n in ['runtime-renderer.ts','background.ts','sprites.ts']];return out

def main():
 before={str(p.relative_to(ROOT)):sha(p)for p in files()};tmp=ROOT/'.local/contra-bot-visual-check/tmp';tmp.mkdir(parents=True,exist_ok=True)
 env={**os.environ,'GOCACHE':str(ROOT/'.local/go-build'),'GOMODCACHE':str(ROOT/'.local/go-mod'),'GOTMPDIR':str(tmp),'TMPDIR':str(tmp),'GOTOOLCHAIN':'local'}
 command=['go','test','-race','./games/contra/engine','-run','^(TestBotVisualFrame|TestVerifiedBotVisualCatalog|TestBotVisualSpriteTemplates)','-count=1','-json'];log=E/'bot-visual-acceptance.jsonl'
 with log.open('w')as f:r=subprocess.run(command,cwd=ROOT,env=env,stdout=f,stderr=subprocess.STDOUT)
 after={str(p.relative_to(ROOT)):sha(p)for p in files()};changed=sorted(k for k in before.keys()|after.keys()if before.get(k)!=after.get(k))
 tests=[x['Test']for x in map(json.loads,log.read_text().splitlines())if x.get('Action')=='pass'and'Test'in x]
 rootcheck=ROOT/'.local/contra-world/checks/visual-provider-root-race.log';benchmark=ROOT/'.local/contra-world/checks/visual-benchmark.log'
 manifest={'passed':r.returncode==0 and not changed,'command':command,'exitCode':r.returncode,'sourceChangedDuringCheck':changed,'sourceHashes':before,'logSHA256':sha(log),'tests':tests,'originalPixelFrames':20,'originalPixels':1228800,'templateCases':36,'scope':'Final visible pixels, immutable lazy asset catalog and mechanical static-template generation only; not strategy fairness, whole-campaign or server capacity acceptance.','independentRootCheck':{'path':str(rootcheck.relative_to(ROOT)),'sha256':sha(rootcheck)}if rootcheck.exists()else None,'initialBenchmark':{'path':str(benchmark.relative_to(ROOT)),'sha256':sha(benchmark),'limitation':'One M4 sample, not a server capacity result.'}if benchmark.exists()else None}
 (E/'bot-visual-verification.json').write_text(json.dumps(manifest,indent=2)+'\n');print(json.dumps({'passed':manifest['passed'],'tests':len(tests),'changed':changed}));tmp.rmdir();tmp.parent.rmdir();raise SystemExit(r.returncode or bool(changed))
if __name__=='__main__':main()
