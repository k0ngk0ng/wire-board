"""Failure inventory before campaign runner: inherited diagnostic environment,
skip-only success, one missing seat result, wrong mode/outcome, source drift,
prefix or grants labelled standard, and diagnostics counted as CI acceptance.
"""
import importlib.util,json,tempfile,unittest
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
spec=importlib.util.spec_from_file_location('campaigns',Path(__file__).with_name('campaign_modes.py'));campaigns=importlib.util.module_from_spec(spec);spec.loader.exec_module(campaigns)
class Modes(unittest.TestCase):
 def test_isolated_environments(self):
  base={'PATH':'kept','CONTRA_BOT_MODE':'diagnostic','CONTRA_BOT_PREFIX':'old','CONTRA_BOT_TARGET':'2','CONTRA_BOT_SEATS':'1','CONTRA_BOT_INFINITE_LIVES':'1','CONTRA_BOT_FRAMES':'1','CONTRA_BOT_EVIDENCE':'old','CONTRA_BOT_UNKNOWN':'leak'}
  for mode in ['standard-terminal','assisted-clear']:
   env=campaigns.campaign_env(mode,ROOT/'.local/example',base)
   self.assertEqual(env['PATH'],'kept');self.assertEqual(env['CONTRA_BOT_MODE'],mode);self.assertEqual(env['CONTRA_BOT_TARGET'],'8');self.assertNotIn('CONTRA_BOT_PREFIX',env);self.assertNotIn('CONTRA_BOT_SEATS',env);self.assertNotIn('CONTRA_BOT_UNKNOWN',env)
   self.assertEqual(env['CONTRA_BOT_INFINITE_LIVES'],'1'if mode=='assisted-clear'else'0')
  with self.assertRaises(ValueError):campaigns.campaign_env('diagnostic',ROOT/'.local/example',base)
 def test_missing_skip_and_mislabelled_reports_fail(self):
  directory=ROOT/'.local/contra-campaign-modes';directory.mkdir(parents=True,exist_ok=True)
  with tempfile.TemporaryDirectory(dir=directory)as name:
   out=Path(name);events=[{'Action':'run','Test':'TestBotStandardCampaignE2E'},{'Action':'pass','Test':'TestBotStandardCampaignE2E'}]
   for seat in [1,2]:
    events.extend([{'Action':'run','Test':f'TestBotStandardCampaignE2E/players{seat}'},{'Action':'pass','Test':f'TestBotStandardCampaignE2E/players{seat}'}])
    d=out/f'players{seat}';d.mkdir();(d/'result.json').write_text(json.dumps({'mode':'standard-terminal','acceptance':True,'passed':True,'players':seat,'target_level':8,'prefix_frames':0,'infinite_lives':False,'finalized':True,'outcome':'failed','source_changed_during_run':False,'restore_checkpoints':1}))
   log=out/'go-test.jsonl';log.write_text(''.join(json.dumps(row)+'\n'for row in events));campaigns.validate_reports(out,'standard-terminal',[1,2],log)
   for mutation in ['skip','missing-seat','diagnostic','prefix','assistance','unfinished','drift','wrong-clear']:
    with self.subTest(mutation=mutation):
     original=(out/'players2/result.json').read_text();value=json.loads(original);saved=log.read_text()
     if mutation=='skip':log.write_text(saved+json.dumps({'Action':'skip','Test':'TestBotStandardCampaignE2E/players1'})+'\n')
     elif mutation=='missing-seat':(out/'players2/result.json').unlink()
     elif mutation=='diagnostic':value['acceptance']=False
     elif mutation=='prefix':value['prefix_frames']=2
     elif mutation=='assistance':value['infinite_lives']=True
     elif mutation=='unfinished':value['finalized']=False
     elif mutation=='drift':value['source_changed_during_run']=True
     else:value['mode']='assisted-clear'
     if mutation not in ['skip','missing-seat']:(out/'players2/result.json').write_text(json.dumps(value))
     with self.assertRaises(ValueError):campaigns.validate_reports(out,'standard-terminal',[1,2],log)
     (out/'players2/result.json').write_text(original);log.write_text(saved)
if __name__=='__main__':unittest.main()
