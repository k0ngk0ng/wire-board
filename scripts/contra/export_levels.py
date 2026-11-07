"""Make runtime-only level resources from the verified Japanese data catalog.

No execution oracle, ROM bank, CPU code or screenshot is included. Sources and
hashes remain in a separate manifest. Runtime consumers can request one stage.
"""
import gzip,hashlib,json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]

def maps(value):
    return {'rows':value['mapRows'],'entries':[{'index':m['index'],'encoded':m['encoded'],'aliasOf':m['aliasOf']} for m in value['entries']]}

def main():
    source=ROOT/'docs/research/nes-contra/evidence/levels-catalog.json.gz'
    raw=source.read_bytes();d=json.loads(gzip.decompress(raw))
    assert d['schema']==1 and d['romSHA256']=='6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe'
    palette_source=ROOT/'docs/research/nes-contra/evidence/palette-additional-colors.json'
    additions=json.loads(palette_source.read_text())
    assert additions['romSHA256']==d['romSHA256']
    palette_by_id={p['id']:p['colors'] for p in d['paletteColors']}
    for p in additions['colors']:
        if p['id'] in palette_by_id:assert palette_by_id[p['id']]==p['colors']
        palette_by_id[p['id']]=p['colors']
    requirements=additions.get('runtimePaletteRequirements',[])
    for requirement in requirements:
        assert requirement['stage'] in range(1,9)
        assert all(index in palette_by_id for index in requirement['paletteIndices'])
    stages=[]
    for s in d['stages']:
        stage={key:s[key] for key in ['stage','sceneType','scrollType','tileSetId','initialPaletteState','chrPreset','scrollSpanPixels','initialCollisionCheck','stopScrollScreen']}
        stage['maps']=maps(s['maps'])
        stage['alternate']={key:s['alternate'][key] for key in ['triggerScreen','mode','chrPreset','collisionLimits','cycleIndices','paletteIndices']}
        if 'baseBoss' in s:
            stage['baseBoss']={key:s['baseBoss'][key] for key in ['sceneType','mapIndex','mapSetId','tileSetId','chrPreset','trigger','enterAtScreen']}
        stages.append(stage)
    tile_sets=[]
    for t in d['tileSets']:
        tile_sets.append({'id':t['id'],'supertiles':[{'tiles':x['tiles'],'attribute':x['attribute']} for x in t['supertiles']],
                          'updateBase':t['updates']['baseSupertile'],'updateCount':t['updates']['count'],
                          'sourceAttributePage':t['updates']['attributesSource']['cpu']>>8})
    result={'schema':1,'romSHA256':d['romSHA256'],'stages':stages,'tileSets':tile_sets,
            'extraMapSets':{key:maps(value) for key,value in d['extraMapSets'].items()},
            'chrPresets':[{'id':p['id'],'pages':p['pages']} for p in d['chrPresets']],
            'paletteColors':[{'id':key,'colors':value} for key,value in sorted(palette_by_id.items())]}
    encoded=(json.dumps(result,separators=(',',':'))+'\n').encode()
    out=ROOT/'games/contra/engine/leveldata';out.mkdir(parents=True,exist_ok=True)
    (out/'levels.json').write_bytes(encoded)
    stage_files=[]
    for stage in stages:
        ids={stage['tileSetId']};presets={stage['chrPreset']};extra={}
        if stage['alternate']['chrPreset'] is not None:presets.add(stage['alternate']['chrPreset'])
        if 'baseBoss' in stage:
            boss=stage['baseBoss'];ids.add(boss['tileSetId']);presets.add(boss['chrPreset'])
            original=result['extraMapSets'][boss['mapSetId']]
            extra[boss['mapSetId']]={**original,'entries':[m for m in original['entries'] if m['index']==boss['mapIndex']]}
        partial={**result,'stages':[stage],'tileSets':[t for t in tile_sets if t['id'] in ids],
                 'chrPresets':[p for p in result['chrPresets'] if p['id'] in presets],'extraMapSets':extra}
        payload=(json.dumps(partial,separators=(',',':'))+'\n').encode();name=f'stage-{stage["stage"]}.json'
        (out/name).write_bytes(payload);stage_files.append({'file':name,'bytes':len(payload),'sha256':hashlib.sha256(payload).hexdigest()})
    manifest={'source':str(source.relative_to(ROOT)),'sourceSHA256':hashlib.sha256(raw).hexdigest(),
              'additionalPaletteSource':str(palette_source.relative_to(ROOT)),
              'additionalPaletteSHA256':hashlib.sha256(palette_source.read_bytes()).hexdigest(),
              'runtimePaletteRequirements':requirements,'paletteColorCount':len(palette_by_id),
              'additionalPaletteIDs':[p['id']for p in additions['colors']],
              'outputSHA256':hashlib.sha256(encoded).hexdigest(),'romSHA256':d['romSHA256'],
              'scope':'Eight-stage map/art definitions only; no gameplay execution oracle or ROM interpreter.',
              'stages':8,'normalMaps':sum(len(s['maps']['entries']) for s in stages),
              'bossMaps':sum(len(v['entries']) for v in result['extraMapSets'].values()),'tileSets':len(tile_sets),'stageFiles':stage_files}
    (out/'levels-source.json').write_text(json.dumps(manifest,indent=2)+'\n')
    print(json.dumps({'bytes':len(encoded),**manifest}))

if __name__=='__main__':main()
