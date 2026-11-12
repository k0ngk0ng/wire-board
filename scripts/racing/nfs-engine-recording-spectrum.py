#!/usr/bin/env python3
"""Measure/plot a retained native engine recording without replacing its audio.

Spectral centroid is a pitch-content indicator, not the engine fundamental or
an audibility score. captureStream recordings are not hardware-speaker tests.
"""
import argparse,hashlib,json,re,statistics,subprocess
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()

def main():
 parser=argparse.ArgumentParser(description=__doc__)
 parser.add_argument('--input',type=Path,required=True)
 parser.add_argument('--capture-json',type=Path)
 parser.add_argument('--out',type=Path,required=True)
 parser.add_argument('--windows',nargs='+',default=['1:3','5:7','7.5:8','8.1:8.35','8.35:8.6','8.6:8.9','9:9.4'])
 args=parser.parse_args();source=args.input.resolve();out=args.out.resolve()
 if not out.is_relative_to(ROOT):parser.error('output must stay in this repository')
 out.mkdir(parents=True,exist_ok=True)
 probe=json.loads(subprocess.run(['ffprobe','-v','error','-show_streams','-show_format','-of','json',str(source)],capture_output=True,text=True,check=True).stdout)
 report={'input':str(source.relative_to(ROOT))if source.is_relative_to(ROOT)else str(source),'inputBytes':source.stat().st_size,'inputSha256':sha(source),
  'method':'FFmpeg aspectralstats, Hann/default analysis window2048, overlap0.5; median spectral centroid per selected interval',
  'limitations':['Spectral centroid is not the engine fundamental or a subjective listening judgment.','This recording does not establish physical-speaker or device mute-switch behavior.'],'streamProbe':probe,'windows':[]}
 if args.capture_json:
  evidence=json.loads(args.capture_json.read_text());artifact=next((a for a in evidence.get('audioArtifacts',[])if a['file']==source.name),None)
  if not artifact or artifact['sha256']!=sha(source):raise ValueError('recording is not bound to the supplied capture evidence')
  report['capture']={'evidencePath':str(args.capture_json),'evidenceSha256':sha(args.capture_json),'method':evidence.get('captureMethod'),
                     'artifact':artifact,'errors':evidence.get('captureErrors'),'hardwareMutedByLaunchFlag':evidence.get('hardwareMutedByLaunchFlag')}
 for item in args.windows:
  start,end=map(float,item.split(':'))
  if not 0<=start<end:raise ValueError('invalid spectrum interval')
  filters=f'atrim=start={start}:end={end},aspectralstats=win_size=2048:overlap=0.5,ametadata=print:key=lavfi.aspectralstats.1.centroid'
  result=subprocess.run(['ffmpeg','-hide_banner','-i',str(source),'-af',filters,'-f','null','-'],capture_output=True,text=True,check=True)
  values=[float(x)for x in re.findall(r'lavfi\.aspectralstats\.1\.centroid=([0-9.eE+-]+)',result.stderr)]
  if not values:raise ValueError('interval has no measurable audio')
  report['windows'].append({'start':start,'end':end,'frames':len(values),'centroidMedianHz':statistics.median(values),'centroidMinHz':min(values),'centroidMaxHz':max(values),'filter':filters})
 image=out/'spectrum.png'
 subprocess.run(['ffmpeg','-nostdin','-v','error','-y','-i',str(source),'-lavfi','showspectrumpic=s=1440x520:legend=1:scale=log:fscale=log:color=intensity:win_func=hann','-frames:v','1',str(image)],capture_output=True,check=True)
 report['spectrum']={'path':image.name,'sha256':sha(image)}
 (out/'analysis.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 print(json.dumps({'input':report['input'],'out':str(out.relative_to(ROOT)),'windows':[{k:r[k]for k in ['start','end','centroidMedianHz']}for r in report['windows']]},ensure_ascii=False))

if __name__=='__main__':main()
