"""Source assets and actual per-video postresult vectors, before implementation."""
import argparse,base64,gzip,hashlib,json,subprocess
from pathlib import Path
from investigate_postresult_scheduler import ROOT,ROM,PRG,ResourceMemory,background_work
from verify_pass_floor_original import MPU
from static_sprites import SpriteROM
from postresult_art import decode_background,palette_groups,PRESETS,BG_PALETTES,SPRITE_PALETTES
from verify_campaign_settlement import progress
OUT=ROOT/'games/basketball/engine/testdata';rom=SpriteROM(ROM)
def u(r,base=0x374):return dict(address=r[base]*256+r[base+1],length=r[base+2],data=list(r[base+3:base+35]))
def palette_cases():
 rows=[]
 for bg in BG_PALETTES:
  for sp in [(2,3),(13,14),(29,30),(49,50),(51,51)]:
   for flags in [0,128,1,64]:
    for level in range(5):
     r=bytearray(2048);r[0x588]=7;r[0x589]=0;r[0x301]=flags;r[0x303]=level*16;r[0x304]=bg;r[0x305:0x307]=bytes(sp);r[0x374:0x397]=bytes([255,1,3,*range(32)])
     m=ResourceMemory(r);m[0x8000:0xc000]=PRG[4*16384:5*16384];c=MPU(memory=m,pc=0x9526);c.sp=255;c.p=0x20;c.stPushWord(0x6fff)
     for _ in range(10000):
      if c.pc==0x961e:break
      c.step()
     else:raise AssertionError('palette queue boundary not reached')
     rows.append(dict(background=bg,sprites=sp,flags=flags,level=level*16,before=u(r),after=u(m)))
 return rows

def generate_tables():
 costs={r['preset']:r['cycles']for r in background_work()}
 lines=['// Code generated from original ROM source resources by verify_postresult_video_sources.py; DO NOT EDIT.','package engine','','var originalPresentationBackgrounds = map[uint8]PresentationBackground{']
 for preset in PRESETS:
  r=decode_background(rom,preset)
  lines.append(f'{preset}:{{Target:{r["target"]},FieldCHR:[2]uint8{{{r["field_banks"][0]},{r["field_banks"][1]}}},PaletteID:{r["palette_preset"]},CompressedCHR:{r["compressed_bank"]},WorkCycles:{costs[preset]},Data:[]uint8{{'+','.join(str(v)for v in r['data'])+'}},')
 lines.append('}')
 for name,ids,sprite in [('originalPresentationBGPalettes',BG_PALETTES,False),('originalPresentationSpritePalettes',SPRITE_PALETTES,True)]:
  count=8 if sprite else 16;lines.append(f'var {name} = map[uint8][{count}]uint8'+'{')
  for ident in ids:lines.append(f'{ident}:{{'+','.join(str(v)for v in palette_groups(rom,ident,sprite))+'},')
  lines.append('}')
 lines.append('var originalPresentationResultFieldCHR = [8]uint8{'+','.join(str(v)for v in PRG[4*16384+0x3ba3:4*16384+0x3bab])+'}')
 return subprocess.check_output(['gofmt'],input=('\n'.join(lines)+'\n').encode())
def videos():
 chains=[]
 for kind in ['reward','ending']:
  p=ROOT/f'.local/nes-basket/runs/postresult-{kind}-observed';events=[json.loads(x)for x in (p/'events.jsonl').open()];raw=(p/'ram.bin').read_bytes();boundaries=(p/'boundaries.ram').read_bytes()
  start=next(e for e in events if e['kind']=='resultEnd' and (e['scene']==7 if kind=='ending' else e['scene']==1 and e['phase']==7))
  end=next(e for e in events if e['kind']=='reset')if kind=='ending'else next(e for e in events if e['kind']=='rewardEnd'and e['scene']==2)
  initial=raw[(start['frame']-25550)*2048:(start['frame']-25549)*2048]
  indexed={}
  for e in events:
   if e['kind']in ('scene1','scene7','schedule','rewardEnd','endingEnd','reset','sound','fadeBegin','fadeEnd','backgroundBegin','backgroundEnd','messageBegin','messageEnd'):
    indexed.setdefault(e['frame'],[]).append(dict(kind=e['kind'],value=e['value'],phase=e['phase'],scene=e['scene'],counter=e['videoCounter']))
  rows=[]
  for frame in range(start['frame']+1,end['frame']+1):
   sample=raw[(frame-25550)*2048:(frame-25549)*2048]
   effects=indexed.get(frame,[])
   rows.append(dict(frame=frame,after=base64.b64encode(sample).decode(),pressed=16 if kind=='ending'and frame==39549 else 0,held=16 if kind=='ending'and frame in(39549,39550)else 0,reset=kind=='ending'and frame==end['frame'],events=effects))
  chains.append(dict(kind=kind,startFrame=start['frame'],initial=base64.b64encode(initial).decode(),progress=progress(initial),rows=rows,eventsSHA256=hashlib.sha256((p/'events.jsonl').read_bytes()).hexdigest(),ramSHA256=hashlib.sha256(raw).hexdigest(),scope='whole-console controlled score/progress injection, continuous per-video RAM; final reset bytes are after boundary and not semantic afterstate'))
 return chains

def main():
 p=argparse.ArgumentParser();p.add_argument('--emit-tables',action='store_true');p.add_argument('--check',action='store_true');p.add_argument('--videos',action='store_true');a=p.parse_args()
 if a.emit_tables:
  data=generate_tables();path=ROOT/'games/basketball/engine/postresult_video_assets_generated.go'
 elif a.videos:
  rows=videos();data=gzip.compress(json.dumps(rows,separators=(',',':')).encode(),mtime=0);path=OUT/'postresult-video.json.gz'
 else:
  rows=dict(palettes=palette_cases(),backgrounds=[decode_background(rom,p)for p in PRESETS],work=background_work());data=gzip.compress(json.dumps(rows,separators=(',',':')).encode(),mtime=0);path=OUT/'postresult-video-assets.json.gz'
 if a.check:assert path.read_bytes()==data
 else:path.write_bytes(data)
 if a.videos:
  meta=dict(romSHA256=hashlib.sha256(ROM).hexdigest(),fixtureSHA256=hashlib.sha256(data).hexdigest(),chains=[dict(kind=c['kind'],startFrame=c['startFrame'],videoFrames=len(c['rows']),eventsSHA256=c['eventsSHA256'],ramSHA256=c['ramSHA256'])for c in rows],scope='actual whole-console per-video source, controlled result score/progress injection; initial after original confirmation tail; final reset after-RAM lies outside controller boundary',omissions='no per-video rows omitted; final reset validates event only, subsequent machine reset instructions are outside component')
  metadata=OUT/'postresult-video-source.json'
 else:
  files=[OUT/'postresult-video-assets.json.gz',ROOT/'games/basketball/engine/postresult_video_assets_generated.go']
  meta=dict(romSHA256=hashlib.sha256(ROM).hexdigest(),files={str(f.relative_to(ROOT)):hashlib.sha256(f.read_bytes()).hexdigest()for f in files if f.exists()},scope='static source resources generated from ROM, never replay fixtures;1900 unchanged9526 palette calls;9 originalDC15..DCB0 decoded outputs and source cycle costs',ranges=['fixed D8C4 descriptor pointers, DB8B base presets','bank3 compressed streams or selected CHR page data','bank4 962F/97AB palettes','fixed DC15..DCB0 disabled upload work','bank4 BBA3 result fieldCHR table; bank3 82ED..8320 teamCHR/palette initialization'],handoffSource=[dict(bank=b,start=f'{start:04X}',endExclusive=f'{end:04X}',hex=PRG[b*16384+start-base:b*16384+end-base].hex()) for b,start,end,base in [(3,0x82ed,0x8320,0x8000),(4,0xbba3,0xbbab,0x8000),(4,0xba40,0xba56,0x8000),(4,0xbcc1,0xbcc5,0x8000),(7,0xc9d7,0xc9e5,0xc000),(7,0xf29d,0xf2d4,0xc000)]],generatorSHA256=hashlib.sha256(Path(__file__).read_bytes()).hexdigest())
  metadata=OUT/'postresult-video-assets-source.json'
 encoded=(json.dumps(meta,indent=2)+'\n').encode()
 if a.check:assert metadata.read_bytes()==encoded
 else:metadata.write_bytes(encoded)
 print(json.dumps(dict(file=str(path.relative_to(ROOT)),sha256=hashlib.sha256(data).hexdigest(),romSHA256=hashlib.sha256(ROM).hexdigest())))
if __name__=='__main__':main()
