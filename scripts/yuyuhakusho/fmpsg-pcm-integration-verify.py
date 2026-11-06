"""Audit two separate full-chip composition layers and unchanged production."""
from pathlib import Path
import argparse,gzip,hashlib,json,re
ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'games/yuyuhakusho/verification/fmpsg-pcm-integration'
PORTS=ROOT/'games/yuyuhakusho/audio/pcmcommands/testdata/original-command-ports-v3.json.gz'
CMD=ROOT/'games/yuyuhakusho/verification/audio-pcm-command-go/validation-ports-v3.json'
BASE=ROOT/'games/yuyuhakusho/verification/fmpsg-reset/validation.json'
FUNCTIONS=['TestPCMFullChipGivenNativePorts','TestPCMFullChipAutonomousDevice']
def sha(raw):return hashlib.sha256(raw).hexdigest()
def entry(path):
 raw=path.read_bytes();return dict(path=str(path.relative_to(ROOT)),bytes=len(raw),sha256=sha(raw))
def logrows(path):return [json.loads(s)for s in path.read_text().splitlines()if s.startswith('{')]
def positive(path):
 rows=logrows(path);assert not any(r.get('Action')in ['fail','build-fail']for r in rows),path
 result={}
 for function in FUNCTIONS:
  parents=[r for r in rows if r.get('Action')=='pass'and r.get('Test')==function]
  children=[r for r in rows if r.get('Action')=='pass'and r.get('Test','').startswith(function+'/')]
  assert len(parents)==1 and len(children)==23,(path,function,len(children))
  result[function]=dict(cases=23,frames=1274,stereo_frames=937609,elapsed_seconds=parents[0].get('Elapsed'))
 return dict(layers=result,log=entry(path))
def output_streams(path):
 rows=logrows(path);assert any(r.get('Action')=='fail'for r in rows)and not any(r.get('Action')=='build-fail'for r in rows),path
 streams={}
 for r in rows:
  if r.get('Action')=='output':streams[r.get('Test','')]=streams.get(r.get('Test',''),'')+r.get('Output','')
 return streams
def difference(a,b,path=''):
 if type(a)!=type(b):return [path]
 if isinstance(a,dict):return sum((difference(a.get(k),b.get(k),path+'.'+k)for k in a.keys()|b.keys()),[])
 if isinstance(a,list):return [path+'.len']if len(a)!=len(b)else sum((difference(x,y,path+'['+str(i)+']')for i,(x,y)in enumerate(zip(a,b))),[])
 return []if a==b else[path]
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--audit',action='store_true');a=p.parse_args();target=OUT/'fmpsg-pcm-integration-validation.json'
 if a.audit:
  frozen=json.loads(target.read_text())
  for item in frozen['sources']+frozen['artifacts']:assert entry(ROOT/item['path'])==item,('frozen file changed',item['path'])
  print(json.dumps(dict(pass_=True,sources=len(frozen['sources']),artifacts=len(frozen['artifacts']),sha256=sha(target.read_bytes()))));return
 assert sha(PORTS.read_bytes())=='e435f0f778088e0bb133082f82720c8ca7ed10c7d13be388faea837d76fff995'
 assert sha(CMD.read_bytes())=='fcbcbcf0e184cfe99dce059ecb210decaa470ab3f32c92e5ecc2f6c32e172c4e'
 command=json.loads(CMD.read_text());assert command['pass']
 for item in command['sources']:assert sha((ROOT/item['path']).read_bytes())==item['sha256'],('frozen Device source changed',item['path'])
 base=json.loads(BASE.read_text());unchanged=[]
 for item in base['sources']:
  path=item['path']
  if path.startswith('games/yuyuhakusho/audio/')and path.endswith('.go'):assert entry(ROOT/path)==item,('existing chip/DSP/PCM production or tests changed',path);unchanged.append(item)
 metadata_path=OUT/'fmpsg-pcm-integration-native-capture.json';native=json.loads(metadata_path.read_text());blob=OUT/'fmpsg-pcm-integration-native.json.gz'
 assert native['pass']and native['gzip_sha256']==sha(blob.read_bytes())and native['json_sha256']==sha(gzip.decompress(blob.read_bytes()))
 assert native['producer_sha256']==sha((ROOT/'scripts/yuyuhakusho/fmpsg-pcm-integration-capture.py').read_bytes())
 assert (native['cases'],native['frames'],native['stereo_frames'],native['physical_ledger_events'])==(23,1274,937609,519256)
 assert not native['address_times_synthesized']and not native['raw_cycles_modified']
 assert native['original']['full_initial_sha256']==native['observed']['full_initial_sha256']
 for mode in ['original','observed']:
  assert native[mode]['observed_quiescent_fm_sidecar']==dict(fm_cycles_start=0,fm_cycles_count=0,fm_cycles_busy=0,fm_ptr_offset_bytes=0)
  assert len(native[mode]['cases'])==23
  for case in native[mode]['cases']:assert case['sidecar_restore']==[]and case['full_initial_sha256']==native[mode]['full_initial_sha256']and case['original_cpu_pcm_and_captured_DSP_equal']and case['actual_original_address_latches_equal']
 negative_data_path=OUT/'fmpsg-pcm-integration-data-only-red.jsonl';streams=output_streams(negative_data_path)
 text=streams['TestPCMFullChipGivenNativePorts/single-reset-between-address-data']
 actual,_=json.JSONDecoder().raw_decode(text[text.index('Go=')+3:]);expected,_=json.JSONDecoder().raw_decode(text[text.index('Native=')+7:])
 assert actual['YM']['Address']==43 and expected['YM']['Address']==42 and difference(actual,expected)==['.YM.Address']
 negative_sort_path=OUT/'fmpsg-pcm-integration-sort-frame-detail-red.jsonl';streams=output_streams(negative_sort_path);text=streams['TestPCMFullChipGivenNativePorts/single-reset-crossing-chip-tick']
 assert 'frame=8 (zero-based)'in text and re.search(r'PCM sample28:',text),text
 checks={name:positive(OUT/('fmpsg-pcm-integration-'+file))for name,file in [('native','native-final.jsonl'),('race','race-final.jsonl'),('wasm','wasm-final.jsonl')]}
 assert (OUT/'fmpsg-pcm-integration-vet-final.txt').read_text()==''
 first=json.loads((OUT/'fmpsg-pcm-integration-test-first.json').read_text())
 assert sha((OUT/'fmpsg-pcm-integration-test-before-format.go.txt').read_bytes())==first['files']['games/yuyuhakusho/audio/chips/fmpsg_pcm_integration_e2e_test.go']
 assert sha((ROOT/'docs/research/md-yuyuhakusho/fmpsg-pcm-integration-failures.md').read_bytes())==first['files']['docs/research/md-yuyuhakusho/fmpsg-pcm-integration-failures.md']
 detail=json.loads((OUT/'fmpsg-pcm-integration-frame-detail.json').read_text());assert sha((OUT/'fmpsg-pcm-integration-test-before-frame-detail.go.txt').read_bytes())==detail['before_test_sha256']
 sources=set()
 for folder in ['games/yuyuhakusho/audio/chips','games/yuyuhakusho/audio/chips/psg','games/yuyuhakusho/audio/dsp','games/yuyuhakusho/audio/pcmcommands','games/yuyuhakusho/audio/phaseclock']:
  sources.update(x for x in (ROOT/folder).glob('*.go')if x.is_file());sources.update(x for x in (ROOT/folder).glob('*.md')if x.is_file())
 sources.update((ROOT/'scripts/yuyuhakusho').glob('fmpsg-pcm-integration-*.py'))
 sources.update((ROOT/'docs/research/md-yuyuhakusho').glob('fmpsg-pcm-integration-*.md'))
 sources.update(ROOT/x for x in ['go.mod','go.sum','scripts/yuyuhakusho/reference.py','scripts/yuyuhakusho/research_audio_loops.py','scripts/yuyuhakusho/extract_audio_assets.py','scripts/yuyuhakusho/inspect_dac_dsp_state.py','scripts/yuyuhakusho/pcm-command-capture.py','scripts/yuyuhakusho/pcm-player-capture.py','scripts/yuyuhakusho/fmpsg-reset-capture.py','scripts/yuyuhakusho/fmpsg_capture_ym.py','scripts/yuyuhakusho/fmpsg_psg_capture.py'])
 artifacts=set(x for x in OUT.rglob('*')if x.is_file()and x!=target);artifacts.update([PORTS,CMD,BASE])
 owner=ROOT/'games/yuyuhakusho/verification/audio-pcm-command'
 for name in ['data-only-address-red.json','actual-address-verified.json','dsp-sort-chip-tick-red.json','dsp-ordered-chip-tick.json','sort-hypothesis-correction.json']:artifacts.add(owner/name)
 correction=json.loads((owner/'sort-hypothesis-correction.json').read_text());assert correction['initial_attempt_produced_failure']is False
 records=[entry(x)for x in sorted(sources)]
 env=dict(PATH='/opt/homebrew/bin:/Users/jason/.nvm/versions/node/v24.15.0/bin:/usr/bin:/bin',LANG='C.UTF-8',LC_ALL='C.UTF-8',LC_CTYPE='C.UTF-8',TEST_TELEMETRY_DIR=str(ROOT/'.local/go-telemetry'),GOCACHE=str(ROOT/'.local/go-cache'),GOMODCACHE=str(ROOT/'.local/go-mod'),GOTMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'),TMPDIR=str(ROOT/'.local/md-yuyuhakusho/tmp'))
 args=['-json','-count=1','-run','^TestPCMFullChip','./games/yuyuhakusho/audio/chips']
 result=dict(pass_=True,scope='Two distinct composition proofs: given original actual ports→full chips; autonomous typed Go PCM Device output→same full chips from matching observed full initial. No production source change, original event-time replay in Device, global world/IRQ schedule or speaker-equivalence claim.',layers=dict(native_oracle=native,go_given_native_ports=dict(cases=23,frames=1274,stereo_frames=937609),go_autonomous_device=dict(cases=23,frames=1274,stereo_frames=937609,full_ledger_events=519256,input='Immutable resources/descriptors + typed accepted commands/holds/DMA only',native_event_times_are_Device_inputs=False)),negative_hypotheses=dict(data_only=dict(case='single-reset-between-address-data',frame=0,actual_address=43,native_address=42,only_complete_state_difference='YM.Address',log=entry(negative_data_path)),raw_sort=dict(case='single-reset-crossing-chip-tick',frame=8,interleaved_sample=28,first_byte=56,log=entry(negative_sort_path)),old20_no_difference_correction_preserved=entry(owner/'sort-hypothesis-correction.json')),checks=checks,vet_pass=True,unchanged_preexisting_audio_go=unchanged,frozen_device_validation=entry(CMD),sources=records,source_manifest_sha256=sha(json.dumps(records,separators=(',',':')).encode()),artifacts=[entry(x)for x in sorted(artifacts)],commands=dict(native_race=dict(argv=['nice','-n','15','/opt/homebrew/bin/go','test','-race',*args],environment=env),wasm=dict(argv=['nice','-n','15','/opt/homebrew/bin/go','test','-exec','/opt/homebrew/Cellar/go/1.26.2/libexec/lib/wasm/go_js_wasm_exec',*args],environment=dict(env,GOOS='js',GOARCH='wasm')),vet=dict(argv=['nice','-n','15','/opt/homebrew/bin/go','vet','./games/yuyuhakusho/audio/chips'],environment=env)))
 result['pass']=result.pop('pass_');target.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(dict(pass_=True,sources=len(records),artifacts=len(artifacts),sha256=sha(target.read_bytes()))))
if __name__=='__main__':main()
