"""Publish reviewed lossless audio once, without overwriting immutable objects.

Default preflight is read-only. --publish uploads, reads back actual CDN bytes
and Range, decodes PCM and writes the pin only after every object passes.
"""
from pathlib import Path
import argparse,concurrent.futures,hashlib,json,os,re,subprocess,sys,urllib.request
ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'scripts'))
import cdn_delivery as cdn

def plan_audio(raw,media):
 catalog=json.loads(raw);digest=hashlib.sha256(raw).hexdigest()
 base='https://cdn.ichenj.com/bga/yuyuhakusho/audio-'+digest
 rows=[];seen=set()
 for entry in [*catalog['music'].values(),*catalog['effects'].values()]:
  name=entry['file']
  if not re.fullmatch(r'[0-9a-f]{2}\.flac',name)or name in seen:raise ValueError('Unsafe/duplicate audio path')
  seen.add(name);file=Path(media)/name
  if file.is_symlink():raise ValueError('Audio symlink not allowed')
  body=file.read_bytes()
  if len(body)!=entry['bytes']or cdn.sha(body)!=entry['sha256']:raise ValueError('Audio source differs: '+name)
  rows.append({'path':name,'url':base+'/'+name,'bytes':len(body),'sha256':entry['sha256'],'mime':'audio/flac','encoding':'','requiresRange':True,'frames':entry['frames'],'pcm_sha256':entry['pcm_sha256']})
 return {'schema':1,'catalogSHA256':digest,'assetsBaseURL':base,'files':len(rows),'bytes':sum(r['bytes']for r in rows)},sorted(rows,key=lambda r:r['path'])

def verify_audio(row):
 def operation():
  request=urllib.request.Request(row['url'],headers={'Origin':'https://bga.ichenj.com','Accept-Encoding':'identity'})
  with urllib.request.urlopen(request,timeout=90)as response:
   data=response.read();cdn.validate_response(row,data,response.headers)
  if data[:4]!=b'fLaC'or data[4]&127!=0 or int.from_bytes(data[5:8],'big')!=34:raise ValueError('Invalid FLAC stream info')
  packed=int.from_bytes(data[18:26],'big')
  actual=(packed>>44,(packed>>41&7)+1,(packed>>36&31)+1,packed&((1<<36)-1))
  if actual!=(44100,2,16,row['frames']):raise ValueError('FLAC sample metadata differs')
  with urllib.request.urlopen(urllib.request.Request(row['url'],headers={'Origin':'https://bga.ichenj.com','Accept-Encoding':'identity','Range':'bytes=0-63'}),timeout=45)as response:
   cdn.validate_range_response(row,data[:64],response.read(),response.headers,response.status)
  pcm=subprocess.check_output(['ffmpeg','-v','error','-i','pipe:0','-f','s16le','-acodec','pcm_s16le','pipe:1'],input=data,cwd=ROOT)
  if len(pcm)!=row['frames']*4 or cdn.sha(pcm)!=row['pcm_sha256']:raise ValueError('Decoded PCM differs')
  return {**row,'decodedPCMVerified':True,'format':{'sampleRate':44100,'channels':2,'bits':16}}
 return cdn.retry_request(operation,row['url'])

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--publish',action='store_true');p.add_argument('--media',type=Path,default=ROOT/'.local/md-yuyuhakusho/playable-audio/media');p.add_argument('--out',type=Path,default=ROOT/'.local/md-yuyuhakusho/deployment/audio-publication.json');a=p.parse_args()
 raw=(ROOT/'games/yuyuhakusho/assets/audio-playback.json').read_bytes();pin,rows=plan_audio(raw,a.media)
 if len(rows)!=143:raise ValueError('Incomplete reviewed audio set')
 a.out.parent.mkdir(parents=True,exist_ok=True);report={'passed':False,'mode':'publish'if a.publish else 'preflight','pin':pin,'files':[{k:v for k,v in r.items()if k not in ['frames','pcm_sha256']}for r in rows]};a.out.write_text(json.dumps(report,indent=2)+'\n')
 if not a.publish:print(json.dumps({'preflight':True,**pin}));return
 cdn.publish(a.media,rows,'stoneage-assets','stoneage-assets.oss-cn-shanghai.aliyuncs.com')
 with concurrent.futures.ThreadPoolExecutor(max_workers=6)as pool:verified=list(pool.map(verify_audio,rows))
 report.update(passed=True,files=verified,allDecodedPCMVerified=True);a.out.write_text(json.dumps(report,indent=2)+'\n')
 (ROOT/'games/yuyuhakusho/assets/audio-release.json').write_text(json.dumps(pin,indent=2)+'\n')
 print(json.dumps({'passed':True,'objects':len(rows),'decoded_pcm_verified':True,'report':str(a.out)}))
if __name__=='__main__':main()
