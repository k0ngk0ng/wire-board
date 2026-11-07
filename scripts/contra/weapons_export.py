"""Retain bounded weapon REA evidence, repeated captures and check identities."""
import argparse,gzip,hashlib,json,re
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-weapons'
OUT=ROOT/'docs/research/nes-contra/evidence'

def digest(raw):return hashlib.sha256(raw).hexdigest()

def main():
    parser=argparse.ArgumentParser();parser.add_argument('--weapon-rea',default='final-rea');parser.add_argument('--fixed-rea',default='fixed2-rea')
    parser.add_argument('--equipment-labels',nargs=2,default=['equipment-v1','equipment-v2'])
    parser.add_argument('--isolated-labels',nargs=2,default=['isolated-v1','isolated-v3'])
    parser.add_argument('--natural-labels',nargs=2,default=['natural-v1','natural-v2'])
    parser.add_argument('--machine-labels',nargs=2,default=['natural-machinegun-01','natural-machinegun-02'])
    args=parser.parse_args()
    assert all(re.fullmatch(r'[a-z0-9][a-z0-9-]{0,64}',name) for name in args.equipment_labels+args.isolated_labels+args.natural_labels+args.machine_labels)
    functions=[];bundles=[]
    for name,directory in [('weapon',args.weapon_rea),('fixed',args.fixed_rea)]:
        path=(BASE/directory).resolve();assert path.is_relative_to(BASE)
        source=path/'evidence.json';compressed=gzip.compress(source.read_bytes(),mtime=0)
        destination=OUT/f'weapons-{name}-rea.json.gz';destination.write_bytes(compressed)
        bundles.append({'path':destination.name,'sha256':digest(compressed)})
        for p in sorted(path.glob('function-*.json')):
            d=json.loads(p.read_text());v=d['normalized_result']
            functions.append({'address':v['procedure']['address'],'evidence_id':d['evidence_id'],'subject':d['subject'],'profile':d['analysis_profile']['digest'],'assembly':v['assembly'],'limitations':d['limitations']+v['limitations']})
    repeats=[]
    runs=BASE/'oracle/runs'
    equipment=list(sorted(runs.glob(args.equipment_labels[0]+'-*')));assert len(equipment)==30
    for first in equipment:
        second=runs/(args.equipment_labels[1]+first.name[len(args.equipment_labels[0]):])
        for filename in ['ram.bin','weapons.jsonl']:
            data=(first/filename).read_bytes();assert data==(second/filename).read_bytes()
            repeats.append({'run':first.name,'file':filename,'sha256':digest(data),'bytes':len(data)})
    for filename in ['ram.bin','weapons.jsonl']:
        data=(runs/args.isolated_labels[0]/filename).read_bytes();assert data==(runs/args.isolated_labels[1]/filename).read_bytes()
        repeats.append({'run':args.isolated_labels[0],'file':filename,'sha256':digest(data),'bytes':len(data)})
    assert (runs/args.natural_labels[0]/'ram.bin').read_bytes()==(runs/args.natural_labels[1]/'ram.bin').read_bytes()
    # The new record adds one observed global; compare all original fields.
    def natural(name):
        rows=[json.loads(line) for line in (runs/name/'weapons.jsonl').read_text().splitlines()]
        for row in rows:row.pop('screenCleared',None)
        return rows
    assert natural(args.natural_labels[0])==natural(args.natural_labels[1])
    machine_repeats=[]
    for filename in ['ram.bin','weapons.jsonl']:
        first=BASE/'pickup/runs'/args.machine_labels[0]/filename
        second=BASE/'pickup/runs'/args.machine_labels[1]/filename
        data=first.read_bytes();assert data==second.read_bytes()
        machine_repeats.append({'file':filename,'sha256':digest(data),'bytes':len(data)})
    fixtures={}
    for name in ['natural','isolated','equipment','machine']:
        p=OUT/f'weapons-{name}.json.gz';d=json.loads(gzip.decompress(p.read_bytes()))
        fixtures[name]={'sha256':digest(p.read_bytes()),'fire':len(d['fire']),'steps':len(d['steps']),'interventions':d['interventions'],'scope':d['scope']}
    report={'rom_sha256':'6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe','functions':functions,'bundles':bundles,'fixtures':fixtures,'repeat_byte_identical':repeats,'machine_repeat':machine_repeats,'natural_repeat':'RAM identical; original call fields identical after adding the read-only screenCleared field.','limitations':['These are subsystem call comparisons, not a complete independent campaign replay.','Natural captures cover ordinary weapon and M pickup/use; five-weapon/R trajectory coverage also uses declared staged equipment.','Nonzero horizontal scroll is observed in natural M playback; nonzero vertical scroll and solid-background paths currently have static REA evidence only.','Enemy hit detection, health/score changes, other natural pickups and all-stage traversal belong to further integration acceptance.']}
    (OUT/'weapons-index.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({'passed':True,'functions':len(functions),'repeated_artifacts':len(repeats),'fixtures':fixtures}))

if __name__=='__main__':main()
