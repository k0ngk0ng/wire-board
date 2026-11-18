SCOPE = "Original single-player PAL Board E2E: direct game-surface input, private saves, whole-party ownership and original image identity."
import argparse
import base64
import gzip
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import struct
import time
import uuid
import urllib.request

parser=argparse.ArgumentParser(description=SCOPE)
entry=parser.add_mutually_exclusive_group()
entry.add_argument('--injury',action='store_true')
entry.add_argument('--companion',action='store_true')
entry.add_argument('--rescue',action='store_true')
entry.add_argument('--stone-key',action='store_true')
entry.add_argument('--medicine',action='store_true',help='development HP9999 checkpoint: use original medicine and restore the original three-role party')
entry.add_argument('--jade-bead',action='store_true',help='development HP9999 checkpoint: original Ling-only bead dialogue, illusion ending and three-role party')
entry.add_argument('--blood-pool',action='store_true',help='development HP9999 checkpoint: original Li-only blood-pool dialogue and three-role reunion')
entry.add_argument('--blood-pool-wave',action='store_true',help='new-core genuine campaign checkpoint: real map-only wave, prediction, reading and private continuation')
entry.add_argument('--earth-orb',action='store_true',help='development HP9999 checkpoint: original five-orb lore after the red ghost king and earth-orb reward')
entry.add_argument('--earth-escape',action='store_true',help='development HP9999 checkpoint: selected original earth orb, actual phone use and original cave escape')
entry.add_argument('--ghost-mountain',action='store_true',help='actual scene-wave campaign: Ling farewell, jade-bead departure, physician rescue and original Li/Yue party')
entry.add_argument('--yangzhou',action='store_true',help='actual scene-wave campaign: acquittal, Yue freed and restored original Li/Yue party')
entry.add_argument('--gaijiao',action='store_true',help='actual scene-wave campaign: Gaijiao rescue after the original automatic Miao fight, Ling taken by the sword saint and the original capital plan')
entry.add_argument('--capital-ritual',action='store_true',help='actual scene-wave campaign: original four-item ritual, Wine Sword Immortal asleep and Yue leaving the party')
entry.add_argument('--caiyi-sacrifice',action='store_true',help='actual scene-wave campaign: Caiyi last wish, original flashbacks, sacrifice and flight to Shu with the original Li/Yue party')
entry.add_argument('--shu-gifts',action='store_true',help='actual scene-wave campaign: original wine master gift/teaching explanation and Yue choosing to enter the tower')
entry.add_argument('--tower-sage',action='store_true',help='actual scene-wave campaign: original book sage joins the bag after the pensive ghost question')
entry.add_argument('--tower-bridges',action='store_true',help='actual scene-wave tower floor with raised party layer: direct touch follows the visible original character')
entry.add_argument('--tower-equipment',action='store_true',help='earned seven-star sword: original equipment role selector, saved swap and field medicine role selector')
entry.add_argument('--tower-demons',action='store_true',help='actual tower king victory: original demon plea, book sage and Ling-led three-role party')
entry.add_argument('--tower-collapse',action='store_true',help='actual seventh dragon victory: desktop/phone original collapse, three cutaways and private cinematic continuation')
entry.add_argument('--anu-promise',action='store_true',help='actual sacred-tree campaign: Anu promise, original joining order and private continuation')
entry.add_argument('--kirin-human',action='store_true',help='actual Dali campaign: Kirin human form, sacred horn, original temporary party and private continuation')
entry.add_argument('--past-queen',action='store_true',help='actual past campaign: original queen staff quest, portrait and private continuation')
entry.add_argument('--lingyue',action='store_true',help='actual old Yuhang: original handkerchief recognition, Lingyue rescue and harbor farewell')
entry.add_argument('--yiru',action='store_true',help='actual sacred medicine: original birth, Yiru naming, sacred medicines and worm quest')
entry.add_argument('--prayer-mother',action='store_true',help='actual goddess mother blessing, sacred gifts and original wardrobe')
entry.add_argument('--prayer-ritual',action='store_true',help='actual selected last water orb, visible original prayer cutaways and private cinematic continuation')
entry.add_argument('--prayer-farewell',action='store_true',help='actual post-earth-beast dialogue and next-region arrival')
entry.add_argument('--ling-recovery',action='store_true',help='actual 36-worm handover: original Ling-only recovery dialogue and Ling/Li/Anu reunion')
entry.add_argument('--water-orb',action='store_true',help='actual old Yuhang: original wooden-sword exchange, present return, Anu and egg shell')
entry.add_argument('--past-sacrifice',action='store_true',help='actual past campaign: queen entrustment, original sacrifice, phoenix ride and private continuation')
entry.add_argument('--hidden-loot',action='store_true',help='original HP checkpoint: native invisible search spot, timed black-text loot notice and save during acquisition on retained old core')
args=parser.parse_args()
wave_candidate=args.blood_pool_wave
args.blood_pool=args.blood_pool or wave_candidate
ROOT=Path(__file__).resolve().parents[3]
mode='shu-gifts' if args.shu_gifts else 'caiyi-sacrifice' if args.caiyi_sacrifice else 'capital-ritual' if args.capital_ritual else 'gaijiao' if args.gaijiao else 'yangzhou' if args.yangzhou else 'ghost-mountain' if args.ghost_mountain else 'blood-pool-wave' if wave_candidate else 'hidden-loot' if args.hidden_loot else 'earth-escape' if args.earth_escape else 'earth-orb' if args.earth_orb else 'blood-pool' if args.blood_pool else 'jade-bead' if args.jade_bead else 'medicine' if args.medicine else 'stone-key' if args.stone_key else 'rescue' if args.rescue else 'companion' if args.companion else 'injury' if args.injury else 'opening'
if args.tower_sage:mode='tower-sage'
if args.tower_bridges:mode='tower-bridges'
if args.tower_equipment:mode='tower-equipment'
if args.tower_demons:mode='tower-demons'
if args.tower_collapse:mode='tower-collapse'
if args.anu_promise:mode='anu-promise'
if args.kirin_human:mode='kirin-human'
if args.past_queen:mode='past-queen'
if args.past_sacrifice:mode='past-sacrifice'
if args.lingyue:mode='lingyue'
if args.water_orb:mode='water-orb'
if args.yiru:mode='yiru'
if args.ling_recovery:mode='ling-recovery'
if args.prayer_mother:mode='prayer-mother'
if args.prayer_ritual:mode='prayer-ritual'
if args.prayer_farewell:mode='prayer-farewell'
item_entry=args.stone_key or args.medicine or args.earth_escape or args.prayer_ritual
equipment_entry=args.tower_equipment
field_entry=args.hidden_loot or args.tower_bridges or equipment_entry
OUT=ROOT/('.local/pal/solo-board-browser' if mode=='opening' else '.local/pal/solo-'+mode+'-browser')
variant=os.environ.get('PAL_BOARD_VARIANT','')
if variant:
    assert (variant=='audio' and mode=='prayer-ritual') or variant=='scene-wave' and mode in ['medicine','jade-bead','blood-pool','blood-pool-wave','earth-orb','earth-escape','ghost-mountain','yangzhou','gaijiao','capital-ritual','caiyi-sacrifice','shu-gifts','tower-sage','tower-bridges','tower-equipment','tower-demons','tower-collapse','anu-promise','kirin-human','past-queen','past-sacrifice','lingyue','water-orb','yiru','ling-recovery','prayer-mother','prayer-ritual','prayer-farewell'],'unsupported candidate browser entry'
    OUT=ROOT/('.local/pal/candidate-'+variant)/OUT.name
visual_cdn=os.environ.get('PAL_VISUAL_CDN')=='1'
if visual_cdn:OUT=OUT.with_name('cdn-'+OUT.name)
BASE=(OUT/'url').read_text().strip()
if visual_cdn:
    try:
        urllib.request.urlopen(BASE+'/pal/v1/index.json',timeout=5)
    except urllib.error.HTTPError as error:
        assert error.code==404
    else:
        raise AssertionError('local visual assets are still served')
ENV=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/pab'),AGENT_BROWSER_SCREENSHOT_DIR=str(OUT),TMPDIR=str(ROOT/'.local/tmp'),AGENT_BROWSER_NO_VERSION_CHECK='1',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
for key in ['AGENT_BROWSER_SOCKET_DIR','TMPDIR']:Path(ENV[key]).mkdir(parents=True,exist_ok=True)
SESSION=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','pc-'+mode if variant else 'pal-solo' if mode=='opening' else 'pal-'+mode],cwd=ROOT,env=ENV,text=True).strip()
REPORT={'passed':False,'scope':SCOPE,'mode':mode,'injury':args.injury,'companion':args.companion,'cases':[]}
REPORT['visualCDNOnlyFixture']=visual_cdn
if variant:REPORT['candidate']=variant
if args.medicine or args.jade_bead or args.blood_pool or args.earth_orb or args.earth_escape or args.ghost_mountain or args.yangzhou or args.gaijiao or args.capital_ritual or args.caiyi_sacrifice or args.shu_gifts or args.tower_sage or args.tower_bridges or equipment_entry or args.tower_demons or args.tower_collapse or args.anu_promise or args.kirin_human or args.past_queen or args.past_sacrifice or args.lingyue or args.water_orb or args.yiru or args.ling_recovery or args.prayer_mother or args.prayer_farewell:REPORT['developmentHealthHP']=9999
if args.prayer_ritual:REPORT['developmentHealthHP']=9999
if args.prayer_ritual:REPORT['physicalShakeAccepted']=False
if args.blood_pool or args.earth_orb:REPORT['waveRenderingAccepted']=False
if mode!='opening':REPORT['entrySHA256']=hashlib.sha256((OUT/'entry-state.json').read_bytes()).hexdigest()
if wave_candidate:
    subprocess.run(['node','--input-type=module','-e',"import {build} from 'esbuild';await build({entryPoints:[process.argv[1]],outfile:process.argv[2],bundle:true,format:'esm',target:'es2022'});",str(ROOT/'games/pal/tests/field-render-probe.ts'),str(OUT/'renderer-probe.js')],cwd=ROOT/'web',env=ENV,check=True)

class Browser:
    def __init__(self,index):
        self.index=index;self.profile=OUT/('profile-'+str(index));self.profile.mkdir(parents=True,exist_ok=True)
        self.env=dict(ENV,AGENT_BROWSER_PROFILE=str(self.profile));self.mobile_process=None
    def b(self,*values):
        r=subprocess.run(['agent-browser','--session',SESSION+'-'+str(self.index),*map(str,values),'--json'],cwd=ROOT,env=self.env,capture_output=True,text=True,timeout=45)
        assert r.returncode==0,r.stdout+r.stderr
        d=json.loads(r.stdout);assert d['success'],d;return d['data']
    def js(self,code):return self.b('eval',code)['result']
    def request(self,path,body=None):
        options='' if body is None else ',{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify('+json.dumps(body)+')}'
        return self.js('(async()=>{const r=await fetch('+json.dumps('/api'+path)+options+');return {status:r.status,body:await r.json()}})()')
    def api(self,path,body=None):
        d=self.request(path,body);assert 200<=d['status']<300,d;return d['body']
    def room(self):return self.api('/state').get('room')
    def data(self):return self.room()['game']['data']
    def command(self,kind):
        r=self.room();return self.api('/rooms/'+r['id'],{'type':kind,'version':r['version'],'nonce':uuid.uuid4().hex})
    def touch(self,x,y,mode='tap',to_x=None,to_y=None):
        r=self.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return {x:r.x,y:r.y,w:r.width,h:r.height}})()')
        px=r['x']+x*r['w']/320;py=r['y']+y*r['h']/200
        qx=r['x']+(x if to_x is None else to_x)*r['w']/320;qy=r['y']+(y if to_y is None else to_y)*r['h']/200
        result=subprocess.run(['node','games/pal/tests/screen-touch.mjs',self.b('get','cdp-url')['cdpUrl'],BASE,mode,*map(str,[px,py,qx,qy])],cwd=ROOT,env=self.env,capture_output=True,text=True,timeout=25)
        assert result.returncode==0,result.stdout+result.stderr
    def tap(self,selector):
        data=self.data() if 'data-pal-action=' in selector or 'data-pal-direction=' in selector else None
        if selector.startswith('[data-pal-') and 'action=' in selector:
            if 'cancel' in selector:
                self.touch(160,100,'hold');return
            if data['phase']=='dialogue':
                self.touch(160,100);return
            items=data.get('menu',{}).get('items',[])
            selected=next((row for row in items if row['value']==data['menu']['selected']),None)
            if selected:self.touch(selected['x']+8,selected['y']+8);return
            cursors=[op for op in visible_ops(data) if op['kind']==1 and op.get('source')==2 and op.get('frame')==69]
            assert cursors,{'no visible original confirmation target':data['menu']}
            cursor=cursors[-1];self.touch(cursor['x']-17,cursor['y']-2);return
        if selector.startswith('[data-pal-direction='):
            direction=selector.split('"')[1]
            if data['phase']=='field':
                dx,dy={'down':(-64,32),'up':(64,-32),'left':(-64,-32),'right':(64,32)}[direction]
                x,y=surface_centre(data);self.touch(x+dx,y+dy);return
            dx,dy={'down':(0,32),'up':(0,-32),'left':(-32,0),'right':(32,0)}[direction]
            self.touch(160,100,'drag',160+dx,100+dy);return
        self.js('document.querySelector('+json.dumps(selector)+').scrollIntoView({block:"center",behavior:"instant"})')
        x,y=self.js('(async()=>{await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));const r=document.querySelector('+json.dumps(selector)+').getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()')
        result=subprocess.run(['node','scripts/browser_touch.mjs',self.b('get','cdp-url')['cdpUrl'],BASE,'tap',*map(str,[x,y,x,y])],cwd=ROOT,env=self.env,capture_output=True,text=True,timeout=25)
        assert result.returncode==0,result.stdout+result.stderr
    def mobile(self):
        self.b('set','viewport',320,820)
        self.mobile_process=subprocess.Popen(['node','scripts/browser_device.mjs',self.b('get','cdp-url')['cdpUrl'],BASE,'320','820','--keep'],cwd=ROOT,env=self.env,stdout=subprocess.PIPE,text=True)
        assert self.mobile_process.stdout.readline().strip()=='ready'

def wait(check,label,seconds=60):
    end=time.monotonic()+seconds
    while time.monotonic()<end:
        if check():return
        time.sleep(.12)
    raise AssertionError(label)

def record(d):
    REPORT['cases'].append(d);print(json.dumps(d,ensure_ascii=False),flush=True)

def ground_direction(data):
    directions=[('down',(-16,8),(0,1)),('up',(16,-8),(0,-1)),('left',(-16,-8),(-1,0)),('right',(16,8),(1,0))]
    if not data.get('field'):
        assert not wave_candidate,'new-core actual blood-pool wave still disables prediction'
        assert (args.blood_pool or args.earth_orb) and data['scene']==60 and data['phase']=='field'
        ground=gzip.decompress((ROOT/'.local/pal/visual/pal/v1/maps/76.bin.gz').read_bytes())
        def open_cell(row):
            x,y=data['x']+row[1][0],data['y']+row[1][1]
            index=(y//16*64+x//32)*2+x%32//16
            return not struct.unpack_from('<I',ground,index*4)[0]&8192
        return next(row for row in directions if open_cell(row))
    cells=base64.b64decode(data['field']['cells'])
    return next(row for row in directions if cells[(row[2][1]+8)*17+row[2][0]+8]==0)

def surface_centre(data):
    if not data.get('field'):return data['x']-data['cameraX'],data['y']-data['cameraY']-14
    actor=next(a for a in data['field']['actors'] if a[0]==0 and a[1]==data['party'][0]['role'])
    atlas=json.loads((ROOT/'.local/pal/visual/pal/v1/atlases/mgo'/f'{actor[2]}.json').read_text())
    frame=atlas['frames'][actor[5]]
    return actor[3]-data['cameraX'],actor[4]-data['cameraY']+4-frame['height']/2

def visible_ops(data):
    raster=data.get('raster')
    if not raster:return data.get('overlay',[])
    ops=[];seen=set()
    def visit(index):
        if index<=0 or index>len(raster['nodes']) or index in seen:return
        seen.add(index);n=raster['nodes'][index-1]
        visit(n[1]);visit(n[2])
        if n[0]==2:ops.append({'kind':0,'id':n[5],'color':n[8],'x':n[6],'y':n[7]})
        if n[0]==1:ops.append({'kind':1,'source':n[5],'id':n[6],'frame':n[7],'x':n[8],'y':n[9]})
    visit(raster['root'])
    return ops

def visible_text(data):
    return ''.join(chr(op['id']) for op in visible_ops(data) if op['kind']==0 and op.get('color',0)!=0)

def native_row(data,y,left,right):
    letters={op['x']:chr(op['id']) for op in visible_ops(data) if op['kind']==0 and op.get('color') and op['y']==y and left<=op['x']<right}
    return ''.join(letters[x] for x in sorted(letters))

def native_number(data,y,left,right):
    digits={op['x']:op['frame']-56 for op in visible_ops(data) if op['kind']==1 and op.get('source')==2 and op.get('id')==0 and 56<=op.get('frame',0)<=65 and op['y']==y and left<=op['x']<=right}
    return int(''.join(str(digits[x]) for x in sorted(digits))) if digits else None

def selected_original_role(data):
    glyphs={(op['x'],op['y']):op for op in visible_ops(data) if op['kind']==0 and op.get('color')}
    for member in data['party']:
        name=member['name']
        for p in glyphs.values():
            if not (249<=p['color']<=254 or p['color']==28):continue
            if all(glyphs.get((p['x']+i*16,p['y']),{}).get('id')==ord(c) for i,c in enumerate(name)):return name
    return None

def native_item_position(data,name):
    glyphs={(op['x'],op['y']):op['id'] for op in visible_ops(data) if op['kind']==0 and op.get('color')}
    return next(((x,y) for x,y in glyphs if x in (15,115,215) and 12<=y<=120 and all(glyphs.get((x+i*16,y))==ord(c) for i,c in enumerate(name))),None)

def collapse_sequence(player,phone):
    label='phone' if phone else 'desktop'
    if phone:player.mobile()
    player.b('click','[data-fullscreen-enter]');player.b('wait','[data-room-fullscreen]')
    canvas=player.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.width,r.height]})()')
    expected=[320,200] if phone else [1280,800]
    assert all(abs(a-b)<2 for a,b in zip(canvas,expected)),canvas
    assert not player.js('!!document.querySelector("[data-pal-controls] button")')
    initial=player.data();assert initial['scene']==145 and initial['dialogue']['text']=='七根龙柱全部毁去'
    pid=initial['dialogue']['id']
    if phone:player.touch(160,100)
    else:player.b('click','[data-pal-board] canvas')
    wait(lambda:player.data().get('dialogue',{}).get('id')!=pid,'collapse reading did not advance')
    seen={};brightness={};pixels={};saved=False;end=time.monotonic()+100
    while time.monotonic()<end:
        current=player.data();scene=current['scene']
        assert current['cash']==initial['cash'] and all(p['controller']==0 for p in current['party'])
        if scene in [145,150,151,152]:
            sample=player.js('(async()=>{const c=document.querySelector("[data-pal-board] canvas"),a=c.getContext("2d").getImageData(0,0,c.width,c.height).data,h=await crypto.subtle.digest("SHA-256",a);let sum=0;for(let i=0;i<a.length;i+=4)sum+=a[i]+a[i+1]+a[i+2];return {hash:[...new Uint8Array(h)].map(b=>b.toString(16).padStart(2,"0")).join(""),brightness:sum/(a.length/4*3)}})()')
            pixels.setdefault(str(scene),set()).add(sample['hash'])
            if sample['brightness']>=12 and sample['brightness']>brightness.get(scene,0)+6:
                seen[scene]=current
                brightness[scene]=sample['brightness']
                (OUT/f'{label}-scene-{scene}-view.json').write_text(json.dumps(current,ensure_ascii=False)+'\n')
                player.b('screenshot',OUT/f'{label}-scene-{scene}.png')
            if phone and scene==151 and scene in seen and not saved:
                player.tap('[data-room-fullscreen] [data-save-game]');wait(lambda:not player.room(),'phone cinematic save did not release owner')
                frozen=json.loads((OUT/'frozen-view.json').read_text())
                assert frozen['scene']==151,'cinematic save missed its actual scene'
                player.b('wait','[data-resume-save]');player.tap('[data-resume-save]')
                wait(lambda:player.room() and not player.room().get('loading'),'phone cinematic save did not resume',80)
                restored=json.loads((OUT/'restored-boundary-view.json').read_text())
                normalize=lambda d:{k:v for k,v in d.items() if k not in ['epoch','seq','appliedSeq','discardedSeq','controlAt']}
                assert normalize(frozen)==normalize(restored),'cinematic save changed the original scene/pixels/script boundary'
                (OUT/'phone-cinematic-frozen.json').write_text(json.dumps(frozen,ensure_ascii=False)+'\n')
                (OUT/'phone-cinematic-restored.json').write_text(json.dumps(restored,ensure_ascii=False)+'\n')
                player.b('click','[data-fullscreen-enter]');player.b('wait','[data-room-fullscreen]');saved=True
        if scene==174:
            assert current['map']==20 and [p['role'] for p in current['party']]==[0]
            break
        time.sleep(.06)
    else:raise AssertionError('original tower collapse/cutaways did not reach the next dream entry')
    assert all(scene in seen for scene in [145,150,151,152]),seen.keys()
    assert len(pixels['145'])>1,'original tower collapse did not visibly animate'
    assert not phone or saved,'phone cinematic continuation was not exercised'
    player.b('screenshot',OUT/f'{label}-next-entry.png')
    player.tap('[data-room-fullscreen] [data-save-game]');wait(lambda:not player.room(),'next-entry personal save did not release owner')
    (OUT/f'{label}-pixel-hashes.json').write_text(json.dumps({k:sorted(v) for k,v in pixels.items()},indent=2)+'\n')
    record({'check':label+' original tower collapse, three visible cutaways, next dream entry and private save; no extra reading after the seventh-pillar notice','passed':True,'canvas':canvas,'scenes':sorted(seen),'pixelFrameCounts':{k:len(v) for k,v in pixels.items()},'brightness':brightness,'cinematicSave':saved})

def prayer_sequence(player,phone):
    label='phone' if phone else 'desktop'
    if phone:player.mobile()
    player.b('click','[data-fullscreen-enter]');player.b('wait','[data-room-fullscreen]')
    canvas=player.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.width,r.height]})()')
    expected=[320,200] if phone else [1280,800]
    assert all(abs(a-b)<2 for a,b in zip(canvas,expected)),canvas
    assert not player.js('!!document.querySelector("[data-pal-controls] button")')
    initial=player.data();assert initial['scene']==242 and initial['menu']['kind']==5
    point=native_item_position(initial,'水灵珠');assert point
    if phone:player.touch(point[0]+8,point[1]+8)
    else:
        player.js('document.querySelector("[data-pal-board] canvas").focus();true')
        player.b('press','Enter')
    wait(lambda:player.data()['phase']!='menu','original last water orb was not used')
    seen={};brightness={};pixels={};saved=False;pages=0;end=time.monotonic()+210
    while time.monotonic()<end:
        room=player.room()
        if room.get('loading'):
            time.sleep(.06)
            continue
        current=room['game']['data'];scene=current['scene']
        assert current['cash']==initial['cash'] and all(p['controller']==0 for p in current['party'])
        if scene in [228,231,244,245,246,232]:
            sample=player.js('(async()=>{const c=document.querySelector("[data-pal-board] canvas"),a=c.getContext("2d").getImageData(0,0,c.width,c.height).data,h=await crypto.subtle.digest("SHA-256",a);let sum=0;for(let i=0;i<a.length;i+=4)sum+=a[i]+a[i+1]+a[i+2];return {hash:[...new Uint8Array(h)].map(b=>b.toString(16).padStart(2,"0")).join(""),brightness:sum/(a.length/4*3)}})()')
            pixels.setdefault(str(scene),set()).add(sample['hash'])
            if sample['brightness']>=8 and sample['brightness']>brightness.get(scene,0)+5:
                seen[scene]=current;brightness[scene]=sample['brightness']
                (OUT/f'{label}-scene-{scene}-view.json').write_text(json.dumps(current,ensure_ascii=False)+'\n')
                player.b('screenshot',OUT/f'{label}-scene-{scene}.png')
            if phone and scene==244 and scene in seen and not saved:
                player.tap('[data-room-fullscreen] [data-save-game]');wait(lambda:not player.room(),'phone prayer cinematic save did not release owner')
                frozen=json.loads((OUT/'frozen-view.json').read_text());assert frozen['scene']==244
                player.b('wait','[data-resume-save]');player.tap('[data-resume-save]')
                wait(lambda:player.room() and not player.room().get('loading'),'phone prayer cinematic save did not resume',80)
                restored=json.loads((OUT/'restored-boundary-view.json').read_text())
                normalize=lambda d:{k:v for k,v in d.items() if k not in ['epoch','seq','appliedSeq','discardedSeq','controlAt']}
                assert normalize(frozen)==normalize(restored),'prayer cinematic frozen boundary changed'
                (OUT/'phone-prayer-frozen.json').write_text(json.dumps(frozen,ensure_ascii=False)+'\n')
                (OUT/'phone-prayer-restored.json').write_text(json.dumps(restored,ensure_ascii=False)+'\n')
                player.b('click','[data-fullscreen-enter]');player.b('wait','[data-room-fullscreen]');saved=True
                continue
        if scene==232 and current.get('dialogue') and '我代全族的人向您致谢' in current['dialogue']['text']:
            assert [p['role'] for p in current['party']]==[1,0,4]
            break
        if current.get('dialogue'):
            prompt=current['dialogue']['id']
            wait(lambda:player.js('document.querySelector("[data-pal-board] .pal-sr-only")?.textContent==='+json.dumps(current['dialogue']['text'])),'prayer dialogue did not reach the visible Board')
            player.js('new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))')
            if phone:player.touch(160,100)
            else:player.b('press','Enter')
            wait(lambda:player.data().get('dialogue',{}).get('id')!=prompt,'original prayer reading did not advance')
            pages+=1
        time.sleep(.06)
    else:raise AssertionError('original prayer cutaways did not reach the tribe thanks')
    assert all(scene in seen for scene in [228,231,244,245,246,232]),seen.keys()
    assert any(len(values)>1 for values in pixels.values()),'prayer cutaways never visibly changed'
    assert not phone or saved,'phone prayer cinematic continuation was not exercised'
    player.b('screenshot',OUT/f'{label}-tribe-thanks.png')
    player.tap('[data-room-fullscreen] [data-save-game]');wait(lambda:not player.room(),'prayer endpoint save did not release owner')
    (OUT/f'{label}-pixel-hashes.json').write_text(json.dumps({k:sorted(v) for k,v in pixels.items()},indent=2)+'\n')
    record({'check':label+' original last-orb input, six visible prayer/disaster/thanks scenes and private cinematic save; physical earthquake presentation is separately unaccepted','passed':True,'canvas':canvas,'scenes':sorted(seen),'pixelFrameCounts':{k:len(v) for k,v in pixels.items()},'brightness':brightness,'cinematicSave':saved,'pages':pages})

players=[Browser(0),Browser(1)]
try:
    a,stranger=players
    for p in players:
        p.b('open',BASE);p.b('set','viewport',1280,900)
        p.api('/register',{'name':'单人验收'+uuid.uuid4().hex[:7],'password':'pal-solo-test-123','invite':'test-invite'})
    room=a.api('/rooms',{'kind':'pal','name':'单人原版续档','capacity':1})
    a.command('ready');a.command('start');a.b('reload')
    wait(lambda:a.room() and not a.room().get('loading'),'single canvas did not become ready',80)
    wait(lambda:a.data()['phase']==('menu' if item_entry or equipment_entry else 'field' if field_entry else 'dialogue'),'original entry did not arrive',80)
    a.b('snapshot','-i')
    initial=a.data();original_room=a.room()
    assert len(original_room['seats'])==1
    if not item_entry and not field_entry:assert initial['dialogue']['read']==[False]
    assert all(p['controller']==0 for p in initial['party'])
    if args.companion:
        assert initial['scene']==40 and [p['role'] for p in initial['party']]==[0]
        assert '已经是伙伴了' in initial['dialogue']['text']
    if args.jade_bead:
        assert initial['scene']==59 and [p['role'] for p in initial['party']]==[1]
        assert '以后你就叫做' in initial['dialogue']['text']
    if args.blood_pool:
        assert initial['scene']==60 and [p['role'] for p in initial['party']]==[0]
        assert '赤鬼王所在的血池' in initial['dialogue']['text']
    if args.earth_orb:
        assert initial['scene']==60 and [p['role'] for p in initial['party']]==[0,1,2]
        assert '女娲大神聚天地灵气' in initial['dialogue']['text']
    if args.ghost_mountain:
        assert variant=='scene-wave' and initial['scene']==68 and [p['role'] for p in initial['party']]==[0]
        assert '我就跟你们走' in initial['dialogue']['text']
    if args.yangzhou:
        assert variant=='scene-wave' and initial['scene']==82 and [p['role'] for p in initial['party']]==[0]
        assert '你们可以自由离去了' in initial['dialogue']['text']
    if args.gaijiao:
        assert variant=='scene-wave' and initial['scene']==110 and [p['role'] for p in initial['party']]==[0,2]
        assert '那位老道士自称是独孤剑圣' in initial['dialogue']['text']
    if args.capital_ritual:
        assert variant=='scene-wave' and initial['scene']==116 and [p['role'] for p in initial['party']]==[0]
        assert '千万不要离开我七步以外' in initial['dialogue']['text']
    if args.caiyi_sacrifice:
        assert variant=='scene-wave' and initial['scene']==141 and [p['role'] for p in initial['party']]==[0]
        assert '请诸位永远不要让相公知道' in initial['dialogue']['text']
    if args.shu_gifts:
        assert variant=='scene-wave' and initial['scene']==159 and [p['role'] for p in initial['party']]==[0]
        assert '一生之' in initial['dialogue']['text'] and '只能使用九次' in initial['dialogue']['text']
    if args.tower_sage:
        assert variant=='scene-wave' and initial['scene']==155 and [p['role'] for p in initial['party']]==[0,2]
        assert '你们带我走' in initial['dialogue']['text']
    if args.tower_bridges:
        assert variant=='scene-wave' and initial['scene']==157 and initial['map']==156 and [p['role'] for p in initial['party']]==[0,2]
        assert initial['field']['layer']==48
    if args.tower_demons:
        assert variant=='scene-wave' and initial['scene']==154 and '请您帮助我们离开这地方吧' in initial['dialogue']['text']
    if args.tower_collapse:
        assert variant=='scene-wave' and initial['scene']==145 and initial['dialogue']['text']=='七根龙柱全部毁去'
    if args.anu_promise:
        assert variant=='scene-wave' and initial['scene']==189 and [p['role'] for p in initial['party']]==[0]
        assert '一言为定，跟我走' in initial['dialogue']['text']
    if args.kirin_human:
        assert variant=='scene-wave' and initial['scene']==202 and initial['map']==187 and [p['role'] for p in initial['party']]==[0]
        assert '它变成人了耶' in initial['dialogue']['text']
    if args.past_queen:
        assert variant=='scene-wave' and initial['scene']==248 and [p['role'] for p in initial['party']]==[0]
        assert '请你先替我取回我的' in initial['dialogue']['text'] and '天蛇杖' in initial['dialogue']['text']
    if args.past_sacrifice:
        assert variant=='scene-wave' and initial['scene']==251 and [p['role'] for p in initial['party']]==[3,0]
        assert '请你能代我好好照顾她' in initial['dialogue']['text']
        assert initial['dialogue']['face']==73
    if args.lingyue:
        assert variant=='scene-wave' and initial['scene']==274 and [p['role'] for p in initial['party']]==[0]
        assert '你怎么会有这条手绢' in initial['dialogue']['text'] and initial['dialogue']['face']==0
    if args.water_orb:
        assert variant=='scene-wave' and initial['scene']==267 and [p['role'] for p in initial['party']]==[0]
        assert '好～交换' in initial['dialogue']['text'] and initial['dialogue']['face']==85
    if args.yiru:
        assert variant=='scene-wave' and initial['scene']==173 and [p['role'] for p in initial['party']]==[0]
        assert '李忆如' in initial['dialogue']['text'] and initial['dialogue']['face']==6
    if args.ling_recovery:
        assert variant=='scene-wave' and initial['scene']==175 and [p['role'] for p in initial['party']]==[1]
        assert '你不多休养几天再走吗' in initial['dialogue']['text'] and initial['dialogue']['face']==82
    if args.prayer_mother:
        assert variant=='scene-wave' and initial['scene']==234 and [p['role'] for p in initial['party']]==[0]
        assert '我的孩子～辛苦你了' in initial['dialogue']['text'] and initial['dialogue']['face']==73
    if args.prayer_farewell:
        assert variant=='scene-wave' and initial['scene']==232 and [p['role'] for p in initial['party']]==[1,0,4]
        assert '我们去揪出这幕后的主使者' in initial['dialogue']['text'] and initial['dialogue']['face']==27
    if args.prayer_ritual:
        assert variant in ['scene-wave','audio'] and initial['scene']==242 and [p['role'] for p in initial['party']]==[1,0,4]
        assert initial['menu']['kind']==5 and '水灵珠' in visible_text(initial)
    if equipment_entry:
        assert variant=='scene-wave' and initial['scene']==146 and initial['map']==74 and initial['menu']['kind']==6
        assert [p['role'] for p in initial['party']]==[0,2] and selected_original_role(initial)=='李逍遥'
        assert native_row(initial,77,130,226)=='铁剑'
        assert any(op['kind']==1 and op.get('source')==5 and op.get('id')==57 for op in visible_ops(initial))
    if args.hidden_loot:
        assert initial['scene']==2 and [p['role'] for p in initial['party']]==[0] and initial['cash']==0
        assert not any(actor[0]==1 and actor[1]==5 for actor in initial['field']['actors']),'invisible original search spot became a visible actor'
    if (args.rescue or item_entry) and not args.prayer_ritual:
        assert initial['scene']==(60 if args.earth_escape else 54 if args.medicine else 48) and [p['role'] for p in initial['party']]==([0,1,2] if args.earth_escape else [0,2])
    if args.rescue:assert '女孩子们救出去' in initial['dialogue']['text']
    if args.stone_key:assert initial['menu']['kind']==5 and '石钥匙' in visible_text(initial)
    if args.medicine:assert initial['menu']['kind']==5 and '六神丹' in visible_text(initial)
    if args.earth_escape:assert initial['menu']['kind']==5 and '土灵珠' in visible_text(initial)
    assert not a.js('!!document.querySelector(".pal-play-status,[data-pal-panel-action]")'),'solo UI still shows shared reading/leader controls'
    assert not a.js('!!document.querySelector("[data-pal-controls]")?.getClientRects().length'),'desktop shows virtual controls'
    face=None if item_entry or field_entry else initial['dialogue']['face']
    item_ball=1 if args.prayer_ritual else 6 if args.earth_escape else 236 if args.medicine else 237 if args.stone_key else None
    if item_entry:
        assert any(op['kind']==1 and op['source']==5 and op['id']==item_ball for op in visible_ops(initial)),'original selected item has a stale BALL image binding'
    elif not field_entry and not args.tower_collapse and not args.lingyue:assert face>0 and any(op['kind']==1 and op['source']==1 and op['id']==face for op in initial['overlay']),'original RGM portrait has a stale background binding'
    if not equipment_entry:assert not any(op['kind']==1 and op['source']==4 for op in initial['overlay']),'a full FBP was projected as the portrait'
    a.b('screenshot',OUT/'desktop.png','--full')
    record({'check':'single seat, whole-party owner, solo UI, desktop without virtual buttons and correct original image identity','passed':True,'face':face,'stoneKeyBALL':237 if args.stone_key else None,'medicineBALL':236 if args.medicine else None,'roles':[p['role'] for p in initial['party']]})
    a.b('click','[data-save-game]')
    wait(lambda:not a.room(),'personal save did not release the only seat')
    if variant:
        frozen=json.loads((OUT/'frozen-view.json').read_text())
        for key in ['scene','map','x','y','cash','party','dialogue','menu']:
            assert frozen.get(key)==initial.get(key),{'changed before the save boundary':key}
        initial=frozen
    assert stranger.api('/state')['saves']==[],'stranger discovered a private solo save'
    assert stranger.request('/saves/'+room['id']+'/resume',{})['status'] in [403,404],'stranger resumed the personal save'
    a.b('wait','[data-resume-save]');a.b('click','[data-resume-save]')
    wait(lambda:a.room() and a.room()['status']=='playing' and not a.room().get('loading'),'personal continue waited for another account',80)
    resumed=a.data()
    restored_boundary=json.loads((OUT/'restored-boundary-view.json').read_text()) if variant else resumed
    if variant:
        (OUT/'initial-restored-boundary-view.json').write_text(json.dumps(restored_boundary,ensure_ascii=False)+'\n')
        for key in ['scene','map','x','y','cash','party','dialogue','menu']:
            assert resumed.get(key)==restored_boundary.get(key),{'changed after continue before user input':key}
    (OUT/'initial-view.json').write_text(json.dumps(initial,ensure_ascii=False)+'\n')
    (OUT/'resumed-view.json').write_text(json.dumps(resumed,ensure_ascii=False)+'\n')
    for key in ['scene','map','x','y','cash','party','dialogue','menu','draws','overlay','raster']:
        before_value,after_value=initial.get(key),restored_boundary.get(key)
        if item_entry and key=='overlay' and not variant:
            def selection_phase(ops):
                return [dict(op,color=249) if op['kind']==0 and 249<=op.get('color',0)<=254 else op for op in ops]
            before_value,after_value=selection_phase(before_value),selection_phase(after_value)
        assert after_value==before_value,{'changed':key}
    assert resumed['epoch']!=initial['epoch'] and a.room().get('matchId')==original_room.get('matchId')
    assert not a.js('!!document.querySelector("[data-saved-restoring]")'),'solo continue shows waiting-for-teammates UI'
    record({'check':'one-account save and immediate private continue; original identity, scene, party and reading retained','passed':True})
    if args.prayer_ritual:
        prayer_sequence(a,False)
        stranger.api('/rooms',{'kind':'pal','name':'手机祈雨续档','capacity':1})
        stranger.command('ready');stranger.command('start');stranger.b('reload')
        wait(lambda:stranger.room() and not stranger.room().get('loading') and stranger.data()['menu']['kind']==5,'phone prayer entry did not become ready',80)
        prayer_sequence(stranger,True)
        REPORT['passed']=True
        raise SystemExit(0)
    if args.tower_collapse:
        collapse_sequence(a,False)
        room=stranger.api('/rooms',{'kind':'pal','name':'手机塔毁续档','capacity':1})
        stranger.command('ready');stranger.command('start');stranger.b('reload')
        wait(lambda:stranger.room() and not stranger.room().get('loading') and stranger.data()['phase']=='dialogue','phone collapse entry did not become ready',80)
        collapse_sequence(stranger,True)
        REPORT['passed']=True
        raise SystemExit(0)
    if args.caiyi_sacrifice or args.anu_promise or args.kirin_human or args.past_queen or args.past_sacrifice or args.lingyue or args.water_orb or args.yiru or args.ling_recovery or args.prayer_mother or args.prayer_farewell:
        pid=a.data()['dialogue']['id'];a.b('click','[data-pal-board] canvas')
        wait(lambda:a.data().get('dialogue',{}).get('id')!=pid,'desktop game-surface click did not advance the original page')
        record({'check':'native desktop game-surface click advances one original dialogue page','passed':True})
    a.mobile();a.b('click','[data-fullscreen-enter]');a.b('wait','[data-room-fullscreen]')
    assert not a.js('!![...document.querySelectorAll("[data-pal-controls] button")].find(b=>b.getClientRects().length)'), 'phone still shows virtual reading/direction/action buttons instead of direct game-surface input'
    if args.caiyi_sacrifice or args.anu_promise or args.kirin_human or args.past_queen or args.past_sacrifice or args.lingyue or args.water_orb or args.yiru or args.ling_recovery or args.prayer_mother or args.prayer_farewell:
        wait(lambda:a.data()['phase']=='dialogue','next original phone reading did not arrive')
        pid=a.data()['dialogue']['id']
        a.touch(100,80,'cancel',150,110)
        assert a.data()['dialogue']['id']==pid,'cancelled touch advanced the original page'
        a.touch(140,80,'pinch',140,80)
        assert a.data()['dialogue']['id']==pid,'pinch gesture advanced the original page'
        record({'check':'cancelled touch and pinch do not advance dialogue; phone has no virtual reading, direction or action buttons','passed':True})
    if not item_entry and not field_entry or args.earth_escape:
        geometry=a.js('''(()=>{const r=document.querySelector('[data-pal-board] canvas').getBoundingClientRect();return {canvas:{x:r.x,y:r.y,w:r.width,h:r.height},buttons:[...document.querySelectorAll('[data-pal-controls] button')].filter(b=>b.getClientRects().length).map(b=>{const q=b.getBoundingClientRect();return {label:b.textContent,x:q.x,y:q.y,w:q.width,h:q.height}})}})()''')
        canvas=geometry['canvas'];overlaps=[]
        for op in visible_ops(a.data()):
            if op['kind']!=0 or not op.get('color') or not op.get('id'):continue
            x=canvas['x']+op['x']*canvas['w']/320;y=canvas['y']+op['y']*canvas['h']/200
            w=16*canvas['w']/320;h=16*canvas['h']/200
            for button in geometry['buttons']:
                if min(x+w,button['x']+button['w'])>max(x,button['x']) and min(y+h,button['y']+button['h'])>max(y,button['y']):overlaps.append({'glyph':chr(op['id']),'button':button['label']})
        assert not overlaps,{'phone reading text covered by overlay controls':overlaps}
        record({'check':'phone controls do not cover original visible dialogue or inventory glyphs','passed':True,'dialoguePosition':initial.get('dialogue',{}).get('position'),'menuKind':initial['menu']['kind'],'geometry':geometry})
    a.b('screenshot',OUT/'phone-dialogue.png')
    pages=0
    if args.hidden_loot:
        notice=lambda:''.join(chr(op['id']) for op in visible_ops(a.data()) if op['kind']==0)
        a.tap('[data-pal-action="confirm"]')
        wait(lambda:'得到止血草' in notice(),'native phone search did not reveal the original hidden item')
        a.tap('[data-room-fullscreen] [data-save-game]')
        wait(lambda:not a.room(),'saving during timed hidden-item acquisition did not release owner')
        a.b('wait','[data-resume-save]');a.tap('[data-resume-save]')
        wait(lambda:a.room() and not a.room().get('loading'),'hidden-item timed save did not resume',80)
        a.b('click','[data-fullscreen-enter]');a.b('wait','[data-room-fullscreen]')
        wait(lambda:'得到止血草' in notice(),'timed original black-text loot window lost on resume')
        a.b('screenshot',OUT/'phone-loot-notice.png')
        wait(lambda:a.data()['phase']=='field','original timed loot notice did not finish',20)
        for _ in range(3):
            a.tap('[data-pal-action="confirm"]')
            assert a.data()['phase']=='field' and '得到止血草' not in notice(),'resumed hidden item can be repeatedly collected'
        record({'check':'native invisible search, original black-text timed loot popup, personal save during acquisition on retained core and repeated search after resume','passed':True,'event':5,'item':'止血草'})
    elif args.medicine:
        a.tap('[data-pal-action="confirm"]')
        wait(lambda:a.data()['phase']!='menu','native phone did not use the selected original medicine')
    elif args.earth_escape:
        selected=lambda:''.join(chr(op['id']) for op in visible_ops(a.data()) if op['kind']==0 and 249<=op.get('color',0)<=254)
        assert '土灵珠' in selected(),'original earth orb is not the active menu selection'
        a.tap('[data-pal-direction="up"]')
        wait(lambda:'土灵珠' not in selected(),'native phone inventory cursor did not move up')
        a.tap('[data-pal-direction="down"]')
        wait(lambda:'土灵珠' in selected(),'native phone inventory cursor did not return to the original earth orb')
        a.tap('[data-pal-action="confirm"]')
        wait(lambda:a.data()['phase']=='field' and a.data()['scene']==65,'native phone did not execute original earth-orb cave escape',100)
    elif args.stone_key:
        a.tap('[data-pal-action="confirm"]')
        wait(lambda:a.data()['phase']=='field' and any(actor[0]==1 and actor[1]==758 and actor[5]>=3 for actor in (a.data().get('field') or {}).get('actors',[])),'original stone door did not open')
        a.b('screenshot',OUT/'phone-open-door.png')
        a.tap('[data-pal-action="cancel"]')
        wait(lambda:a.data()['menu']['kind']==1,'phone did not open the original main menu')
        for _ in range(4):
            if a.data()['menu']['selected']==3:break
            old=a.data()['menu']['selected'];a.tap('[data-pal-direction="down"]')
            wait(lambda:a.data()['menu']['selected']!=old,'phone main menu cursor did not move')
        assert a.data()['menu']['selected']==3
        a.tap('[data-pal-action="confirm"]')
        wait(lambda:a.data()['menu']['kind']==4,'phone inventory submenu did not open')
        if a.data()['menu']['selected']!=2:
            a.tap('[data-pal-direction="down"]')
            wait(lambda:a.data()['menu']['selected']==2,'phone did not select original use-item entry')
        a.tap('[data-pal-action="confirm"]')
        wait(lambda:a.data()['menu']['kind']==5,'phone did not reopen the original item list')
        assert '石钥匙' not in visible_text(a.data()),'native touch did not consume the original stone key'
        a.b('screenshot',OUT/'phone-key-consumed.png')
        a.tap('[data-pal-action="cancel"]')
        wait(lambda:a.data()['phase']=='field','phone did not close the original inventory')
        for _ in range(2):
            before=a.data();a.tap('[data-pal-direction="left"]')
            wait(lambda:a.data()['scene']!=before['scene'] or [a.data()['x'],a.data()['y']]!=[before['x'],before['y']],'phone could not walk through the opened stone door')
        wait(lambda:a.data()['scene']==49 and a.data()['phase']=='dialogue','original rescued-girls scene did not arrive',100)
        record({'check':'saved selected stone key, native touch consumes it, original door animation opens and ordinary touch movement exits into the captive rescue scene','passed':True})
    elif equipment_entry:
        a.touch(23,134)
        wait(lambda:selected_original_role(a.data())=='林月如','direct touch on the original gray equipment role did not move its visible highlight')
        inactive_weapon=native_row(a.data(),77,130,226)
        before=a.data();a.touch(23,134)
        wait(lambda:a.data()['seq']>before['seq'] and a.data()['seq']==a.data()['appliedSeq'],'inactive original equipment confirmation did not settle')
        assert native_row(a.data(),77,130,226)==inactive_weapon and selected_original_role(a.data())=='林月如'
        a.touch(23,116)
        wait(lambda:selected_original_role(a.data())=='李逍遥','direct touch did not return original equipment focus to Li')
        assert native_row(a.data(),77,130,226)=='铁剑'
        before_stats=[native_number(a.data(),y,260,278) for y in [14,36,58,80,102]]
        a.touch(23,116)
        wait(lambda:native_row(a.data(),77,130,226)=='七星剑','native phone did not equip the earned original sword')
        after_stats=[native_number(a.data(),y,260,278) for y in [14,36,58,80,102]]
        assert [after-before for before,after in zip(before_stats,after_stats)]==[110,50,4,32,33],(before_stats,after_stats)
        a.b('screenshot',OUT/'phone-sword-equipped.png')
        a.tap('[data-room-fullscreen] [data-save-game]');wait(lambda:not a.room(),'phone equipped sword save did not release owner')
        a.b('wait','[data-resume-save]');a.tap('[data-resume-save]')
        wait(lambda:a.room() and not a.room().get('loading'),'equipped original sword did not resume',80)
        assert native_row(a.data(),77,130,226)=='七星剑' and selected_original_role(a.data())=='李逍遥'
        record({'check':'native phone original equipment roles select and confirm, inactive Yue cannot equip, actual five sword attributes and equipped save are preserved','passed':True,'beforeStats':before_stats,'afterStats':after_stats})
        a.b('click','[data-fullscreen-enter]');a.b('wait','[data-room-fullscreen]')
        for _ in range(3):
            if a.data()['phase']=='field':break
            a.touch(160,100,'hold')
            wait(lambda:a.data()['phase']=='field' or a.data()['menu']['kind']==5,'phone could not leave equipment panel')
        assert a.data()['phase']=='field'
        x,y=surface_centre(a.data());a.touch(x,y,'hold');wait(lambda:a.data()['menu']['kind']==1,'original main menu did not open after phone equipment')
        for kind,value,next_kind in [(1,3,4),(4,2,5)]:
            q=next(row for row in a.data()['menu']['items'] if row['value']==value)
            a.touch(q['x']+8,q['y']+8)
            wait(lambda:a.data()['menu']['selected']==value or a.data()['menu']['kind']==next_kind,'original inventory entry was not selected')
            if a.data()['menu']['kind']==kind:a.touch(q['x']+8,q['y']+8)
            wait(lambda:a.data()['menu']['kind']==next_kind,'original inventory entry did not open')
        for _ in range(40):
            point=native_item_position(a.data(),'止血草')
            if point:break
            a.tap('[data-pal-direction="up"]')
        assert point,'original healing herb was not found through phone inventory navigation'
        a.touch(point[0]+8,point[1]+8)
        wait(lambda:'止血草' in ''.join(chr(op['id']) for op in visible_ops(a.data()) if op['kind']==0 and 249<=op.get('color',0)<=254),'healing herb did not become the original selection')
        a.touch(point[0]+8,point[1]+8)
        wait(lambda:a.data()['menu']['kind']==7,'original field medicine role selector did not open')
        selection_hp=[p['hp'] for p in a.data()['party']]
        if selected_original_role(a.data())=='林月如':
            a.touch(133,24);wait(lambda:selected_original_role(a.data())=='李逍遥','phone field medicine selector could not change its previous saved role')
        a.touch(133,44);wait(lambda:selected_original_role(a.data())=='林月如','phone field medicine target did not select Yue')
        assert [p['hp'] for p in a.data()['party']]==selection_hp,'selecting a different medicine target executed it before confirmation'
        before_hp=[p['hp'] for p in a.data()['party']]
        a.tap('[data-room-fullscreen] [data-save-game]');wait(lambda:not a.room(),'field medicine target save did not release owner')
        a.b('wait','[data-resume-save]');a.tap('[data-resume-save]')
        wait(lambda:a.room() and not a.room().get('loading'),'saved original medicine target did not resume',80)
        assert a.data()['menu']['kind']==7 and selected_original_role(a.data())=='林月如'
        a.b('click','[data-fullscreen-enter]');a.b('wait','[data-room-fullscreen]');a.touch(133,44)
        wait(lambda:a.data()['party'][1]['hp']>before_hp[1],'phone original medicine did not heal the selected saved target')
        assert [p['hp'] for p in a.data()['party']]==[before_hp[0],before_hp[1]+50]
        a.b('screenshot',OUT/'phone-medicine-target.png')
        for _ in range(3):
            if a.data()['phase']=='field':break
            a.touch(160,100,'hold')
            wait(lambda:a.data()['phase']=='field' or a.data()['menu']['kind']==5,'phone could not return through the original field medicine menus')
        assert a.data()['phase']=='field'
        record({'check':'saved original field medicine target heals Yue by 50 without changing Li; long press follows the original menu levels back to the field','passed':True,'beforeHP':before_hp,'afterHP':[p['hp'] for p in a.data()['party']]})
    elif args.tower_bridges:
        record({'check':'original raised tower floor resumes with layer 48 and a visible manually owned Li/Yue party','passed':True,'layer':a.data()['field']['layer']})
    else:
        old=initial['dialogue']['id'];a.tap('[data-pal-action="confirm"]')
        wait(lambda:a.data().get('dialogue',{}).get('id')!=old,'native solo touch did not advance reading')
        pages=1
    page_limit=120 if args.tower_demons else 60
    phoenix_steps=0
    while pages<page_limit:
        wait(lambda:a.data()['phase'] in ['dialogue','field'],'next original solo reading/field',100)
        current=a.data()
        if args.past_sacrifice and [p['role'] for p in current['party']]==[3] and not REPORT.get('soloQueenSaved'):
            a.tap('[data-room-fullscreen] [data-save-game]')
            wait(lambda:not a.room(),'solo queen cinematic save did not release the owner')
            frozen=json.loads((OUT/'frozen-view.json').read_text())
            assert [p['role'] for p in frozen['party']]==[3] and frozen['phase']=='dialogue'
            a.b('wait','[data-resume-save]');a.tap('[data-resume-save]')
            wait(lambda:a.room() and not a.room().get('loading'),'solo queen cinematic save did not continue',80)
            restored=json.loads((OUT/'restored-boundary-view.json').read_text())
            transient={'epoch','seq','appliedSeq','discardedSeq','controlAt'}
            assert {k:v for k,v in frozen.items() if k not in transient}=={k:v for k,v in restored.items() if k not in transient},'solo queen cinematic continuation changed'
            assert all(p['controller']==0 for p in restored['party']) and restored['epoch']!=frozen['epoch']
            (OUT/'phone-solo-queen-frozen.json').write_text(json.dumps(frozen,ensure_ascii=False)+'\n')
            (OUT/'phone-solo-queen-restored.json').write_text(json.dumps(restored,ensure_ascii=False)+'\n')
            a.b('click','[data-fullscreen-enter]');a.b('wait','[data-room-fullscreen]')
            a.b('screenshot',OUT/'phone-solo-queen.png')
            REPORT['soloQueenSaved']=True
            record({'check':'phone private save during original solo-queen sacrifice dialogue preserves the full frozen public boundary, owner and script continuation','passed':True})
            continue
        if args.water_orb and current['scene']==204 and current['phase']=='dialogue' and not REPORT.get('presentReturnSaved'):
            a.tap('[data-room-fullscreen] [data-save-game]')
            wait(lambda:not a.room(),'present-return save did not release the owner')
            frozen=json.loads((OUT/'frozen-view.json').read_text())
            assert frozen['scene']==204 and frozen['phase']=='dialogue'
            a.b('wait','[data-resume-save]');a.tap('[data-resume-save]')
            wait(lambda:a.room() and not a.room().get('loading'),'present-return save did not continue',80)
            restored=json.loads((OUT/'restored-boundary-view.json').read_text())
            transient={'epoch','seq','appliedSeq','discardedSeq','controlAt'}
            assert {k:v for k,v in frozen.items() if k not in transient}=={k:v for k,v in restored.items() if k not in transient},'present-return continuation changed'
            assert all(p['controller']==0 for p in restored['party']) and restored['epoch']!=frozen['epoch']
            (OUT/'phone-present-frozen.json').write_text(json.dumps(frozen,ensure_ascii=False)+'\n')
            (OUT/'phone-present-restored.json').write_text(json.dumps(restored,ensure_ascii=False)+'\n')
            a.b('click','[data-fullscreen-enter]');a.b('wait','[data-room-fullscreen]')
            a.b('screenshot',OUT/'phone-present-save.png')
            REPORT['presentReturnSaved']=True
            record({'check':'phone private save during original present-return dialogue preserves the full frozen public boundary, owner and continuation','passed':True})
            continue
        if current['phase']=='field':
            if args.past_sacrifice and current['scene']==252:
                phoenix_steps+=1;assert phoenix_steps<=80,'phone could not approach the original phoenix'
                actor=next((row for row in current['field']['actors'] if row[0]==1 and row[1]==4439),None)
                if not actor:
                    wait(lambda:a.data()['phase']!='field' or any(row[0]==1 and row[1]==4439 for row in a.data()['field']['actors']),'original phoenix did not arrive')
                    continue
                choices=[]
                for name,(dx,dy),(u,z) in [('down',(-16,8),(0,1)),('left',(-16,-8),(-1,0)),('up',(16,-8),(0,-1)),('right',(16,8),(1,0))]:
                    flags=current['field']['cells'][(z+8)*17+u+8]
                    if flags&1 and not flags&2:continue
                    score=abs(current['x']+dx-actor[3])+abs(current['y']+dy-actor[4])*2
                    choices.append((score,dx,dy))
                assert choices,'original phoenix approach has no ordinary move'
                _,dx,dy=min(choices);x,y=surface_centre(current);a.touch(x+dx*4,y+dy*4)
                wait(lambda:a.data()['scene']!=252 or a.data()['phase']!='field' or a.data()['seq']>current['seq'],'phone phoenix approach was not applied')
                continue
            break
        pid=current['dialogue']['id'];assert current['dialogue']['read']==[False]
        a.tap('[data-pal-action="confirm"]')
        wait(lambda:a.data().get('dialogue',{}).get('id')!=pid,'solo original page did not advance',100)
        pages+=1
    final=a.data();assert final['phase']=='field' and all(p['controller']==0 for p in final['party'])
    if args.injury:
        assert [p['role'] for p in final['party']]==[0,1] and final['field']['actors'][0][2]==2
        assert final['cash']==initial['cash'] and final['party'][0]['mp']==final['party'][0]['maxMP']
    elif args.companion:
        assert final['scene']==40 and [p['role'] for p in final['party']]==[2,0]
        assert final['cash']==initial['cash']
    elif args.jade_bead:
        assert final['scene']==58 and [p['role'] for p in final['party']]==[0,1,2]
        assert final['cash']==initial['cash']
        record({'check':'native phone reading resumes the original Ling-only bead scene, ends the temple illusion and restores the manually owned three-role party','passed':True,'pages':pages})
    elif args.blood_pool:
        assert final['scene']==60 and [p['role'] for p in final['party']]==[0,1,2]
        assert final['cash']==initial['cash']
        record({'check':'native phone resumes the original Li-only blood-pool dialogue after the general/fall and restores the manually owned three-role party','passed':True,'pages':pages})
        if wave_candidate:
            assert final.get('field') and final['draws'][0]['kind']==2 and final['draws'][0]['a']==2
    elif args.earth_orb:
        assert final['scene']==60 and [p['role'] for p in final['party']]==[0,1,2]
        assert final['cash']==initial['cash']
        record({'check':'native phone resumes original five-orb lore and Ling dialogue after the red ghost king; whole-party manual ownership and reward cash remain unchanged','passed':True,'pages':pages})
    elif args.earth_escape:
        assert final['scene']==65 and final['map']==54 and [p['role'] for p in final['party']]==[0,1,2]
        assert final['cash']==initial['cash'] and final['party']==initial['party']
        record({'check':'native phone uses the saved selected original earth orb and executes actual cave escape; original party and cash preserved','passed':True,'earthOrbBALL':6})
    elif args.ghost_mountain:
        assert final['scene']==78 and final['map']==24 and [p['role'] for p in final['party']]==[0,2]
        assert final['cash']==initial['cash'] and final['party'][0]['hp']>0 and pages>=10
        record({'check':'native phone continues original Ling farewell, jade-bead departure and physician rescue, then restores manually owned Li/Yue party in the original resting room','passed':True,'pages':pages})
    elif args.yangzhou:
        assert final['scene']==82 and final['map']==84 and [p['role'] for p in final['party']]==[0,2]
        assert final['cash']==initial['cash'] and pages>=4
        record({'check':'native phone continues original acquittal and Yue release, then restores the manually owned Li/Yue party in Yangzhou court without repeating rewards','passed':True,'pages':pages})
    elif args.gaijiao:
        assert final['scene']==110 and final['map']==101 and [p['role'] for p in final['party']]==[2,0]
        assert final['cash']==initial['cash'] and pages>=6
        record({'check':'native phone continues original Gaijiao rescue, Ling taken by the sword saint and capital plan; restores the original manually owned Yue/Li order without repeating rewards','passed':True,'pages':pages})
    elif args.capital_ritual:
        assert final['scene']==116 and final['map']==106 and [p['role'] for p in final['party']]==[0]
        assert final['cash']==initial['cash'] and pages>=6
        record({'check':'native phone continues the original manor ritual, master falling asleep and Yue departing; Li remains the sole manually owned original party member without repeating ritual payment','passed':True,'pages':pages})
    elif args.caiyi_sacrifice:
        assert final['scene']==156 and final['map']==132 and [p['role'] for p in final['party']]==[0,2]
        assert final['cash']==initial['cash'] and pages>=20
        record({'check':'native phone continues Caiyi last wish, original sacrifice/flashbacks, reflection and flight to Shu; restores the manually owned Li/Yue party without repeating rewards','passed':True,'pages':pages})
    elif args.shu_gifts:
        assert final['scene']==159 and final['map']==134 and [p['role'] for p in final['party']]==[0,2]
        assert final['cash']==initial['cash'] and pages>=4
        record({'check':'native phone game-surface taps continue the original wine-god limitation and teaching/gift explanation; Yue chooses to enter the tower and rejoins the manually owned party','passed':True,'pages':pages})
    elif args.tower_sage:
        assert final['scene']==155 and final['map']==147 and [p['role'] for p in final['party']]==[0,2]
        assert final['cash']==initial['cash'] and pages>=2
        record({'check':'native phone game-surface taps continue the original sage agreement and bag entry, preserve the manually owned Li/Yue party and do not repeat cash rewards','passed':True,'pages':pages})
    elif args.tower_demons:
        assert final['scene']==153 and [p['role'] for p in final['party']]==[1,0,2]
        assert final['cash']==initial['cash'] and pages>=4
        record({'check':'native phone taps continue original demons and sage after the king, then restore the original Ling-led manually owned three-role party without repeating rewards','passed':True,'pages':pages})
    elif args.anu_promise:
        assert final['scene']==189 and final['map']==153 and [p['role'] for p in final['party']]==[4,0]
        assert final['cash']==initial['cash'] and pages>=2
        record({'check':'native phone taps continue original Anu promise and tree-cave request, then restore the original Anu/Li order under the sole player; personal save preserves story and cash','passed':True,'pages':pages})
    elif args.kirin_human:
        assert final['scene']==202 and final['map']==187 and [p['role'] for p in final['party']]==[4,0]
        assert final['cash']==initial['cash'] and pages>=20
        record({'check':'native phone taps continue original Kirin human form, Ling lineage, sacred horn and temple instruction; Anu/Li rejoin under the sole player without repeating victory rewards','passed':True,'pages':pages})
    elif args.past_queen:
        assert final['scene']==248 and final['map']==184 and [p['role'] for p in final['party']]==[0]
        assert final['cash']==initial['cash'] and pages>=3
        record({'check':'native phone taps continue original queen staff quest and stone-elder password, preserve solo Li and private continuation without premature staff grant','passed':True,'pages':pages})
    elif args.past_sacrifice:
        assert final['scene']==267 and final['map']==5 and [p['role'] for p in final['party']]==[0]
        assert final['cash']==initial['cash'] and pages>=3 and REPORT.get('soloQueenSaved')
        record({'check':'native phone taps continue queen entrustment, solo-queen sacrifice, departure and original phoenix ride to old Yuhang; Li remains under the sole account without replaying battle rewards','passed':True,'pages':pages,'phoenixApproachInputs':phoenix_steps})
    elif args.lingyue:
        assert final['scene']==274 and final['map']==196 and [p['role'] for p in final['party']]==[0]
        assert final['cash']==initial['cash'] and pages>=20
        record({'check':'native phone taps continue original handkerchief recognition, Lingyue appearance, grandmother healing, Ling apprenticeship and harbor farewell; sole Li ownership and private continuation preserved','passed':True,'pages':pages})
    elif args.water_orb:
        assert final['scene']==204 and final['map']==175 and [p['role'] for p in final['party']]==[0,4]
        assert final['cash']==initial['cash'] and pages>=10 and REPORT.get('presentReturnSaved')
        record({'check':'native phone taps continue original wooden-sword water-orb exchange and present return, Anu reunion and egg-shell handover; original Li/Anu order and private continuation preserved','passed':True,'pages':pages})
    elif args.yiru:
        assert final['scene']==173 and final['map']==202 and [p['role'] for p in final['party']]==[0]
        assert final['cash']==initial['cash'] and pages>=10
        record({'check':'native phone taps continue original Yiru naming, Yue remembrance, Anu holding the child and the 36-worm quest; solo Li and original private continuation preserved','passed':True,'pages':pages})
    elif args.ling_recovery:
        assert final['scene']==175 and final['map']==202 and [p['role'] for p in final['party']]==[1,0,4]
        assert final['cash']==initial['cash'] and pages>=6
        record({'check':'native phone taps continue original Ling recovery, mother visit and sage entrustment, then restore the Ling-led three-role party under the sole account; private save preserves the initial Ling-only party','passed':True,'pages':pages})
    elif args.prayer_mother:
        assert final['scene']==235 and final['map']==175 and [p['role'] for p in final['party']]==[1,0,4]
        assert final['cash']==initial['cash'] and pages>=2
        assert any(actor[0]==0 and actor[1]==1 and actor[2]==38 for actor in final['field']['actors'])
        record({'check':'native phone taps continue original mother blessing and sacred gifts, then restore the original Ling/Li/Anu order and new Ling wardrobe; temporary hidden Li and private continuation preserved','passed':True,'pages':pages})
    elif args.prayer_farewell:
        assert final['scene']==292 and final['map']==190 and [p['role'] for p in final['party']]==[1,0,4]
        assert final['cash']==initial['cash'] and pages>=3
        record({'check':'native phone taps continue original post-earth-beast decision and actual arrival at the next cave region; original party, private continuation and earned reward cash preserved','passed':True,'pages':pages})
    elif args.tower_bridges:
        assert final['scene']==157 and final['map']==156 and [p['role'] for p in final['party']]==[0,2]
        assert final['cash']==initial['cash'] and final['field']['layer']==48
    elif equipment_entry:
        assert final['scene']==146 and final['map']==74 and [p['role'] for p in final['party']]==[0,2]
        assert final['cash']==initial['cash']
    elif args.medicine:
        assert final['scene']==54 and [p['role'] for p in final['party']]==[0,1,2]
        assert final['cash']==initial['cash']
        record({'check':'native phone uses saved original six-god medicine, reads treatment/temple story and restores the manually owned three-role party','passed':True,'pages':pages})
    elif args.rescue or args.stone_key:
        assert final['scene']==(49 if args.stone_key else 48) and [p['role'] for p in final['party']]==[0,2]
        assert final['cash']==initial['cash']
    else:assert final['scene']==2 and [p['role'] for p in final['party']]==[0]
    size=a.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.width,r.height]})()')
    assert abs(size[0]-320)<2 and abs(size[1]-200)<2,size
    assert not a.js('!!document.querySelector("[data-pal-controls] button")'),'virtual operation buttons remain in the game surface'
    a.b('screenshot',OUT/'phone-field.png')
    if args.caiyi_sacrifice or args.tower_bridges:
        centre=surface_centre
        d=a.data();x,y=centre(d);a.touch(x,y,'hold')
        wait(lambda:a.data()['menu']['kind']==1,'long press on the character did not open the original menu')
        d=a.data();target=next(row for row in d['menu']['items'] if row['value']==3)
        a.touch(target['x']+8,target['y']+8)
        wait(lambda:a.data()['menu']['selected']==3,'touch on the original inventory entry did not move its native cursor')
        assert a.data()['menu']['kind']==1,'selecting another original menu entry executed it before confirmation'
        a.touch(target['x']+8,target['y']+8)
        wait(lambda:a.data()['menu']['kind']==4,'second tap did not open the selected original inventory entry')
        d=a.data();target=next(row for row in d['menu']['items'] if row['value']==2)
        a.touch(target['x']+8,target['y']+8)
        wait(lambda:a.data()['menu']['selected']==2,'touch did not select the original use-item entry')
        if a.data()['menu']['kind']==4:a.touch(target['x']+8,target['y']+8)
        wait(lambda:a.data()['menu']['kind']==5,'direct native menu taps did not open the original item list')
        a.b('screenshot',OUT/'phone-native-inventory.png')
        a.touch(160,100,'hold')
        wait(lambda:a.data()['phase']=='field','long press did not return from the original inventory')
        record({'check':'long press opens original menu; original menu entries select and confirm by native touch; long press returns without virtual buttons','passed':True})
        directions=[('up',(16,-8),2),('left',(-16,-8),1),('down',(-16,8),0),('right',(16,8),3)]
        for direction,(dx,dy),facing in directions:
            before=a.data();cells=base64.b64decode(before['field']['cells']);u=(dx//16+dy//8)//2;z=(dy//8-dx//16)//2
            flags=cells[(z+8)*17+u+8];assert not flags&2,'direction test reached a narrative trigger'
            a.tap('[data-pal-direction="'+direction+'"]')
            wait(lambda:a.data()['seq']>before['seq'] and a.data()['seq']==a.data()['appliedSeq'],'surface direction was not accepted and applied')
            after=a.data();assert after['field']['direction']==facing,(direction,after['field']['direction'])
            assert [after['x'],after['y']]==([before['x'],before['y']] if flags&1 else [before['x']+dx,before['y']+dy]),direction
        before=a.data();x,y=centre(before);a.touch(x,y,'two')
        wait(lambda:a.data()['menu']['kind']==1,'two-finger tap did not open the original menu')
        a.touch(160,100,'hold');wait(lambda:a.data()['phase']=='field','long press did not return from the original main menu')
        assert [a.data()['x'],a.data()['y']]==[before['x'],before['y']],'two-finger menu gesture moved the party'
        before=a.data();x,y=centre(before);a.touch(x,y,'pinch')
        assert a.data()['phase']=='field' and [a.data()['x'],a.data()['y']]==[before['x'],before['y']],'pinch moved the party or opened a menu'
        for mode in ['hold','hold-cancel']:
            before=a.data();direction,(dx,dy),_=ground_direction(before);x,y=centre(before)
            a.touch(x+dx*4,y+dy*4,mode)
            wait(lambda:[a.data()['x'],a.data()['y']]!=[before['x'],before['y']],'held surface direction did not move the party')
            time.sleep(.2);stopped=a.data();time.sleep(.4)
            assert [a.data()['x'],a.data()['y']]==[stopped['x'],stopped['y']],'released or cancelled surface hold kept walking'
        a.b('screenshot',OUT/'phone-surface-field.png')
        record({'check':'four character-relative directional regions use original collision/facing; two-finger menu gesture and pinch do not move the party; held movement stops on release/cancel','passed':True})
    a.mobile_process.terminate();a.mobile_process.wait(timeout=5);a.mobile_process=None
    a.b('set','viewport',1280,900);a.b('click','[data-pal-board] canvas')
    assert not a.js('!!document.querySelector("[data-pal-controls]")?.getClientRects().length')
    desktop_size=a.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.width,r.height]})()')
    assert abs(desktop_size[0]-1280)<2 and abs(desktop_size[1]-800)<2,desktop_size
    a.b('screenshot',OUT/'desktop-fullscreen.png')
    final=a.data();name,(dx,dy),_=ground_direction(final)
    a.b('press','Arrow'+name.title())
    expected=[final['x']+dx,final['y']+dy]
    wait(lambda:[a.data()['x'],a.data()['y']]==expected,'native desktop keyboard did not move the solo party')
    time.sleep(.35)
    assert [a.data()['x'],a.data()['y']]==expected,'released desktop key kept walking'
    record({'check':'native desktop keyboard moves original party once, releases correctly and has no virtual buttons; maximum proportional fullscreen canvas','passed':True,'position':expected,'canvas':desktop_size})
    if wave_candidate:
        def visible_wave():
            result=a.js('(async()=>{const {authorityWaveVisible}=await import("/pal-renderer-probe.js"),d=(await(await fetch("/api/state")).json()).room.game.data;return authorityWaveVisible(d)})()')
            return result['matches'] and result['differsFromUnwaved']
        wait(visible_wave,'actual blood-pool Canvas does not show its original map wave',100)
        record({'check':'actual new-core blood-pool Board pixels match current wave-bearing authority and differ from the same map/actors without waving; public motor supports native keyboard','passed':True})
    a.mobile();final=a.data();name,(dx,dy),_=ground_direction(final)
    a.tap('[data-pal-direction="'+name+'"]')
    expected=[final['x']+dx,final['y']+dy]
    wait(lambda:[a.data()['x'],a.data()['y']]==expected,'native phone could not move the solo party')
    before=a.data();a.tap('[data-room-fullscreen] [data-save-game]')
    wait(lambda:not a.room(),'field save did not release solo seat')
    a.b('wait','[data-resume-save]');a.tap('[data-resume-save]')
    wait(lambda:a.room() and a.room()['status']=='playing' and not a.room().get('loading'),'single account could not continue field',80)
    after=a.data()
    for key in ['scene','map','x','y','cash','party']:
        assert before[key]==after[key],{'changed':key}
    if wave_candidate:
        assert after.get('field') and after['draws'][0]['kind']==2 and after['draws'][0]['a']==2
        wait(visible_wave,'actual blood-pool wave did not remain visible after phone save/continue',100)
        REPORT['waveRenderingAccepted']=True
    record({'check':'native phone reading/movement and second personal save; maximum proportional canvas and manual whole-party ownership','passed':True,'pages':pages,'canvas':size,'party':after['party'],'cash':after['cash']})
    REPORT['passed']=True
except Exception as error:
    REPORT['error']=str(error)
    REPORT['diagnostics']=[]
    for p in players:
        try:
            REPORT['diagnostics'].append({'player':p.index,'state':p.api('/state'),'page':p.js('''(()=>({focused:document.hasFocus(),hidden:document.hidden,errors:[...document.querySelectorAll('.pal-board-error,[role=alert]')].map(e=>e.textContent),board:document.querySelector('[data-pal-board]')?.outerHTML,resources:performance.getEntriesByType('resource').filter(r=>r.name.includes('/visual-v1/')).map(r=>({name:r.name,duration:r.duration,bytes:r.decodedBodySize}))}))()''')})
            p.b('screenshot',OUT/('failure-'+str(p.index)+'.png'))
        except Exception:
            pass
    raise
finally:
    for p in players:
        if p.mobile_process:p.mobile_process.terminate();p.mobile_process.wait(timeout=5)
        try:p.b('close')
        except Exception:pass
        shutil.rmtree(p.profile,ignore_errors=True)
    (OUT/'result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
    (OUT/'stop').touch();print(json.dumps(REPORT,ensure_ascii=False))
