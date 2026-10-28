"""Source-bound verification for reset/menu owners, independent of other games."""
import gzip,hashlib,json,subprocess
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
BASE=ROOT/'.local/contra-campaign-reset'
EVIDENCE=ROOT/'docs/research/nes-contra/evidence'
def digest(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def sources():
 paths=list((ROOT/'games/contra/engine').rglob('*.go'))+list((ROOT/'games/contra/engine').glob('*.json'))+list((ROOT/'games/contra/engine/leveldata').glob('*.json'))
 return {str(p.relative_to(ROOT)):digest(p)for p in sorted(paths)}
def main():
 before=sources();output=EVIDENCE/'campaign-reset-race-bound.jsonl'
 with output.open('w')as log:
  code=subprocess.run(['zsh','-c','source scripts/env.sh && go test -race -json ./games/contra/engine -run "^TestCampaignReset" -count=1'],cwd=ROOT,stdout=log,stderr=subprocess.STDOUT).returncode
 after=sources()
 assert code==0,'reset/menu checks failed'
 assert before==after,'source changed while checking; no verified artifact claimed'
 rows=[json.loads(l)for l in output.read_text().splitlines()]
 passed=[r['Test']for r in rows if r.get('Action')=='pass'and r.get('Test')]
 assert len(passed)==105 and rows[-1]['Action']=='pass'
 records=[json.loads(p.read_text())for p in sorted((BASE/'fixed-rea').glob('function-*.json'))]
 assert len(records)==8 and all(r['operation']=='analyze_function'for r in records)
 bundle=EVIDENCE/'campaign-reset-rea.json.gz'
 bundle.write_bytes(gzip.compress(json.dumps({'scope':'Complete private-session REA Evidence. Declared original runtime captures separately establish execution boundaries.','records':records},separators=(',',':')).encode(),mtime=0))
 repeat=json.loads((BASE/'oracle/runs/repeat-boundaries/repeat-verification.json').read_text());assert repeat['passed']and repeat['records']==138
 original=EVIDENCE/'campaign-reset-original.json.gz'
 original_data=json.loads(gzip.decompress(original.read_bytes()));assert len(original_data['cases'])==74 and len(original_data['records'])==138
 report={'passed':True,'scope':'Original CD1F/C309 runtime reset, C2CF resources and CDAF/game-over display phases; full campaign/network acceptance belongs to enclosing World integration.','original_cases':74,'original_entry_return_records':138,'runtime_reset_comparisons':40,'continue_resource_comparisons':8,'gameover_display_comparisons':8,'continue_display_comparisons':32,'failure_subcases':19,'test_entries_passed':len(passed),'source_unchanged_during_test':True,'source_sha256':before,'original':{'path':str(original.relative_to(ROOT)),'sha256':digest(original)},'rea':{'path':str(bundle.relative_to(ROOT)),'sha256':digest(bundle),'records':8},'independent_repeat':repeat,'test_output':{'path':str(output.relative_to(ROOT)),'sha256':digest(output)},'red_outputs':{name:digest(EVIDENCE/name)for name in ['campaign-reset-runtime-red.jsonl','campaign-reset-resources-red.jsonl','campaign-reset-display-red.jsonl']},'test_command':'source scripts/env.sh && go test -race -json ./games/contra/engine -run "^TestCampaignReset" -count=1','limitations':['Controlled original phase inputs are declared; no naturally completed campaign is claimed.','Game-over presentation covers non-demo product mode only; progression and all sounds remain owned by CampaignPlan.','The isolated exhausted-menu routine has no display writes and requires no separate display implementation.']}
 (EVIDENCE/'campaign-reset-verification.json').write_text(json.dumps(report,indent=2)+'\n')
 print(json.dumps({k:report[k]for k in ['passed','original_cases','original_entry_return_records','test_entries_passed','source_unchanged_during_test']}))
if __name__=='__main__':main()
