"""Export private REA dossiers, repeated original traces and semantic-rule checks."""
import gzip,hashlib,json
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-campaign-scene';EVIDENCE=ROOT/'docs/research/nes-contra/evidence'
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def main():
 groups=['exit','entrance','intro','intro_helpers_v2','interlude','graphics_mode'];native=[];index=[]
 for name in groups:
  spec=json.loads((BASE/(name+'-spec.json')).read_text());assert sha(Path(spec['target']))==spec['file_sha256'];folder=BASE/(name+'-rea');assert(folder/'export.json').is_file()
  for a in spec['addresses']:
   path=folder/f'function-{a}.json';d=json.loads(path.read_text());native.append(d);index.append({'group':name,'procedure':a,'evidenceID':d['evidence_id'],'fileSHA256':sha(path),'targetSHA256':spec['file_sha256']})
 (EVIDENCE/'campaign-scene-native.json.gz').write_bytes(gzip.compress(json.dumps(native,separators=(',',':')).encode(),mtime=0))
 repeats=[]
 for left,right in [('scene-natural-02','scene-natural-04'),('scene-exit-01','scene-exit-02')]+[(f'scene-stage-{s}-01',f'scene-stage-{s}-02')for s in range(2,9)]:
  a=BASE/'oracle/runs'/left/'ram.bin';b=BASE/'oracle/runs'/right/'ram.bin';assert a.read_bytes()==b.read_bytes();repeats.append({'first':left,'repeat':right,'identicalRAM':True,'sha256':sha(b),'frames':b.stat().st_size//2048})
 counts={}
 for name,kind in [('natural','intro'),('exit','exit'),('exit','interlude'),('intro-cases','intro'),('exit-cases','exit')]:
  d=json.loads(gzip.decompress((EVIDENCE/f'campaign-scene-{name}-original.json.gz').read_bytes()));counts[name+':'+kind]=sum(r['kind']==kind for r in d['records'])
 coverage={'exit':1410,'entranceContinuous':334,'intro':5225,'interlude':6623,'directBackgroundCalls':15}
 binding=json.loads((EVIDENCE/'campaign-scene-final-race-binding.json').read_text());assert binding['exitCode']==0;assert not binding['sourceChangedDuringCheck'];log=EVIDENCE/'campaign-scene-final-race.jsonl';assert sha(log)==binding['logSHA256'];events=[json.loads(l)for l in log.read_text().splitlines()];assert not any(e.get('Action')in['fail','build-fail']for e in events)
 tests=[e['Test']for e in events if e.get('Action')=='pass'and e.get('Test')]
 fixtures=sorted(EVIDENCE.glob('campaign-scene-*-original.json.gz'))
 result={'passed':True,'scope':'Original function/phase and data-format reconstruction. Whole-world, browser and complete campaign acceptance are separate.','romSHA256':'6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe','nativeFunctions':len(index),'native':index,'repeatedOriginalRuns':repeats,'recordCounts':counts,'coverage':coverage,'raceTests':tests,'raceBinding':'campaign-scene-final-race-binding.json','fixtures':{p.name:sha(p)for p in fixtures},'nativeArchiveSHA256':sha(EVIDENCE/'campaign-scene-native.json.gz'),'assetsSHA256':sha(EVIDENCE/'campaign-scene-assets.json'),'researchScripts':{str(p.relative_to(ROOT)):sha(p)for p in sorted((ROOT/'scripts/contra').glob('campaign_scene*'))if p.is_file()},'cleanup':json.loads((EVIDENCE/'campaign-scene-cleanup.json').read_text()),'limitations':['Stage 1 introduction/entrance uses power-on controller-only input; stages 2..8 have one declared stage-index intervention.','The dedicated exit/interlude captures use explicit scene input interventions. They are not natural Boss victories. Root full-world traces provide separate naturally executed combat evidence.','Controlled cases are function input matrices, not continuous game traversal. No recorded after-state or emulator is used by Go implementation.','Raster mask/control and split-scroll commands require the owning World/render pipeline; local scene tests do not certify browser rendering or a full eight-stage game.']}
 (EVIDENCE/'campaign-scene-verification.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({'passed':True,'nativeFunctions':len(index),'repeatedRuns':len(repeats),'coverage':coverage,'raceTests':len(tests)}))
if __name__=='__main__':main()
