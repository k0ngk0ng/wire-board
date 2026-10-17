"""Original resting-ball support/air flag probes around the Court7 rear edge."""
import copy,hashlib,json
from probe_rest_stall import ROOT,encode,run,SoundMemory,PRG,ROM
from verify_court_profiles import image,profiles

def main():
 out=ROOT/'.local/basketball/rea/court-seven-stall';source=ROOT/'.local/basketball/rea/court-seven/runtime-all-computer-limit-before.json';s=json.loads(source.read_text());grid=profiles(image())[7];rows=[]
 assert hashlib.sha256(ROM).hexdigest()=='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
 for depth in [79*256+218,80*256,80*256+218]:
  for airborne in [False,True]:
   trial=copy.deepcopy(s);b=trial['playing']['world']['Preparation']['Actions']['ball'];b['body']['position']['depth']=depth;b['airborne']=airborne
   after,sounds=run(SoundMemory(PRG,encode(trial)));x=b['body']['position']['x']//256;y=depth//256;support=grid[y//8][x//8]&248
   if support==0:assert after['airborne'] and after['action']==0 and after['body']['gravity']==0
   else:assert support==24 and not after['airborne'] and after['action']==6 and after['body']['velocity']['depth']==-128
   rows.append(dict(before=b,after=after,support=support,cell=grid[y//8][x//8],sounds=sounds))
 result=dict(scope='six isolated original E149 calls at actual and adjacent depth; no gameplay repair',romSHA256=hashlib.sha256(ROM).hexdigest(),runtimeSourceSHA256=hashlib.sha256(source.read_bytes()).hexdigest(),rows=rows)
 (out/'original-support-probes.json').write_text(json.dumps(result,indent=2)+'\n');print('matched original support/airborne boundaries',len(rows))
if __name__=='__main__':main()
