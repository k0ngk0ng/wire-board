"""Bind the independently audited late-stage visual recipes and original art."""
import gzip,hashlib,json,os,subprocess
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];E=ROOT/'docs/research/nes-contra/evidence'
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def main():
 fixture=E/'bot-future-sprites-original-art.json.gz';src=json.loads(gzip.decompress(fixture.read_bytes()));paths=[ROOT/p for p in src['sourceHashes']]
 paths += [ROOT/'games/contra/engine'/p for p in ['bot_vision_future_sprites.go','bot_vision_future_sprites_test.go','bot_visual_templates.go','bot_visual_frame.go','bot_visual_frame_pixels.go','bot_vision.go','presentation_sprites.go']]
 paths += [fixture,Path(__file__).resolve(),ROOT/'scripts/contra/bot_future_sprites_reference.py'];before={str(p.relative_to(ROOT)):sha(p)for p in paths}
 tmp=ROOT/'.local/contra-future-sprite-check/tmp';tmp.mkdir(parents=True,exist_ok=True);env={**os.environ,'GOCACHE':str(ROOT/'.local/go-build'),'GOMODCACHE':str(ROOT/'.local/go-mod'),'GOTMPDIR':str(tmp),'TMPDIR':str(tmp),'GOTOOLCHAIN':'local'}
 command=['go','test','-race','./games/contra/engine','-run','^TestBotFutureSprite','-count=1','-json'];log=E/'bot-future-sprites-final-race.jsonl'
 with log.open('w')as f:r=subprocess.run(command,cwd=ROOT,env=env,stdout=f,stderr=subprocess.STDOUT)
 after={str(p.relative_to(ROOT)):sha(p)for p in paths};changed=[p for p in before if before[p]!=after[p]];tests=[d['Test']for d in map(json.loads,log.read_text().splitlines())if d.get('Action')=='pass'and'Test'in d]
 result={'passed':r.returncode==0 and not changed,'command':command,'exitCode':r.returncode,'sourceChangedDuringCheck':changed,'sourceHashes':before,'logSHA256':sha(log),'recipeCount':73,'originalArtCases':73,'tests':tests,'scope':'Static visible-art knowledge and alias rejection only. Does not certify the all-stage strategy, current-stage filtering, or live engine state.'};(E/'bot-future-sprites-verification.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({'passed':result['passed'],'tests':len(tests),'changed':changed}));tmp.rmdir();tmp.parent.rmdir();raise SystemExit(r.returncode or bool(changed))
if __name__=='__main__':main()
