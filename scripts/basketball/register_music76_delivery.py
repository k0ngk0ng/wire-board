"""Register only the independently verified76 derivative; no upload or rerip."""
import hashlib,json,shutil
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2];SOURCE=ROOT/'.local/music76-source/assets';TARGET=ROOT/'.local/basketball/audio/export'
def sha(p):return hashlib.file_digest(p.open('rb'),'sha256').hexdigest()
def main():
 p=SOURCE/'music76-delivery-manifest.json';m=json.loads(p.read_text())
 assert m['status']=='offline-asset-verified' and m['id']==76 and m['loop'] is True
 assert m['sample_rate']==44100 and m['loop_end_sample']==m['decoded_samples'] and m['loop_end_sample']-m['loop_start_sample']==m['loop_samples']
 for file,key in [(m['file'],'distribution_sha256'),(m['master'],'master_sha256')]:
  source,target=SOURCE/file,TARGET/file;assert sha(source)==m[key]
  if target.exists():assert sha(target)==m[key],'existing76 asset differs; do not overwrite'
  else:shutil.copyfile(source,target)
 target=TARGET/p.name
 if target.exists():assert target.read_bytes()==p.read_bytes(),'existing76 manifest differs; do not overwrite'
 else:shutil.copyfile(p,target)
 path=ROOT/'games/basketball/assets/audio-playback.json';playback=json.loads(path.read_text())
 entry=dict(wav=m['master'],sha256=m['master_sha256'],loop=True,source_manifest=p.name,file=m['file'],mime_type='audio/mpeg',distribution_sha256=m['distribution_sha256'],encoding_manifest=p.name,processing=m['processing']['method'],accepted_for_release=False,listening_accepted=False)
 for k in ['sample_rate','loop_start_sample','loop_end_sample','loop_samples','decoded_samples']:entry[k]=m[k]
 old=playback['music'].get('76');assert old is None or old==entry,'existing76 playback entry differs'
 playback['music']['76']=entry;path.write_text(json.dumps(playback,indent=2)+'\n')
 ledger_path=ROOT/'games/basketball/assets/court-music-source.json';ledger=json.loads(ledger_path.read_text())
 track=next(t for t in ledger['ending_music']['tracks']if t['id']==76);track.update(entry='phase8 $BD27',runtime_mapping=True,source_manifest=p.name,delivery_file=m['file']);ledger_path.write_text(json.dumps(ledger,indent=2,ensure_ascii=False)+'\n')
 print(json.dumps(dict(id=76,file=m['file'],sha256=m['distribution_sha256'],loopStartSeconds=m['loop_start_sample']/m['sample_rate'])))
if __name__=='__main__':main()
