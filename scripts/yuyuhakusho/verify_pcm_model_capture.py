"""Acceptance contract for original unfiltered PCM algorithm observations."""
from pathlib import Path
import argparse,hashlib,json
ROOT=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser(description=__doc__);p.add_argument('--capture',type=Path,required=True);p.add_argument('--out',type=Path,required=True);a=p.parse_args()
out=a.out.resolve();assert out.is_relative_to(ROOT)
try:
 capture=a.capture.resolve();assert capture.is_relative_to(ROOT);r=json.loads(capture.read_text())
 assert r['pass'] and r['all_original_observed_frames_equal'] and r['observer_overflows']==0
 assert r['tables']['all_delta_entries_equal'] and r['tables']['all_mix_entries_equal']
 names=set()
 for case in r['cases']:
  assert case['name']not in names;names.add(case['name'])
  assert case['frames_match'] and case['dac_events']>0 and case['bank_reads']>0 and case['decoder_pairs']>0
  for f in case['artifacts']:
   path=ROOT/f['path'];assert hashlib.sha256(path.read_bytes()).hexdigest()==f['sha256']
 assert {'single-short','pair-short','single-mode','pair-long'}.issubset(names)
 result={'pass':True,'scope':'Original unfilteredDAC/value/bank evidence; no independent cadence/filter/synthesis completion','capture_path':str(capture.relative_to(ROOT)),'capture_sha256':hashlib.sha256(capture.read_bytes()).hexdigest(),'cases':len(r['cases']),'verifier_sha256':hashlib.sha256(Path(__file__).read_bytes()).hexdigest()}
 out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
except Exception as error:
 out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps({'pass':False,'error':repr(error),'verifier_sha256':hashlib.sha256(Path(__file__).read_bytes()).hexdigest()},indent=2)+'\n');raise
