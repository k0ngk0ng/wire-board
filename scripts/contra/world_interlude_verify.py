"""Bind the continuous interlude acceptance to actual source and oracle hashes."""
import hashlib,json,os,subprocess
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];E=ROOT/'docs/research/nes-contra/evidence';BASE=ROOT/'.local/contra-world-interlude'
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def files():
 return sorted(p for p in (ROOT/'games/contra/engine').rglob('*')if p.is_file()and p.suffix in['.go','.json','.gz'])+[E/'world-interlude-original.json.gz',ROOT/'scripts/contra/world_interlude_capture.py',ROOT/'scripts/contra/world_interlude_verify.py']
def main():
 tmp=BASE/'checks/tmp';tmp.mkdir(parents=True,exist_ok=True);env={**os.environ,'GOCACHE':str(ROOT/'.local/go-build'),'GOMODCACHE':str(ROOT/'.local/go-mod'),'GOTMPDIR':str(tmp),'TMPDIR':str(tmp),'GOTOOLCHAIN':'local'}
 before={str(p.relative_to(ROOT)):sha(p)for p in files()};command=['go','test','-race','./games/contra/engine','-run','^(TestOriginalContinuousInterludeWorld|TestCampaignScene)','-count=1','-json']
 log=E/'world-interlude-final-race.jsonl'
 with log.open('w')as f:r=subprocess.run(command,cwd=ROOT,env=env,stdout=f,stderr=subprocess.STDOUT)
 after={str(p.relative_to(ROOT)):sha(p)for p in files()};changed=sorted(k for k in before.keys()|after.keys()if before.get(k)!=after.get(k))
 runs=[]
 for name in ['world-interlude-v1','world-interlude-v2']:
  d=BASE/'oracle/runs'/name;runs.append({'name':name,'RAMSHA256':sha(d/'ram.bin'),'worldRAMSHA256':sha(d/'world-frames.bin'),'worldPPUSHA256':sha(d/'world-ppu.bin'),'inputSHA256':sha(d/'inputs.csv')})
 for key in ['RAMSHA256','worldRAMSHA256','worldPPUSHA256','inputSHA256']:assert runs[0][key]==runs[1][key]
 manifest={'passed':r.returncode==0 and not changed,'command':command,'exitCode':r.returncode,'sourceChangedDuringCheck':changed,'log':str(log.relative_to(ROOT)),'logSHA256':sha(log),'files':before,'originalRuns':runs,'scope':'7997 continuous original World calls from the established reserve30 controller route: genuine original Boss defeat, 701 interlude, reset, second-stage 669 intro/32 entrance/120 battle. RAM, PPU, sounds, duplicate ticks and per-tick JSON restoration. No after-state injection; only original pads and declared idle entropy are inputs. Not natural standard-lives eight-stage or browser/network acceptance.'}
 (E/'world-interlude-verification.json').write_text(json.dumps(manifest,indent=2)+'\n');print(json.dumps({'passed':manifest['passed'],'exitCode':r.returncode,'changed':changed,'repeatedSourcesIdentical':True}));raise SystemExit(r.returncode or bool(changed))
if __name__=='__main__':main()
