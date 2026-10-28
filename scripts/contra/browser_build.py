#!/usr/bin/env python3
"""Build a private native/Wasm browser proof from source and normalized assets.

No ROM or captured game state is read. Only source-bound indexed PNG pixels,
the static RGB table and existing current-stage/audio resource descriptors load.
"""
import argparse,base64,gzip,hashlib,json,os,shutil,subprocess,sys
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
sys.dont_write_bytecode=True
sys.path.insert(0,str(ROOT/'games/contra/build'))
from identity import encode_manifest,go_env,inspect_build,native_flags
from media import read_media,verify_pack
from media_archive import prepare_source_media
def sha(data):return hashlib.sha256(data).hexdigest()
def main():
    parser=argparse.ArgumentParser();parser.add_argument('--out',default='.local/contra-browser/build');parser.add_argument('--media-dir');args=parser.parse_args()
    media_dir=(ROOT/args.media_dir).resolve()if args.media_dir else prepare_source_media(ROOT)
    if media_dir is not None:
        assert media_dir.is_relative_to(ROOT),'media pack outside workspace'
        verify_pack(ROOT,media_dir)
    out=(ROOT/args.out).resolve();assert out.is_relative_to(ROOT);public=out/'public';public.mkdir(parents=True,exist_ok=True)
    raw=(ROOT/'games/contra/engine/leveldata/stage-1.json').read_bytes();rules=json.loads(raw)
    context=inspect_build(ROOT);rules_build=context['rules'];env=go_env(ROOT)
    visual_path=ROOT/'games/contra/server/data/visual-catalog-v1.json.gz'
    catalog=json.loads(gzip.decompress(visual_path.read_bytes()))
    scenes=json.loads((ROOT/'games/contra/assets/runtime-stages.json').read_text())
    for name,digest in scenes['sources'].items():assert sha((ROOT/name).read_bytes())==digest,('stale scene catalogue',name)
    assert rules['romSHA256']==catalog['romSHA256']==scenes['romSHA256']
    assert catalog['catalogID']==context['visuals']['id']and catalog['catalog']==context['visuals']['manifest']
    stage_pages={str(stage['level']):sorted({page for name,scene in stage.items()if name!='level'and isinstance(scene,dict)for page in scene.get('chrPages',[])})for stage in scenes['stages']}
    assert set(stage_pages)==set(map(str,range(8)))and all(stage_pages.values())
    needed=set().union(*map(set,stage_pages.values()))
    assets={};pages={}
    def save(name,data,source):
        path=public/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(data)
        entry={'url':'/'+name,'bytes':len(data),'sha256':sha(data),'source':source};assets[name]=entry;return entry
    for page in sorted(needed):
        indices=base64.b64decode(catalog['pages'][str(page)],validate=True);record=catalog['catalog']['pages'][str(page)]
        assert len(indices)==4096 and max(indices)<=3 and sha(indices)==record['sha256']
        pages[str(page)]=save(f'graphics/page-{page:03d}.idx',indices,str(visual_path.relative_to(ROOT))+f'#page-{page}')
    rgb=base64.b64decode(catalog['rgb'],validate=True);assert len(rgb)==192 and sha(rgb)==catalog['catalog']['rgb']['sha256']
    rgb_entry=save('graphics/colors.rgb',rgb,str(visual_path.relative_to(ROOT))+'#rgb')
    assert set(pages)==set(catalog['pages']),'browser/native visual page set differs'
    resources=save('stage-1.json',raw,'games/contra/engine/leveldata/stage-1.json')
    stages={str(i):save(f'stage-{i+1}.json',(ROOT/f'games/contra/engine/leveldata/stage-{i+1}.json').read_bytes(),f'games/contra/engine/leveldata/stage-{i+1}.json')for i in range(8)}
    source=json.loads((ROOT/'games/contra/assets/audio-source.json').read_text());loops=json.loads((ROOT/'games/contra/assets/audio-loops.json').read_text())
    continuous={s['id']for s in loops['songs']};source['sounds']=[s for s in source['sounds']if s['id']not in continuous]
    audio_paths={}
    for sound in source['sounds']+loops['songs']:
        for stem in sound['stems']+([sound]if sound['id']==0x6e else[]):
            name=f"{stem['researchDirectory']}/{stem['file']}"if stem.get('researchDirectory')else stem['file'];path=(ROOT/name).resolve();assert path.is_relative_to(ROOT)
            data=read_media(ROOT,stem,media_dir);entry=save('audio/'+stem['sha256']+'.wav',data,name);audio_paths[name]=entry
    audio_catalog=save('audio.json',json.dumps({'source':source,'loops':loops,'paths':audio_paths},separators=(',',':')).encode(),'games/contra/assets/audio-source.json + audio-loops.json')
    def source_inputs():return {str(p.relative_to(ROOT)):sha(p.read_bytes())for folder in ['games/contra/engine','games/contra/runtime','games/contra/web','games/contra/assets']for p in sorted((ROOT/folder).rglob('*'))if p.is_file()and not p.name.endswith('_test.go')and p.suffix in ('.go','.json','.ts','.tsx','.css')}
    before=source_inputs()
    flags='-s -w -X github.com/k0ngk0ng/wire-board/games/contra/runtime.CompiledRulesBuildID='+rules_build['id']
    subprocess.run(['go','build','-trimpath','-buildvcs=false','-ldflags='+flags,'-o',str(public/'contra.wasm'),'./games/contra/runtime/cmd/wasm'],cwd=ROOT,env=dict(env,GOOS='js',GOARCH='wasm'),check=True)
    wasm=(public/'contra.wasm').read_bytes();payloads={str(p.relative_to(ROOT)):p.read_bytes()for p in sorted((ROOT/'games/contra/engine').rglob('*.json'))};payload_checks=[]
    for name,data in payloads.items():
        if '/leveldata/stage-' not in name:continue
        choices=[(at,data[at:at+256])for at in range(0,len(data)-256,64)]
        at,feature=next((at,feature)for at,feature in choices if not any(feature in other for key,other in payloads.items()if key!=name))
        whole=data in wasm;fragment=feature in wasm
        payload_checks.append({'path':name,'sha256':sha(data),'featureOffset':at,'featureBytes':256,'featureSHA256':sha(feature),'fullPayloadPresent':whole,'uniqueFeaturePresent':fragment})
        assert not whole and not fragment,('unused stage payload retained in Wasm',name)
    assert len(payload_checks)==8
    native_visuals=visual_path.read_bytes();assert native_visuals not in wasm and native_visuals[64:320]not in wasm,'native-only pixels retained in Wasm'
    goroot=subprocess.check_output(['go','env','GOROOT'],cwd=ROOT,env=env,text=True).strip();loader=Path(goroot)/'lib/wasm/wasm_exec.js';shutil.copyfile(loader,public/'wasm_exec.js')
    build_code="import{build}from'./web/node_modules/esbuild/lib/main.js';await build({entryPoints:['scripts/contra/browser_entry.tsx'],outfile:process.argv[1],bundle:true,format:'esm',platform:'browser',jsx:'automatic',nodePaths:[process.cwd()+'/web/node_modules'],loader:{'.svg':'dataurl'}});"
    subprocess.run(['node','--input-type=module','-e',build_code,str(public/'app.js')],cwd=ROOT,env=env,check=True)
    (public/'index.html').write_text('<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>魂斗罗运行层验收</title><link rel="stylesheet" href="/app.css"><div id="root"></div><script type="module" src="/app.js"></script></html>')
    manifest={'version':1,'rulesBuildID':rules_build['id'],'visualsID':context['visuals']['id'],'runtime':context['runtime'],'bundle':rules_build['bundle'],'scope':'Private development and platform browser fixture; per-stage rules and graphics, event-lazy audio; no production registration or ROM runtime','romSHA256':rules['romSHA256'],'resources':resources,'stages':stages,'stagePages':stage_pages,'loader':{'url':'/wasm_exec.js','bytes':loader.stat().st_size,'sha256':sha(loader.read_bytes())},'rgb':rgb_entry,'pages':pages,
              'wasm':{'url':'/contra.wasm','bytes':(public/'contra.wasm').stat().st_size,'sha256':sha((public/'contra.wasm').read_bytes())},'audio':'/audio.json','audioCatalog':audio_catalog}
    runtime_id,encoded=encode_manifest(manifest);(public/'manifest.json').write_bytes(encoded)
    subprocess.run(['go','build','-trimpath','-buildvcs=false','-ldflags='+native_flags(rules_build['id'],runtime_id),'-o',str(out/'development-server'),'./games/contra/runtime/cmd/development'],cwd=ROOT,env=env,check=True)
    inputs=source_inputs();assert inputs==before,'source changed during build; repeat after the current edit finishes'
    assert context==inspect_build(ROOT),'rule/runtime/native pixels changed during build'
    (out/'build-source.json').write_text(json.dumps({'toolchain':subprocess.check_output(['go','version'],cwd=ROOT,env=env,text=True).strip(),'inputs':inputs,'rulesBuild':rules_build,'runtimeBuildID':runtime_id,'visualsID':context['visuals']['id'],'runtime':context['runtime'],'wasmPayloadChecks':payload_checks,'nativeVisualsExcludedFromWasm':True,'wasm':manifest['wasm'],'normalizedPages':sorted(needed),'assetSources':assets},indent=2)+'\n');print(public)

if __name__=='__main__':main()
