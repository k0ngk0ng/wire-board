"""Independent acceptance of original PCM dispatch and mixing observations."""
from pathlib import Path
import argparse, hashlib, json, sys

ROOT=Path(__file__).resolve().parents[2]
def sha(raw): return hashlib.sha256(raw).hexdigest()
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--capture',type=Path,required=True)
parser.add_argument('--expect-linear-mix',action='store_true')
parser.add_argument('--out',type=Path,required=True)
args=parser.parse_args()
try:
 path=args.capture.resolve();out=args.out.resolve()
 assert path.is_relative_to(ROOT) and out.is_relative_to(ROOT)
 r=json.loads(path.read_text())
 assert r['pass'] and r['all_original_observed_frames_equal'] and r['cases']
 assert r['identity']['original']['rom_sha256']=='c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
 assert r['identity']['original']['core_sha256']=='1d08b724cf975de17af146f9ba4292946a609b26d03da3d0d9414c90983095da'
 assert r['identity']['observed']['core_sha256']=='e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306'
 assert r['case_count']==len(r['cases'])>=16
 names=set()
 for case in r['cases']:
  assert case['name'] not in names;names.add(case['name'])
  assert case['frames_match'] and case['frames']>0 and case['dispatches']
  assert case['unknown_boundaries']==0
  for item in case['files']:
   file=ROOT/item['path'];assert sha(file.read_bytes())==item['sha256']
 assert {'single-a','single-b','pair-a-b','pair-a-low-drop','pair-b-high','auto-tie','single-mode','stop-primary','stop-secondary','clear-driver','reset-driver','special-f0'}.issubset(names)
 assert r['linear_mix']['compared_stereo_frames']>0
 if args.expect_linear_mix:assert r['linear_mix']['different_samples']==0,'original paired PCM is not the unmodified linear sum of the isolated captures'
 result={'pass':True,'scope':'Retained original dispatch/mix evidence only; not production playback completion','capture_path':str(path.relative_to(ROOT)),'capture_sha256':sha(path.read_bytes()),'cases':r['case_count'],'frames':sum(c['frames']for c in r['cases']),'linear_mix':r['linear_mix'],'verifier_sha256':sha(Path(__file__).read_bytes())}
 out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
except Exception as error:
 out=args.out.resolve();assert out.is_relative_to(ROOT);out.parent.mkdir(parents=True,exist_ok=True)
 out.write_text(json.dumps({'pass':False,'error':repr(error),'capture_path':str(args.capture),'verifier_sha256':sha(Path(__file__).read_bytes())},indent=2)+'\n');raise
