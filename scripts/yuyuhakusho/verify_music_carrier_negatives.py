"""Mutation rejection checks for the frozen native-carrier E2E harness."""
from pathlib import Path
import argparse,hashlib,json,os,shutil,subprocess
from reference import ROOT,sha


def main(out,fixture):
    base=json.loads((fixture/'manifest.json').read_text());results=[]
    for mode in ('sample-gap','changed-pcm','changed-fragment','unsupported-mime'):
        target=out/(mode+'-fixture');target.mkdir(parents=True,exist_ok=True)
        names=['manifest.json','init.mp4','roundtrip.pcm']+[fragment['file']for fragment in base['fragments']]
        for name in names:shutil.copyfile(fixture/name,target/name)
        manifest=json.loads((target/'manifest.json').read_text())
        if mode=='sample-gap':manifest['fragments'][1]['start_sample']+=1
        elif mode=='changed-pcm':
            p=target/'roundtrip.pcm';raw=bytearray(p.read_bytes());raw[1000]^=1;p.write_bytes(raw)
        elif mode=='changed-fragment':
            p=target/manifest['fragments'][1]['file'];raw=bytearray(p.read_bytes());raw[-1]^=1;p.write_bytes(raw)
        else:manifest['mime']='audio/mp4; codecs="alac"'
        (target/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
        destination=out/mode;destination.mkdir(exist_ok=True)
        command=['python3','scripts/yuyuhakusho/check_music_native_carrier.py','--out',str(destination),'--fixture',str(target)]
        env=dict(os.environ,PYTHONDONTWRITEBYTECODE='1')
        result=subprocess.run(command,cwd=ROOT,env=env,text=True,capture_output=True)
        log=destination/'rejection.txt';log.write_text(result.stdout+result.stderr)
        assert result.returncode!=0,(mode,'mutation accepted')
        if mode=='unsupported-mime':
            error=json.loads((destination/'failure.json').read_text())
            assert any('native MSE codec unsupported'in item for item in error['partial']['errors'])
        else:assert 'AssertionError'in result.stderr
        results.append({'mode':mode,'exit_code':result.returncode,'log':str(log.relative_to(ROOT)),'sha256':sha(log.read_bytes()),'rejected':True})
    report={'pass':True,'scope':'Frozen carrier harness rejects source/packet metadata corruption and unsupported native codec',
            'checks':results,'harness_sha256':sha((ROOT/'scripts/yuyuhakusho/check_music_native_carrier.py').read_bytes()),
            'source_sha256':sha(Path(__file__).read_bytes()),'limits':['Three integrity mutations reject before browser playback; unsupported MIME is exercised in the actual native browser']}
    (out/'validation.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))

if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',required=True,type=Path);p.add_argument('--fixture',required=True,type=Path);a=p.parse_args();out=a.out.resolve();fixture=a.fixture.resolve();assert out.is_relative_to(ROOT/'.local')and fixture.is_relative_to(ROOT/'.local');out.mkdir(parents=True,exist_ok=True);main(out,fixture)
