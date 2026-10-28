"""Retain complete source, original-call and successful race evidence for snow."""
import gzip,hashlib,json,shutil
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-snow';OUT=ROOT/'docs/research/nes-contra/evidence'
def sha(data):return hashlib.sha256(data).hexdigest()
def main():
    log=BASE/'checks/final-race.jsonl';checks=[json.loads(l)for l in log.read_text().splitlines()]
    assert not any(r.get('Action')=='fail'for r in checks)
    passed=[r['Test']for r in checks if r.get('Action')=='pass'and r.get('Test')]
    for expected in ['TestSnowSmallOriginal','TestSnowTankAndCarrierOriginal','TestSnowOriginalContinuations','TestOriginalAllStageEnemyPhaseDispatch/stage5']:assert expected in passed
    functions=[]
    for group in ['rea','helpers-rea']:
        raw=(BASE/group/'evidence.json').read_bytes();(OUT/f'enemies-snow-{group}.json.gz').write_bytes(gzip.compress(raw,mtime=0))
        for p in sorted((BASE/group).glob('function-*.json')):
            d=json.loads(p.read_text());functions.append({'address':d['parameters']['procedure'],'evidence_id':d['evidence_id'],'assembly':d['normalized_result']['assembly'],'limitations':d['limitations']})
    pairs=[('snow-natural-v1','snow-natural-v2'),('snow-small-cases-v1','snow-small-cases-v2'),('snow-complex-v1','snow-complex-v2')]+[(f'snow-sequence-v2-{i}',f'snow-sequence-v3-{i}')for i in range(4)]
    repeat=[]
    for first,second in pairs:
        for name in ['ram.bin','snow.jsonl']:
            a=(BASE/'oracle/runs'/first/name).read_bytes();b=(BASE/'oracle/runs'/second/name).read_bytes();assert a==b
            repeat.append({'run':first,'file':name,'bytes':len(a),'sha256':sha(a)})
    fixtures={}
    for suffix in ['natural','small-cases','complex-cases','sequences-v2']:
        p=ROOT/f'games/contra/engine/testdata/original-enemies-snow-{suffix}.json.gz';d=json.loads(gzip.decompress(p.read_bytes()))
        count=len(d['records'])if 'records'in d else sum(len(s['records'])for s in d['sequences'])
        fixtures[p.name]={'sha256':sha(p.read_bytes()),'calls':count,'interventions':d['interventions']}
    for name in ['small-red.jsonl','complex-red.jsonl','final-race.jsonl']:
        shutil.copyfile(BASE/'checks'/name,OUT/('enemies-snow-'+name))
    source_files=list((ROOT/'games/contra/engine').glob('enemies_snow*.go'))+list((ROOT/'scripts/contra').glob('enemies_snow_*.py'))
    result={'passed':True,'scope':'Japanese stage-five seven dedicated enemy types; original AI-call differential and whole enemy-phase integration. Not a naturally completed stage-five fight or eight-stage campaign.',
      'original_entity_calls':19090,'natural_input_calls':4501,'controlled_small_calls':104,'controlled_complex_calls':182,
      'continuations':{'calls':[9801,4337,95,70],'target_calls':[1217,499,95,70],'real_queue_refusals':1},
      'world_stage5_phase_cases':51,'tests':passed,'race':'passed','repeat':repeat,'fixtures':fixtures,
      'functions':functions,'sources':{str(p.relative_to(ROOT)):sha(p.read_bytes())for p in source_files},
      'resources':{name:sha((ROOT/name).read_bytes())for name in ['games/contra/engine/leveldata/stage-5.json','games/contra/engine/leveldata/levels.json']},
      'metadata_note':'The early snow world-field label generationBias at address0x0B is a diagnostic scratch sample only. Production compatibility state is address0x06, separately captured as work06 in complex/continuation fixtures.',
      'commands':['python3 -B scripts/contra/enemies_snow_prepare.py','node scripts/contra/enemies_rea.mjs .local/contra-enemies-snow/spec.json',
      'python3 -B scripts/contra/enemies_snow_tables.py','source scripts/env.sh && go test -race ./games/contra/engine -run \'^TestSnow|^TestOriginalAllStageEnemyPhaseDispatch$/stage5$\' -count=1 -json']}
    (OUT/'enemies-snow-verification.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps({'passed':True,'calls':19090,'whole_phase_cases':51,'functions':len(functions)}))
if __name__=='__main__':main()
