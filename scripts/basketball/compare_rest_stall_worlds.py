"""Compare independent Go and original whole-world traces at the controlled origin.

Input is retained Go diagnostic JSONL plus the original D355 RAM sequence. Zero
page contact scratch is excluded because it no longer belongs to preparation;
projection is presentation and is compared separately by other image oracles.
"""
import argparse,collections,hashlib,json
from pathlib import Path
from capture_reference import ROOT,BASE
from extract_world_loop import loop_state

def normalized(value):
 if isinstance(value,dict):return {k.lower():normalized(v) for k,v in value.items()}
 if isinstance(value,list):return [normalized(v) for v in value]
 return value

def differences(a,b,path=''):
 if isinstance(a,dict) and isinstance(b,dict):
  for k in sorted(a.keys()|b.keys()):yield from differences(a.get(k,0),b.get(k,0),path+'.'+k)
 elif isinstance(a,list) and isinstance(b,list):
  assert len(a)==len(b),(path,len(a),len(b))
  for i,(x,y) in enumerate(zip(a,b)):yield from differences(x,y,path+str(i))
 elif a!=b:yield path,a,b

def main():
 parser=argparse.ArgumentParser();parser.add_argument('--go',default='go-aligned-worlds.jsonl');parser.add_argument('--output',default='aligned-world-comparison.json');parser.add_argument('--directory',default='.local/basketball/rea/ball-rest-stall');parser.add_argument('--source',default='.local/nes-basket/runs/ball-rest-team5-aligned/world-after.ram');parser.add_argument('--initial-updates',type=int,default=1);args=parser.parse_args()
 out=(ROOT/args.directory).resolve();source=(ROOT/args.source).resolve();go=out/args.go
 assert out.is_relative_to(ROOT) and source.is_relative_to(ROOT)
 assert go.resolve().is_relative_to(out.resolve()) and (out/args.output).resolve().is_relative_to(out.resolve())
 raw=source.read_bytes();counts=collections.Counter();first={};strict=0;matches={key:0 for key in ['actors','ball','teamai','input']};index=-1
 for index,line in enumerate(go.open()):
  g=normalized(json.loads(line));s=normalized(loop_state(raw[index*2048:(index+1)*2048],index+args.initial_updates))
  diffs=[d for d in differences(s,g) if not d[0].startswith('.preparation.contacts.candidates') and not d[0].startswith('.projection')]
  if not diffs:strict+=1
  for path,a,b in diffs:
   counts[path]+=1
   if path not in first:first[path]=dict(index=index,source=a,go=b)
  for key in matches:
   if key in ['actors','ball']:a=s['preparation']['actions'][key];b=g['preparation']['actions'][key]
   elif key=='teamai':a=s['preparation']['teamai'];b=g['preparation']['teamai']
   else:a=s['input'];b=g['input']
   if not list(differences(a,b)):matches[key]+=1
 assert len(raw)==(index+1)*2048
 result=dict(scope='independent Go and original complete worlds after one controlled origin; source scratch and projection excluded',worlds=index+1,strictMatches=strict,componentMatches=matches,differences=counts,firstDifferences=first,sourceSHA256=hashlib.sha256(raw).hexdigest(),goSHA256=hashlib.sha256(go.read_bytes()).hexdigest())
 (out/args.output).write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
if __name__=='__main__':main()
