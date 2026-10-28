"""Run a named standard bot E2E attempt with source and artifact identity."""
import argparse,hashlib,json,os,re,subprocess
from pathlib import Path
from campaign_modes import campaign_env, validate_reports
ROOT=Path(__file__).resolve().parents[2]
def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def sources():return {str(p.relative_to(ROOT)):sha(p)for p in sorted((ROOT/'games/contra/engine').rglob('*'))if p.is_file()and p.suffix in {'.go','.json'}}
def main():
 p=argparse.ArgumentParser();p.add_argument('--name',required=True);p.add_argument('--mode',required=True,choices=['standard-terminal','assisted-clear','diagnostic']);p.add_argument('--players',type=int,choices=[1,2]);p.add_argument('--frames',type=int);p.add_argument('--target',type=int,choices=range(1,9),default=8);p.add_argument('--prefix');p.add_argument('--infinite-lives',action='store_true');a=p.parse_args()
 assert re.fullmatch('[a-z0-9][a-z0-9-]{0,80}',a.name)
 if a.mode!='diagnostic' and (a.prefix or a.target!=8 or a.infinite_lives and a.mode!='assisted-clear'):p.error('prefix/short target/conflicting assistance require diagnostic mode')
 out=ROOT/'.local/contra-bot/runs'/a.name;out.mkdir(parents=True,exist_ok=False)
 before=sources()
 env=campaign_env(a.mode,out,os.environ) if a.mode!='diagnostic' else {k:v for k,v in os.environ.items() if not k.startswith('CONTRA_BOT_')}
 env.update(CONTRA_BOT_MODE=a.mode,CONTRA_BOT_TARGET=str(a.target),CONTRA_BOT_EVIDENCE=str(out))
 if a.frames is not None:env['CONTRA_BOT_FRAMES']=str(a.frames)
 if a.players:env['CONTRA_BOT_SEATS']=str(a.players)
 if a.infinite_lives:env['CONTRA_BOT_INFINITE_LIVES']='1'
 prefix=None
 if a.prefix:
  path=Path(a.prefix).resolve();assert path.is_relative_to(ROOT)and path.is_file()
  env['CONTRA_BOT_PREFIX']=str(path);prefix={'path':str(path.relative_to(ROOT)),'sha256':sha(path)}
 with(out/'go-test.jsonl').open('w')as log:
  code=subprocess.run(['zsh','-c','source scripts/env.sh && go test -json ./games/contra/engine -run "^TestBotStandardCampaignE2E$" -count=1 -timeout=90m'],cwd=ROOT,env=env,stdout=log,stderr=subprocess.STDOUT).returncode
 after=sources()
 report={'test_exit':code,'target_level':a.target,'full_campaign_required':a.target==8,'source_unchanged_during_test':before==after,'source_sha256':before,'source_changes':[name for name in sorted(before.keys()|after.keys())if before.get(name)!=after.get(name)],'results':{p.parent.name:json.loads(p.read_text())for p in sorted(out.glob('players*/result.json'))},'artifacts':{str(p.relative_to(out)):sha(p)for p in sorted(out.rglob('*'))if p.is_file()}}
 report['mode']=a.mode
 report['validation_error']=None
 if a.mode!='diagnostic':
  try:validate_reports(out,a.mode,[a.players] if a.players else [1,2],out/'go-test.jsonl')
  except ValueError as error:report['validation_error']=str(error)
 else:
  events=[json.loads(line) for line in (out/'go-test.jsonl').read_text().splitlines() if line.startswith('{')]
  actions={event.get('Action') for event in events if event.get('Test')=='TestBotStandardCampaignE2E'}
  if not {'run','pass'}<=actions or actions&{'skip','fail'}:report['validation_error']='diagnostic test did not execute successfully'
 report['legal_pad_prefix']=prefix
 report['requested_infinite_lives']=env.get('CONTRA_BOT_INFINITE_LIVES')=='1'
 (out/'verification.json').write_text(json.dumps(report,indent=2)+'\n')
 compact={name:{k:v for k,v in result.items()if k not in {'source_sha256'}}for name,result in report['results'].items()}
 print(json.dumps({'path':str(out.relative_to(ROOT)),'exit':code,'source_unchanged':before==after,'results':compact}))
 raise SystemExit(code or (0 if before==after and not report['validation_error'] else 2))
if __name__=='__main__':main()
