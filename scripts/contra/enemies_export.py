"""Retain complete source-bound enemy evidence and bounded implementation scope."""
import gzip,hashlib,json
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-enemies'
OUT=ROOT/'docs/research/nes-contra/evidence'

def sha(data):return hashlib.sha256(data).hexdigest()

def main():
    groups={'spawn':'spawn-rea','dispatch':'enemy-rea','helpers':'helper-rea',
            'runner':'runner-valid-checked-rea','shared':'small-rea',
            'terrain':'terrain-known-rea','pickup':'pickup-checked-rea',
            'bridge':'bridge-rea','generation':'soldier-rea'}
    index={'identity':json.loads((BASE/'identity.json').read_text()),'analyses':[],
           'limitations':['Natural execution covers the recorded first-stage paths; controlled cases are reported separately.',
                          'Ghidra may include table fallthrough after branches whose runtime condition is constrained; decoded table bytes are not assumed executable.',
                          'No claim of natural Boss defeat, complete first-stage traversal or all-eight-stage implementation.']}
    OUT.mkdir(parents=True,exist_ok=True)
    for name,directory in groups.items():
        folder=BASE/directory;raw=(folder/'evidence.json').read_bytes();bundle=json.loads(raw)
        path=OUT/f'enemies-{name}-rea.json.gz';path.write_bytes(gzip.compress(raw,mtime=0))
        functions=[]
        for p in sorted(folder.glob('function-*.json')):
            d=json.loads(p.read_text())
            functions.append({'address':d['parameters']['procedure'],'evidence_id':d['evidence_id'],
                              'target_sha256':d['subject']['digest']['sha256'],
                              'assembly':d['normalized_result']['assembly'],'limitations':d['normalized_result']['limitations']})
        index['analyses'].append({'group':name,'bundle':path.name,'sha256':sha(path.read_bytes()),'functions':functions})
    natural=json.loads(gzip.decompress((OUT/'enemies-natural.json.gz').read_bytes()))
    index['natural']={'counts':natural['counts'],'states':natural['states'],'sha256':sha((OUT/'enemies-natural.json.gz').read_bytes())}
    generated=json.loads(gzip.decompress((ROOT/'games/contra/engine/testdata/original-enemy-generators.json.gz').read_bytes()))
    index['generation_bias']={'source_address':'$06','writer_counts':generated['burst_writer_counts'],
       'confirmed_sources':{'bank4/5:$92D9':'low byte of a shared meta-sprite pointer',
                            'fixed:$CF09':'end/graphics work result; integration owner restores its semantic source',
                            'fixed:$F225/$F269':'requested enemy-shot speed, then successful clamped speed',
                            'fixed:$EA08':'collision-quadrant loop completion value $FF'},
       'note':'Recorded writer PC is after its instruction; the old label "seed" is a behavior work value, not a new random generator.'}
    (OUT/'enemies-index.json').write_text(json.dumps(index,ensure_ascii=False,indent=2)+'\n')
    sources={}
    for pattern in ['enemies*.go','graphics_queue.go']:
        for p in sorted((ROOT/'games/contra/engine').glob(pattern)):
            sources[str(p.relative_to(ROOT))]=sha(p.read_bytes())
    verification={'scope':'First-stage enemy subsystem and source-verified common helpers. Not a complete game/online release acceptance.',
      'sources':sources,'proofs':{'natural_spawn':1228,'natural_bullet':924,'natural_runner':1123,'natural_capsule':257,
          'natural_capsule_destroy':1,'natural_box':868,'natural_pickup':121,'natural_bridge':161,'natural_generator':1900,
          'controlled_generator':1163,'controlled_bullet':63,'controlled_shared_explosion':43,'controlled_runner_destruction':81},
      'related':['enemies-collision.md','enemies-shooters.md','enemies-terrain.md'],
      'source_identity':'6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe',
      'counts_overlap':'Some nested routines and source scenarios overlap; numbers must not be summed as distinct matches.'}
    (OUT/'enemies-implementation-verification.json').write_text(json.dumps(verification,indent=2)+'\n')
    print(json.dumps({'native_bundles':len(groups),'function_records':sum(len(x['functions'])for x in index['analyses'])}))

if __name__=='__main__':main()
