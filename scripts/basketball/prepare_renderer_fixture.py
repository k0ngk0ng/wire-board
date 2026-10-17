"""Convert original observations into the renderer's semantic test API.
Captured tile IDs are decoded into displayed digits and label glyphs here only.
No OAM, nametable or RAM blob is retained in the browser fixture.
"""
from pathlib import Path
import hashlib,json
from static_sprites import SpriteROM,signed
ROOT=Path(__file__).resolve().parents[2]

def recover_initial_display_ball(snapshot,oam,rom):
 """Recover only the already-visible ball at a loaded-save boundary.
 The saved logical ball has advanced beyond the previously submitted display.
 Match original tile/attribute sequences against static metasprites, never
 expected screenshot pixels. This is a test adapter, not a runtime OAM API.
 """
 if snapshot['reverseSprites']:raise ValueError('Initial inverse fixture expects forward sprite order')
 offset=0
 for object_id in snapshot['drawOrder']:
  obj=snapshot['objects'][object_id]
  if object_id==6:break
  offset+=len(rom.pieces(obj['kind'],obj['pose']&127))
  if obj['airborne'] and not obj['shadowSuppressed']:offset+=1
 else:raise ValueError('No initial visible ball')
 matches=[]
 for pose in range(rom.pose_count(8)):
  pieces=rom.pieces(8,pose)
  if len(pieces)!=4:continue
  for mirror in (0,128):
   projections=[];valid=True
   for i,(tile,attr,x,y) in enumerate(pieces):
    x=signed(x)
    if not mirror:x=-x-8;attr^=64
    py,observed_tile,observed_attr,px=oam[(offset+i)*4:(offset+i+1)*4]
    if tile!=observed_tile or attr!=observed_attr:valid=False;break
    projections.append((px-x,py-signed(y)))
   if valid and len(set(projections))==1:matches.append((pose|mirror,projections[0]))
 if len(matches)!=1:raise ValueError('Initial display ball is not uniquely recovered')
 packed,(x,y)=matches[0];shadow_y,tile,attr,shadow_x=oam[(offset+4)*4:(offset+5)*4]
 expected_dx=-5 if packed&128 else -3
 if tile!=47 or shadow_x!=x+expected_dx:raise ValueError('Initial ball shadow does not bind its display height')
 height=shadow_y-y-16
 snapshot['objects'][6]['pose']=packed
 snapshot['objects'][6]['projected']={'x':x,'y':y}
 snapshot['objects'][6]['position']['height']=height*256
 return {'source':'loaded-save submitted display, inverse static-metasprite match','oam_sha256':hashlib.sha256(oam).hexdigest(),'candidate_count':len(matches),'pose':packed,'projection':[x,y],'integer_height':height,'fractional_physics_height':'not recovered or asserted'}

def prepare():
 out=ROOT/'.local/basketball/renderer/browser';out.mkdir(parents=True,exist_ok=True)
 rom=SpriteROM((ROOT/'.local/nes-basket/basketball.nes').read_bytes());initial_recoveries=[]
 observations=json.loads((ROOT/'.local/basketball/graphics/verification.json').read_text())['frames_detail'];cases=[]
 for name in sorted({i['scenario'] for i in observations}):
  directory=ROOT/'.local/nes-basket/runs'/('renderer-'+name);selected={i['emulator_frame']:i for i in observations if i['scenario']==name};commits=[]
  for file in sorted(directory.glob('*-semantic.json')):
   d=json.loads(file.read_text());commits.extend(d['courtCommits']);frame=int(file.name.split('-')[1])
   if frame not in selected:continue
   item=selected[frame];ppu=file.with_name(file.name.replace('-semantic.json','-ppu.bin')).read_bytes()
   def row(y,a,b):return list(ppu[0x2000+y*32+a:0x2000+y*32+b])
   def digit(tile):
    assert 1<=tile<=10;return tile-1
   def score(start):return sum(digit(tile)*10**(2-i) for i,tile in enumerate(row(3,start,start+3)))
   assert row(1,12,16)==[0xf1,0xf3,0,0] and row(2,12,20)==[0xeb]*8 and row(3,15,17)==[0xfb,0xfc]
   assert row(1,17,18)==[0xf9]
   panels=[]
   for side in range(2):
    start,end,edge=(2,11,1) if side==0 else (21,30,30)
    panels.append({'lines':[row(y,start,end) for y in range(1,4)],'edgeGlyphs':[row(y,edge,edge+1)[0] for y in range(1,4)],'turnMarker':next((x-start for x in range(start,end) if row(0,x,x+1)[0]==0xfe),None)})
   hud={'clock':{'minutes':digit(row(1,16,17)[0]),'seconds':digit(row(1,18,19)[0])*10+digit(row(1,19,20)[0])},'scores':[score(12),score(17)],'panels':panels}
   snapshot={**d['state'],'court':1,'paletteID':2,'camera':d['camera'],'reverseSprites':d['reverseSprites'],'ballCHR':d['ballCHR'],'hud':hud}
   if item['frame']==1:
    recovery=recover_initial_display_ball(snapshot,file.with_name(file.name.replace('-semantic.json','-oam.bin')).read_bytes(),rom);initial_recoveries.append(dict(scenario=name,frame=1,**recovery))
   cases.append({'scenario':name,'frame':item['frame'],'courtCommits':commits,'snapshot':snapshot,'reference':f'/reference/{name}/{item["frame"]:05d}.png'});commits=[]
 doc={'scope':'original renderer inputs; not Go world execution','cases':cases};assert len(cases)==83
 (out/'initial-display-recovery.json').write_text(json.dumps(initial_recoveries,indent=2)+'\n')
 (out/'cases.json').write_text(json.dumps(doc,separators=(',',':'))+'\n')
 return out
if __name__=='__main__':print(prepare())
