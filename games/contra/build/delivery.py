"""Stage exact verified runtime files for review; never upload or rewrite them."""
import argparse
import json
import shutil
from pathlib import Path
from identity import verify_runtime_file, sha

ROOT=Path(__file__).resolve().parents[3]
MIME={'.wasm':'application/wasm','.js':'text/javascript','.json':'application/json',
      '.idx':'application/octet-stream','.rgb':'application/octet-stream','.wav':'audio/wav'}

def delivery_entries(manifest,audio):
    assets=[manifest[k] for k in ['wasm','loader','audioCatalog','rgb','resources']]
    assets+=list(manifest['pages'].values())+list(manifest['stages'].values())+list(audio['paths'].values())
    rows={}
    for asset in assets:
        name=asset['url'].lstrip('/')
        path=Path(name)
        if asset['url']!='/'+name or path.as_posix()!=name or '\\' in name or '..' in path.parts or path.suffix not in MIME or ':' in name:
            raise ValueError('unsupported delivery path: '+name)
        row={'sha256':asset['sha256'],'bytes':asset['bytes'],'mime':MIME[path.suffix],
             'requiresRange':path.suffix=='.wav'}
        if name in rows and rows[name]!=row:
            raise ValueError('conflicting delivery references: '+name)
        rows[name]=row
    return dict(sorted(rows.items()))

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--runtime',required=True,type=Path)
    p.add_argument('--out',required=True,type=Path)
    args=p.parse_args()
    source=args.runtime.resolve();out=args.out.resolve()
    if not out.is_relative_to(ROOT):p.error('output must stay in workspace')
    manifest,runtime_id,_=verify_runtime_file(ROOT,source)
    audio=json.loads((source.parent/manifest['audioCatalog']['url'].lstrip('/')).read_bytes())
    rows=delivery_entries(manifest,audio)
    raw=source.read_bytes()
    rows['manifest.json']={'sha256':sha(raw),'bytes':len(raw),'mime':'application/json','requiresRange':False}
    destination=out/'contra'/('runtime-'+runtime_id)
    out.mkdir(parents=True,exist_ok=False)
    for name,entry in rows.items():
        file=destination/name;file.parent.mkdir(parents=True,exist_ok=True)
        shutil.copyfile(source if name=='manifest.json' else source.parent/name,file)
        body=file.read_bytes()
        if len(body)!=entry['bytes'] or sha(body)!=entry['sha256']:raise ValueError('staged file mismatch: '+name)
    receipt={'runtimeBuildID':runtime_id,'rulesBuildID':manifest['rulesBuildID'],
             'prefix':'contra/runtime-'+runtime_id+'/', 'files':rows,
             'uploaded':False,'scope':'Local verified staging; CDN CORS/MIME/Range checks still required'}
    (out/'delivery.json').write_text(json.dumps(receipt,sort_keys=True,indent=2)+'\n')
    print(json.dumps({'files':len(rows),'out':str(out),'runtimeBuildID':runtime_id}))

if __name__=='__main__':main()
