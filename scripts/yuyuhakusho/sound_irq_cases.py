"""Construct typed whole-IRQ inputs from frozen independent control profiles."""
from pathlib import Path
import argparse,copy,gzip,json
ROOT=Path(__file__).resolve().parents[2]
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--out',type=Path,required=True);a=p.parse_args();out=a.out.resolve();assert out.is_relative_to(ROOT)and not out.exists()
 base=json.loads(gzip.decompress((ROOT/'games/yuyuhakusho/verification/score-clock-driver-control/native-driver-control-full.json.gz').read_bytes()));rows=[]
 for item in base['cases']:
  row=copy.deepcopy(item['input']);row['kind']='interrupt';row['scheduler']={'Delay':0,'DelayedCode':0,'Backlog':0,'Busy':0};rows.append(row)
 empty=copy.deepcopy(rows[0]);empty['root']['fade']={'steps':0,'remaining':0};empty['root']['selected']=255;empty['queue']=[0,0,0,0]
 for delay in [1,2,65535]:
  row=copy.deepcopy(empty);row['id']=f'delay-{delay}';row['scheduler'].update(Delay=delay,DelayedCode=0x82);rows.append(row)
 for name,code,queue in [('empty-six',6,[0]*4),('empty-f0',240,[0]*4),('full',0x82,[0x83,0x84,0x85,0x86]),('full-six',6,[0x82,0x83,0x84,0x85]),('duplicate',0x82,[0x83,0x84,0x82,0x86])]:
  row=copy.deepcopy(empty);row['id']='delayed-'+name;row['scheduler']['DelayedCode']=code;row['queue']=queue;rows.append(row)
 for busy,backlog in [(1,0),(1,255),(255,1)]:
  row=copy.deepcopy(empty);row['id']=f'busy-{busy}-{backlog}';row['scheduler'].update(Busy=busy,Backlog=backlog,DelayedCode=6);rows.append(row)
 for backlog in [0,1,2,3,255]:
  row=copy.deepcopy(empty);row['id']=f'catchup-{backlog}';row['kind']='catchup';row['scheduler']['Backlog']=backlog;rows.append(row)
 for label in ['command-04-music-82','pause-01-state-1','fade-step-request-a0']:
  source=next(row for row in rows if row['id'].endswith(label));row=copy.deepcopy(source);row['id']='catchup-'+label;row['kind']='catchup';row['scheduler']['Backlog']=2;rows.append(row)
 out.parent.mkdir(parents=True,exist_ok=True);out.write_bytes(gzip.compress((json.dumps({'cases':rows},sort_keys=True,separators=(',',':'))+'\n').encode(),mtime=0));print('sound IRQ profiles',len(rows))
if __name__=='__main__':main()
