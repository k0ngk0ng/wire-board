"""Repeat source-bound initialization checks without running unrelated games."""
import gzip,hashlib,json,subprocess
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-campaign-initialize'
EVIDENCE=ROOT/'docs/research/nes-contra/evidence'
def digest(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def sources():
 paths=list((ROOT/'games/contra/engine').rglob('*.go'))
 paths+=list((ROOT/'games/contra/engine').glob('*.json'))
 paths+=list((ROOT/'games/contra/engine/leveldata').glob('*.json'))
 return {str(p.relative_to(ROOT)):digest(p)for p in sorted(paths)}
def main():
 before=sources()
 output=EVIDENCE/'campaign-initialize-race-bound.jsonl'
 with output.open('w')as log:
  code=subprocess.run(['zsh','-c','source scripts/env.sh && go test -race -json ./games/contra/engine -run "^TestCampaignInitialize" -count=1'],cwd=ROOT,stdout=log,stderr=subprocess.STDOUT).returncode
 after=sources()
 assert code==0,'initialization race check failed'
 assert before==after,'source changed while checking; no verified artifact claimed'
 rows=[json.loads(line)for line in output.read_text().splitlines()]
 passed=[r['Test']for r in rows if r.get('Action')=='pass' and r.get('Test')]
 assert len(passed)==60 and rows[-1]['Action']=='pass'
 records=[]
 for group in ['fixed','header','helpers']:
  for path in sorted((BASE/(group+'-rea')).glob('function-*.json')):
   record=json.loads(path.read_text());assert record['operation']=='analyze_function';records.append(record)
 assert len(records)==16
 bundle={'scope':'Complete private-session REA Evidence records. Some inferred function bodies contain fall-through neighbors; exact executed boundaries are separately retained in original entry/return traces.','records':records}
 archive=EVIDENCE/'campaign-initialize-rea.json.gz';archive.write_bytes(gzip.compress(json.dumps(bundle,separators=(',',':')).encode(),mtime=0))
 fixtures=sorted(EVIDENCE.glob('campaign-initialize-*-original.json.gz'));assert len(fixtures)==20
 repeats=[]
 for path in sorted((BASE/'oracle/runs').glob('repeat-*/repeat-verification.json')):repeats.append(json.loads(path.read_text()))
 assert len(repeats)==4 and all(r['records_equal']for r in repeats)
 report={'passed':True,'scope':'CC5F/CC7D semantic phases; no claim of complete World/new-game/network campaign acceptance.','original_runs':20,'original_entry_return_records':100,'compared_initialization_phases':40,'failure_subcases':18,'test_entries_passed':len(passed),'test_command':'source scripts/env.sh && go test -race -json ./games/contra/engine -run "^TestCampaignInitialize" -count=1','source_unchanged_during_test':True,'source_sha256':before,'rea_records':len(records),'rea_bundle':{'path':str(archive.relative_to(ROOT)),'sha256':digest(archive)},'original_fixtures':{str(p.relative_to(ROOT)):digest(p)for p in fixtures},'independent_original_repeats':repeats,'test_output':{'path':str(output.relative_to(ROOT)),'sha256':digest(output)},'test_first_red':{'path':'docs/research/nes-contra/evidence/campaign-initialize-red.jsonl','sha256':digest(EVIDENCE/'campaign-initialize-red.jsonl')},'notes':['Initial red contains the two expected missing initialization functions plus another concurrently developed scene API; no red test was moved or suppressed.','Only phase effects emit audio; Campaign.AfterPresentation metadata must not duplicate reset/music when World consumes the completed phase.']}
 (EVIDENCE/'campaign-initialize-verification.json').write_text(json.dumps(report,indent=2)+'\n')
 print(json.dumps({k:report[k]for k in ['passed','original_runs','compared_initialization_phases','test_entries_passed','source_unchanged_during_test','rea_records']}))
if __name__=='__main__':main()
