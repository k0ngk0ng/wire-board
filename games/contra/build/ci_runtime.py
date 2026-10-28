"""Preserve a verified runtime for native linking before CDN payload trimming."""
import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
from delivery import delivery_entries
from identity import sha, verify_runtime_file
from media_archive import inside

ROOT=Path(__file__).resolve().parents[3]

def prepare_runtime(root,dist,out):
    dist=inside(root,dist);out=inside(root,out)
    receipt=json.loads((dist/'contra-delivery.json').read_text())
    identity=receipt['runtimeBuildID']
    if not re.fullmatch('[a-f0-9]{64}',identity) or receipt['prefix']!=f'contra/runtime-{identity}/':
        raise ValueError('invalid immutable runtime prefix')
    origin=dist/receipt['prefix']
    if origin.is_symlink() or origin.parent.is_symlink():raise ValueError('runtime directory symlink')
    manifest,current,_=verify_runtime_file(root,origin/'manifest.json')
    if current!=identity or manifest['rulesBuildID']!=receipt['rulesBuildID']:
        raise ValueError('native and delivery runtime identities differ')
    audio=json.loads((origin/manifest['audioCatalog']['url'].lstrip('/')).read_bytes())
    expected=delivery_entries(manifest,audio)
    raw=(origin/'manifest.json').read_bytes()
    expected['manifest.json']={'bytes':len(raw),'sha256':sha(raw),'mime':'application/json','requiresRange':False}
    if receipt['files']!=expected:raise ValueError('delivery receipt differs from runtime manifest')
    actual=set()
    for path in origin.rglob('*'):
        if path.is_symlink():raise ValueError('runtime asset symlink')
        if path.is_file():actual.add(path.relative_to(origin).as_posix())
    if actual!=set(expected):raise ValueError('unexpected or missing runtime payload')
    for name,item in expected.items():
        body=(origin/name).read_bytes()
        if len(body)!=item['bytes'] or sha(body)!=item['sha256']:
            raise ValueError('runtime payload differs: '+name)
    if out.exists():raise ValueError('use a new preserved runtime directory')
    shutil.copytree(origin,out)
    verify_runtime_file(root,out/'manifest.json')
    return {'runtimeBuildID':identity,'rulesBuildID':manifest['rulesBuildID'],
            'manifestSHA256':sha(raw),'files':expected,'passed':True}

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--dist',type=Path,default=ROOT/'web/dist')
    p.add_argument('--out',type=Path,required=True)
    args=p.parse_args()
    source=os.environ.get('GITHUB_SHA')
    if source:
        head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip()
        if not re.fullmatch('[a-f0-9]{40}',source) or source!=head:raise ValueError('CI checkout and source SHA differ')
    result=prepare_runtime(ROOT,args.dist,args.out)
    result['sourceSHA']=source
    (args.out/'ci-origin.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps({'passed':True,'runtimeBuildID':result['runtimeBuildID'],'out':str(args.out)}))

if __name__=='__main__':main()
