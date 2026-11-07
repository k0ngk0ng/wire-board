"""Paired original 400-frame runs for the base roller attribute exception.
Failure inventory: later phase, collision profile15, death/Falcon bit7, OAM,
APU, stale-slot reuse or cross-phase scratch alias reads the copied byte.
Only declared initial setup and scenario-specific original-operation triggers
are shared between variants; the sole variant input is roller Attributes.
"""
from pathlib import Path
import argparse,os,json,gzip,hashlib,shutil
ROOT=Path(__file__).resolve().parents[2];BASE=ROOT/'.local/contra-enemies-base/roller-oracle'
os.environ['CONTRA_REFERENCE_ROOT']=str(BASE)
from capture_reference import run
from enemies_base_capture import FIELDS

HOOK=r'''
local slot=15;local fields=FIELDS;local seed=SEED;local attr=ATTR;local mode="MODE";local seq=0;local setup=false;local guard=false
local reads=assert(io.open(out..'/attribute-reads.jsonl','w'));local phases=assert(io.open(out..'/phases.jsonl','w'));local calls=assert(io.open(out..'/calls.jsonl','w'));local audio=assert(io.open(out..'/audio.jsonl','w'));local dma=assert(io.open(out..'/dma.bin','wb'));local hardware=assert(io.open(out..'/hardware.jsonl','w'));local events=assert(io.open(out..'/interventions.jsonl','w'))
local oam=string.rep(string.char(244),256);local dmaframe=-1
local function read(a)return memory.readbyte(a)end
local function set(a,v)memory.writebyte(a,v)end
local function put(i,values)for j,a in ipairs(fields)do set(a+i,values[j])end end
local function ent(i)local t={};for _,a in ipairs(fields)do t[#t+1]=tostring(read(a+i))end;return '['..table.concat(t,',')..']'end
memory.registerread(0x5a8+slot,1,function(a,n,v)
 if guard then return end;guard=true
 reads:write(string.format('{"frame":%d,"pc":%d,"kind":%d,"routine":%d,"value":%d}\n',emu.framecount(),memory.getregister('pc'),read(0x528+slot),read(0x4b8+slot),read(a)))
 guard=false
end)
memory.registerexec(0xe54f,1,function()
 if not setup then
  setup=true
  for i=0,15 do for _,a in ipairs(fields)do set(a+i,0)end end
  for a=0x368,0x4b7 do set(a,0)end
  put(slot,seed);set(0x5a8+slot,attr)
  set(0x86,1);set(0x6a,0);set(0x87,0);set(0x88,0);set(0x89,0);set(0x8a,0)
  set(0x90,1);set(0x334,128);set(0x31a,176);set(0xae,0);set(0xb0,0);set(0xb4,0);set(0xa0,0);set(0xa4,0);set(0xbc,0)
  set(0x91,3);set(0x39,1)
  if mode~='natural' then set(0x33e+slot,128);set(0x508+slot,0);set(0x518+slot,0)end
  events:write(string.format('{"frame":%d,"event":"controlled-initial-state","attribute":%d}\n',emu.framecount(),attr))
 end
 if seq>=30 and seq<=34 and mode=='shot' then
  set(0x368,30);set(0x388,129);set(0x3b8,read(0x324+slot));set(0x3c8,read(0x33e+slot));set(0x438,1);set(0x448,0)
  events:write(string.format('{\"frame\":%d,\"event\":\"stage-player-projectile-at-roller\"}\n',emu.framecount()))
 end
 if seq==30 and mode=='falcon' then
  local item={1,read(0x31a),read(0x334),5,3,0,0,0,0,0,0,0,0,0,0,0,0,34,128,6,0,0,0,0};put(14,item)
  events:write(string.format('{"frame":%d,"event":"place-falcon-pickup-at-player"}\n',emu.framecount()))
 end
 if seq==220 and mode=='reuse' then
  assert(read(0x4b8+slot)==0,'roller slot not yet free')
  local eye={1,48,128,0,3,0,0,0,0,0,0,16,0,0,1,0,1,2,14,0,16,0,0,0};put(14,eye)
  events:write(string.format('{"frame":%d,"event":"place-eye-to-request-original-child-allocation"}\n',emu.framecount()))
 end
end)
for _,address in ipairs({0xe58a,0xe10a,0xe276,0xe342,0xe949,0xedad})do
 memory.registerexec(address,1,function()
  if read(0x8000)~=0x10 then return end
  local i=memory.getregister('x');if address==0xe342 or address==0xe10a or address==0xe276 then i=read(0x83)end
  if i==slot or address==0xe949 then calls:write(string.format('{"frame":%d,"pc":%d,"slot":%d,"kind":%d,"routine":%d,"hp":%d,"collision":%d}\n',emu.framecount(),address,i,read(0x528+slot),read(0x4b8+slot),read(0x578+slot),read(0x588+slot)))end
 end)
end
memory.registerexec(0xf9bc,1,function()audio:write(string.format('{"frame":%d,"kind":"sound","code":%d}\n',emu.framecount(),memory.getregister('a')))end)
memory.registerexec(0xf9d4,1,function()audio:write(string.format('{"frame":%d,"kind":"reset"}\n',emu.framecount()))end)
memory.registerwrite(0x4014,1,function(a,n,v)oam=memory.readbyterange(v*256,256);dmaframe=emu.framecount()end)
for _,start in ipairs({0x2000,0x4000})do
 local size=start==0x2000 and 8 or 24
 memory.registerwrite(start,size,function(a,n,v)if a==0x4014 then oam=memory.readbyterange(v*256,256);dmaframe=emu.framecount() elseif a~=0x4016 then hardware:write(string.format('[%d,%d,%d]\n',emu.framecount(),a,v))end end)
end
emu.registerafter(function()
 guard=true;dma:write(oam)
 phases:write(string.format('{"frame":%d,"dmaFrame":%d,"enemy":%s,"players":[%d,%d],"score":[%d,%d,%d,%d,%d,%d]}\n',emu.framecount(),dmaframe,ent(slot),read(0x90),read(0x91),read(0x7e0),read(0x7e1),read(0x7e2),read(0x7e3),read(0x7e4),read(0x7e5)))
 guard=false
end)
function control_frame(n,pads)seq=n;if mode=='shot'then pads[1].B=n%4==1 end end
function finish_probe()reads:close();phases:close();calls:close();audio:close();dma:close();hardware:close();events:close()end
'''

def main():
 parser=argparse.ArgumentParser();parser.add_argument('--record',type=int,choices=[140,8586,8715,9174],default=140);parser.add_argument('--mode',choices=['natural','contact','shot','falcon','reuse'],default='natural');parser.add_argument('--attr',type=int,choices=[0,12,19,255],required=True);parser.add_argument('--label',default='v1');args=parser.parse_args()
 sourcepath=ROOT/'docs/research/nes-contra/evidence/enemies-base-4-room.json.gz';source=json.loads(gzip.decompress(sourcepath.read_bytes()));record=next(r for r in source['records']if r['id']==args.record)
 found=[(i,b)for i,(a,b)in enumerate(zip(record['before']['pool'],record['after']['pool']))if a[4]==0 and b[4]!=0 and b[11]==17];assert len(found)==1
 seed=found[0][1]
 (BASE/'states').mkdir(parents=True,exist_ok=True);src=ROOT/'.local/contra-campaign/states/stage-4-v2.fcs';dst=BASE/'states/stage-4.fcs'
 if not dst.exists():shutil.copyfile(src,dst)
 assert src.read_bytes()==dst.read_bytes()
 name=f'roller-{args.record}-{args.mode}-{args.attr}-{args.label}'
 spec=dict(name=name,load_state='stage-4',frames=400,segments=[],captures=[1,40,100,200,400],hooks_lua=HOOK.replace('FIELDS','{'+','.join(map(str,FIELDS))+'}').replace('SEED','{'+','.join(map(str,seed))+'}').replace('ATTR',str(args.attr)).replace('MODE',args.mode),timeout_seconds=120)
 path=BASE/(name+'.json');path.write_text(json.dumps(spec,indent=2)+'\n');out=run(path)
 provenance=dict(source_is_original_execution=True,scope='paired 400-frame complete original execution; only variant input is original roller Attributes; scenario interventions are declared and identical',source=json.loads((out/'source.json').read_text()),scenario=spec,original_record_id=record['id'],original_slot=found[0][0],test_slot=15,mode=args.mode,attribute=args.attr,source_fixture_sha256=hashlib.sha256(sourcepath.read_bytes()).hexdigest())
 (out/'provenance.json').write_text(json.dumps(provenance,indent=2)+'\n');phases=[json.loads(l)for l in(out/'phases.jsonl').read_text().splitlines()]
 print(json.dumps(dict(name=name,phases=sorted({r['enemy'][4]for r in phases if r['enemy'][11]==17}),kinds=sorted({r['enemy'][11]for r in phases}),reads=(out/'attribute-reads.jsonl').read_text().splitlines()[:8],players=sorted({r['players'][0]for r in phases}))))
if __name__=='__main__':main()
