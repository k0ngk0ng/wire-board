"""Retain Boss-door state/continuation evidence and the combined rule check."""
from pathlib import Path
import gzip,hashlib,json,shutil
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-shooters';OUT=ROOT/'docs/research/nes-contra/evidence'
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()

def main():
    paths=list((BASE/'bossdoor-rea').glob('function-*.json'))+list((BASE/'bossdoor-helper-rea').glob('function-*.json'))
    paths += [ROOT/'.local/contra-audio/rea/function-0x8719.json',ROOT/'.local/contra-audio/rea/function-0xf9d4.json']
    records=[json.loads(p.read_text())for p in paths]
    bundle={'scope':'Complete Japanese Boss-door function evidence; audio reset target uses the correct audio bank profile, not enemy-bank inferred callees.','records':records}
    (OUT/'enemies-shooters-bossdoor-rea.json.gz').write_bytes(gzip.compress(json.dumps(bundle,separators=(',',':')).encode(),mtime=0))
    log=BASE/'checks/with-bossdoor-race.jsonl';rows=[json.loads(line)for line in log.read_text().splitlines()]
    assert not any(row.get('Action')in ('fail','skip')for row in rows)
    tests=[row['Test']for row in rows if row.get('Action')=='pass'and'Test'in row];assert len(tests)==9
    for source,dest in [('bossdoor-red.jsonl','enemies-shooters-bossdoor-red.jsonl'),('bossdoor-green.jsonl','enemies-shooters-bossdoor-green.jsonl'),('with-bossdoor-race.jsonl','enemies-shooters-with-bossdoor-race.jsonl')]:shutil.copyfile(BASE/'checks'/source,OUT/dest)
    comparisons=[]
    for first,repeat in [(BASE/'bossdoor/runs/bossdoor-v1',BASE/'bossdoor/runs/bossdoor-v2'),
                         (BASE/'bossdoor-sequence/runs/door-sequence-v2-0',BASE/'bossdoor-sequence/runs/door-sequence-v3-0'),
                         (BASE/'bossdoor-sequence/runs/door-sequence-v2-1',BASE/'bossdoor-sequence/runs/door-sequence-v3-1')]:
        files=['ram.bin','bossdoor.jsonl']if first.name.startswith('bossdoor-')else ['ram.bin','door-sequence.jsonl','inputs.csv']
        for name in files:
            assert (first/name).read_bytes()==(repeat/name).read_bytes()
            comparisons.append({'first':str((first/name).relative_to(ROOT)),'repeat':str((repeat/name).relative_to(ROOT)),'sha256':sha(first/name)})
    code=sorted((ROOT/'games/contra/engine').glob('enemies_shooters*.go'))
    scripts=sorted((ROOT/'scripts/contra').glob('enemies_shooters_*.py'))
    fixtures=sorted((ROOT/'games/contra/engine/testdata').glob('original-enemy-shooters*.json.gz'))
    result={'passed':True,'scope':'First-stage Boss door and tunnel; controlled original state/continuation, not a naturally defeated Boss or full-game acceptance.',
            'isolated_original_states':102,'original_tunnel_collision_writes':16,
            'continuation_sequences':[{'mode':'defeat-to-removal','calls':115,'initial_writes':'one explicit defeated-door setup'},{'mode':'tunnel-with-surviving-box-contention','calls':65,'rejected_requests':1,'initial_writes':'one explicit tunnel/box setup'}],
            'signals':{'BossDoorDefeated':{'BossDefeated':1,'DelayLow':255,'DelayHigh':'preserved','audio':'reset before77 and later effects'},'BossDoorTunnelFinished':{'DelayLow':48,'DelayHigh':0,'BossDefeated':'preserved'}},
            'native_records':len(records),'native_evidence_ids':[r['evidence_id']for r in records],
            'race_tests':tests,'repeat_comparisons':comparisons,
            'source_sha256':{str(p.relative_to(ROOT)):sha(p)for p in code+scripts},'fixture_sha256':{str(p.relative_to(ROOT)):sha(p)for p in fixtures},
            'limitations':['Original first-stage Boss fight has not been naturally reached/cleared by this task.',
                           'World must immediately consume progression signals and AudioReset; this component never writes platform victory or ratings.',
                           'Independent complete campaign, browser interaction and computer-player acceptance remain separate.']}
    (OUT/'enemies-shooters-bossdoor-verification.json').write_text(json.dumps(result,indent=2)+'\n')
    old=OUT/'enemies-shooters-verification.json';summary=json.loads(old.read_text());summary['revalidated_with_bossdoor']='enemies-shooters-bossdoor-verification.json';summary['source_sha256']=result['source_sha256'];summary['fixture_sha256']=result['fixture_sha256'];old.write_text(json.dumps(summary,indent=2)+'\n')
    print(json.dumps({'passed':True,'tests':len(tests),'native_records':len(records)}))

if __name__=='__main__':main()
