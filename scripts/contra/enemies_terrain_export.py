"""Retain complete REA and differential-test evidence for graphics requests."""
import gzip,hashlib,json,shutil
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-enemies-terrain'
OUT=ROOT/'docs/research/nes-contra/evidence'

def sha(data):return hashlib.sha256(data).hexdigest()

def main():
    OUT.mkdir(parents=True,exist_ok=True)
    checks=[json.loads(line)for line in (BASE/'checks/abi-green-race.jsonl').read_text().splitlines()]
    assert not any(row.get('Action')=='fail'for row in checks)
    tests=[r['Test']for r in checks if r.get('Action')=='pass'and r.get('Test')]
    assert set(tests)=={'TestOriginalGraphicsCallerAttributeLookup','TestOriginalGraphicsRequests','TestGraphicsQueueMixedScrollAndEnemyBudget','TestGraphicsQueueValidationAndOwnership'}
    native=(BASE/'rea/evidence.json').read_bytes()
    (OUT/'enemies-terrain-rea.json.gz').write_bytes(gzip.compress(native,mtime=0))
    functions=[]
    for p in sorted((BASE/'rea').glob('function-*.json')):
        d=json.loads(p.read_text());functions.append({'requested':d['parameters']['procedure'],'evidence_id':d['evidence_id'],'assembly':d['normalized_result']['assembly'],'limitations':d['limitations']})
    repeats=[]
    for first,second in [('graphics-natural-v1','graphics-natural-v2'),('graphics-controlled-v1','graphics-controlled-v2'),('graphics-controlled-abi-v1','graphics-controlled-abi-v2')]:
        for name in ['ram.bin','graphics-requests.jsonl']:
            a=(BASE/'oracle/runs'/first/name).read_bytes();b=(BASE/'oracle/runs'/second/name).read_bytes();assert a==b
            repeats.append({'run':first,'file':name,'sha256':sha(a),'bytes':len(a)})
    natural=(BASE/'oracle/runs/graphics-natural-v1/ram.bin').read_bytes()
    assert natural==(ROOT/'.local/contra-weapons/pickup/runs/natural-machinegun-01/ram.bin').read_bytes()
    fixtures={}
    for p in sorted((ROOT/'games/contra/engine/testdata').glob('original-graphics-requests-*.json.gz')):
        d=json.loads(gzip.decompress(p.read_bytes()));fixtures[p.name]={'sha256':sha(p.read_bytes()),'records':len(d['records']),'scope':d['scope'],'interventions':d['interventions']}
    for name in ['red.jsonl','green.jsonl','green-race.jsonl','abi-red.jsonl','abi-green-race.jsonl']:
        shutil.copyfile(BASE/'checks'/name,OUT/('enemies-terrain-'+name))
    sources=['games/contra/engine/graphics_queue.go','games/contra/engine/graphics_queue_test.go',
             'scripts/contra/enemies_terrain_prepare.py','scripts/contra/enemies_terrain_capture.py','scripts/contra/enemies_terrain_export.py']
    result={'passed':True,'scope':'C3D1 graphics admission/packets, C656 coordinates/palette masks and caller attribute lookup; first-stage tileset. Does not claim complete enemy AI or campaign acceptance.',
        'natural_requests':3,'controlled_requests':420,'controlled_attribute_lookup_requests':420,'accepted':255,'rejected':168,
        'checks':tests,'race':'passed','natural_observation_ram_unchanged':True,'repeat':repeats,'fixtures':fixtures,
        'functions':functions,'sources':{p:sha((ROOT/p).read_bytes())for p in sources},
        'native_bundle_sha256':sha(native),'commands':['python3 -B scripts/contra/enemies_terrain_prepare.py','node scripts/contra/enemies_rea.mjs .local/contra-enemies-terrain/spec.json',
        '.local/board-art-env/bin/python -B scripts/contra/enemies_terrain_capture.py natural <new-label>',
        '.local/board-art-env/bin/python -B scripts/contra/enemies_terrain_capture.py controlled <new-label>',
        '.local/board-art-env/bin/python -B scripts/contra/enemies_terrain_capture.py controlled <new-abi-label> abi',
        'source scripts/env.sh && go test -race ./games/contra/engine -run \'^Test(OriginalGraphics|GraphicsQueue)\' -count=1 -json']}
    (OUT/'enemies-terrain-verification.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps({'passed':True,'tests':len(tests),'functions':len(functions),'original_calls':3+420+420}))

if __name__=='__main__':main()
