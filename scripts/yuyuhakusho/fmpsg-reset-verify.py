"""Bind ordered-reset source, actual counterexample, bounds and full regressions."""
from pathlib import Path
import argparse,base64,gzip,hashlib,json
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'games/yuyuhakusho/verification/fmpsg-reset'
def sha(raw):return hashlib.sha256(raw).hexdigest()
def item(path):
 raw=path.read_bytes();return dict(path=str(path.relative_to(ROOT)),bytes=len(raw),sha256=sha(raw))
def checklog(path):
 rows=[json.loads(s)for s in path.read_text().splitlines()if s.startswith('{')]
 assert not any(x.get('Action')in ['fail','build-fail']for x in rows),path
 passed=[x for x in rows if x.get('Action')=='pass'];groups={}
 for x in passed:
  if x.get('Test'):
   counts=groups.setdefault(x['Package'],dict(top=0,sub=0));counts['sub'if'/'in x['Test']else'top']+=1
 assert sum(x['top']for x in groups.values())==25 and sum(x['sub']for x in groups.values())==106 and len(groups)==4,(path,groups)
 assert groups['github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/pcmplayer']==dict(top=6,sub=37)
 return dict(packages=groups,log=item(path))
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--audit',action='store_true');a=p.parse_args();target=OUT/'validation.json'
 if a.audit:
  frozen=json.loads(target.read_text())
  for expected in frozen['sources']+frozen['artifacts']:assert item(ROOT/expected['path'])==expected,('frozen identity changed',expected['path'])
  print(json.dumps(dict(pass_=True,sources=len(frozen['sources']),artifacts=len(frozen['artifacts']),sha256=sha(target.read_bytes()))));return
 capture=json.loads((OUT/'native-capture.json').read_text());fixture_raw=gzip.decompress((OUT/'native.json.gz').read_bytes());fixture=json.loads(fixture_raw)
 assert capture['pass']and capture['gzip_sha256']==sha((OUT/'native.json.gz').read_bytes())and capture['json_sha256']==sha(fixture_raw)
 assert capture['producer_sha256']==sha((ROOT/'scripts/yuyuhakusho/fmpsg-reset-capture.py').read_bytes())
 assert (capture['cases'],capture['events'],capture['frames'],capture['stereo_frames'],capture['projection_nochange_checks_per_core'])==(7,4777,24,17658,520)
 assert capture['cpu_source']['all_pcm_and_recorded_dsp_match']and capture['cpu_source']['frames']==12 and capture['cpu_source']['stereo_frames']==8832
 assert sha((ROOT/capture['cpu_source']['path']).read_bytes())==capture['cpu_source']['sha256']
 wrong=json.loads((OUT/'native-sort-counterexample.json').read_text());assert wrong.get('pass',wrong.get('pass_'))is False and wrong['first_difference']==dict(frame=8,first_byte=56)
 correction_path=ROOT/'games/yuyuhakusho/verification/audio-pcm-command/sort-hypothesis-correction.json';correction=json.loads(correction_path.read_text())
 assert correction['initial_attempt_produced_failure']is False and correction['original_20_cases_sorting_pcm_and_state_equal']is True
 bounds=[];reset_ahead=[];completed_ahead=[]
 for case in fixture['Cases']:
  base=0
  for op in case['Operations']:
   if op['Kind']=='frame':base+=896040;continue
   if op['Kind']=='after-dac':bounds.append(op['CompletedDAC']-op['Cycle']);completed_ahead.append(max(0,op['CompletedDAC']-base-896040))
   if op['Kind']in ['reset','after-dac']:reset_ahead.append(max(0,op['Cycle']-base-896040))
 assert max(bounds)==194 and bounds.count(194)>=3 and 74 in bounds and 104 in bounds
 assert max(reset_ahead)==1008 and max(completed_ahead)==1008
 first=json.loads((OUT/'test-first.json').read_text());extension=json.loads((OUT/'full-ahead-matrix-extension.json').read_text())
 before=OUT/'native-before-full-ahead.json.gz'
 assert sha(before.read_bytes())==first['files']['games/yuyuhakusho/verification/fmpsg-reset/native.json.gz']==extension['original_preproduction_fixture']['sha256']
 assert sha((OUT/'producer-before-full-ahead.py.txt').read_bytes())==extension['pre_extension_producer_sha256']
 assert sha((ROOT/'games/yuyuhakusho/audio/chips/reset.go').read_bytes())==extension['production_sha256']
 assert sha((OUT/'reset-test-before-format.go.txt').read_bytes())==first['files']['games/yuyuhakusho/audio/chips/reset_test.go']
 baseline_path=OUT.parent/'fmpsg-interactive/validation.json';baseline=json.loads(baseline_path.read_text());unchanged=[]
 for previous in baseline['sources']:
  if previous['path'].startswith('games/yuyuhakusho/audio/')and previous['path'].endswith('.go'):
   assert item(ROOT/previous['path'])==previous,('baseline Go DSP changed',previous['path']);unchanged.append(previous)
 assert (OUT/'interactive-before-reset.go.txt').read_bytes()==(ROOT/'games/yuyuhakusho/audio/chips/interactive.go').read_bytes()
 assert (OUT/'ym-before-reset.go.txt').read_bytes()==(ROOT/'games/yuyuhakusho/audio/chips/ym.go').read_bytes()
 checks=dict(native_race=checklog(OUT/'race-final.jsonl'),wasm=checklog(OUT/'wasm-final.jsonl'));assert (OUT/'vet-final.txt').read_text()==''
 sources=set()
 for folder in ['games/yuyuhakusho/audio/chips','games/yuyuhakusho/audio/dsp','games/yuyuhakusho/audio/pcmplayer']:
  sources.update(x for x in (ROOT/folder).rglob('*')if x.is_file()and x.suffix in ['.go','.md'])
 sources.update((ROOT/'scripts/yuyuhakusho').glob('fmpsg*.py'))
 sources.update((ROOT/'docs/research/md-yuyuhakusho').glob('FMPSG*.md'))
 sources.update(ROOT/x for x in ['go.mod','go.sum','scripts/yuyuhakusho/reference.py','scripts/yuyuhakusho/research_audio_loops.py','scripts/yuyuhakusho/extract_audio_assets.py','scripts/yuyuhakusho/inspect_dac_dsp_state.py'])
 artifacts=set(x for x in OUT.rglob('*')if x.is_file()and x!=target);artifacts.add(baseline_path)
 for name in ['sort-hypothesis-correction.json','dsp-sort-chip-tick-red.json','dsp-ordered-chip-tick.json','reset-chip-tick-input-plan.json']:artifacts.add(ROOT/'games/yuyuhakusho/verification/audio-pcm-command'/name)
 for previous in baseline['artifacts']:
  if previous['path'].endswith(('.json.gz','.json')):path=ROOT/previous['path'];assert item(path)==previous;artifacts.add(path)
 artifacts.update((ROOT/'games/yuyuhakusho/audio/pcmplayer/testdata').glob('*.json.gz'))
 env=dict(PATH='/opt/homebrew/bin:/Users/jason/.nvm/versions/node/v24.15.0/bin:/usr/bin:/bin',LANG='C.UTF-8',LC_ALL='C.UTF-8',LC_CTYPE='C.UTF-8',TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'))
 packages=['./games/yuyuhakusho/audio/chips/...','./games/yuyuhakusho/audio/dsp','./games/yuyuhakusho/audio/pcmplayer']
 recorded=[item(x)for x in sorted(sources)]
 result=dict(pass_=True,scope='Ordered warm YM reset domains, bounded causal after-DAC exception, unchanged ordinary/queued contract, full chip/PCM regressions; no independent world timing or physical-hardware maximum claim',native=capture,cpu_input_scope=dict(original_records_data_and_reset=True,original_address_port_times_known=False,replay_address='Explicit reconstructed address at each data cycle, not an observed original address-port time',cpu_equivalence='Original PCM and previously captured native_dsp fields only',full_YM_PSG_FIR_equivalence='New native oracle for declared replay inputs; does not prove arbitrary half-frame address-latch wiring from old data-only Ledger'),true_sort_counterexample=wrong,bounds=dict(actual_cpu_retreat=74,declared_private_retreats=bounds,maximum_profile_retreat=194,max_reset_request_ahead=1008,max_completed_dac_ahead=1008,caller_must_be_actual_device_DAC_causal_transaction=True),checks=checks,vet_pass=True,unchanged_baseline_go_sources=unchanged,baseline_validation=item(baseline_path),sources=recorded,source_manifest_sha256=sha(json.dumps(recorded,separators=(',',':')).encode()),artifacts=[item(x)for x in sorted(artifacts)],commands=dict(native_race=dict(argv=['nice','-n','15','/opt/homebrew/bin/go','test','-race','-json','-count=1',*packages],environment=env),wasm=dict(argv=['nice','-n','15','/opt/homebrew/bin/go','test','-exec','/opt/homebrew/Cellar/go/1.26.2/libexec/lib/wasm/go_js_wasm_exec','-json','-count=1',*packages],environment=dict(env,GOOS='js',GOARCH='wasm')),vet=dict(argv=['nice','-n','15','/opt/homebrew/bin/go','vet',*packages],environment=env)))
 result['pass']=result.pop('pass_');target.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(dict(pass_=True,sources=len(recorded),artifacts=len(artifacts),sha256=sha(target.read_bytes()))))
if __name__=='__main__':main()
