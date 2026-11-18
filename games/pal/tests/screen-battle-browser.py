import base64,json,os,re,shutil,subprocess,time,uuid
from pathlib import Path
ROOT=Path(__file__).resolve().parents[3]
FIXTURE=ROOT/'.local/pal/battle-board-browser'
OUT=ROOT/'.local/pal/performance/selection-battle-save' if os.environ.get('PAL_SELECTION_WEB')=='1' else FIXTURE
if os.environ.get('PAL_BATTLE_SAVE_SCOPE')=='continuous':OUT=ROOT/'.local/pal/performance/continuous-battle-save'
if os.environ.get('PAL_BATTLE_SAVE_SCOPE')=='mixed':OUT=ROOT/'.local/pal/performance/mixed-battle-save'
if recovery_scope:=os.environ.get('PAL_RECOVERY_SCOPE'):
    assert re.fullmatch(r'[a-z][a-z0-9-]{0,39}',recovery_scope)
    FIXTURE=ROOT/'.local/pal/performance'/('battle-recovery-'+recovery_scope)
    OUT=FIXTURE/'battle-save'
if OUT!=FIXTURE:OUT.mkdir(exist_ok=False)
BASE=(FIXTURE/'url').read_text().strip()
ENV=dict(os.environ,AGENT_BROWSER_SOCKET_DIR=str(ROOT/'.local/pab'),TMPDIR=str(ROOT/'.local/tmp'),AGENT_BROWSER_NO_VERSION_CHECK='1',AGENT_BROWSER_ARGS='--mute-audio,--disable-breakpad,--disable-crash-reporter')
SESSION=subprocess.check_output(['agent-browser','session','id','--scope','worktree','--prefix','pal-surface-battle'],cwd=ROOT,env=ENV,text=True).strip()
REPORT={'passed':False,'scope':'real solo original battle fixture; native screen input, three-role commands, private saves and original reward; not campaign acceptance','cases':[]}
wave_candidate=False
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
                self.touch(data['x']-data['cameraX']+dx,data['y']-data['cameraY']-14+dy);return
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

players=[Browser(0)]
a=players[0]
def battle():return a.data().get('battle',{})
def ready_role(role):return a.data()['phase']=='battle' and battle().get('phase')==0 and battle().get('state')==1 and battle().get('menu')==0 and battle().get('current')==role
def save_continue(label):
    before=a.data();a.tap('[data-room-fullscreen] [data-save-game]')
    wait(lambda:not a.room(),'battle save did not release the sole owner')
    a.b('wait','[data-resume-save]');a.tap('[data-resume-save]')
    wait(lambda:a.room() and not a.room().get('loading'),'battle did not continue privately',80)
    after=a.data()
    def normalize(b):return {k:[{n:v for n,v in x.items() if n!='frame'}for x in value] if k in ['players','enemies']else value for k,value in b.items()}
    assert normalize(before['battle'])==normalize(after['battle']),label
    assert before['party']==after['party'] and before['cash']==after['cash'] and before['epoch']!=after['epoch'],label
    a.b('click','[data-fullscreen-enter]');a.b('wait','[data-room-fullscreen]')
    record({'check':'private save/continue preserves '+label,'passed':True})
def icon(index):
    coords=[(39,147),(12,162),(66,162),(39,177)];a.touch(*coords[index])
    wait(lambda:battle().get('selected')==index,'native battle icon touch did not select '+str(index))
    assert battle()['state']==1 and battle()['menu']==0
    a.touch(*coords[index])
def defend(role):
    wait(lambda:ready_role(role),'original role command did not become ready '+str(role))
    icon(3)
    wait(lambda:battle().get('menu')==4,'original battle misc menu did not open')
    def defense_glyph():return next((op for op in visible_ops(a.data()) if op['kind']==0 and op.get('color',0)>0 and op['id']==ord('防') and op['x']==16 and 32<=op['y']<=104),None)
    wait(lambda:defense_glyph() is not None,'original battle misc menu has not been drawn')
    item=defense_glyph()
    if not 249<=item['color']<=254:
        a.touch(item['x']+8,item['y']+8)
        wait(lambda:249<=defense_glyph()['color']<=254,'original defend cursor did not move')
    a.touch(item['x']+8,item['y']+8)
    wait(lambda:battle().get('players',[{}]*3)[role].get('action')==1 and (battle()['players'][role].get('state')==2 or battle()['players'][role].get('defending')),'native role did not accept or execute defense')
    assert battle()['players'][role]['action']==1
try:
    a.b('open',BASE);a.b('set','viewport',1280,900)
    a.api('/register',{'name':'画面战斗'+uuid.uuid4().hex[:7],'password':'pal-surface-test-123','invite':'test-invite'})
    room=a.api('/rooms',{'kind':'pal','name':'画面直接操作战斗','capacity':1});a.command('ready');a.command('start');a.b('reload')
    wait(lambda:a.room() and not a.room().get('loading') and ready_role(0),'original solo battle fixture did not become ready',80)
    assert len(a.data()['party'])==3 and [p['controller']for p in a.data()['party']]==[0,0,0]
    a.mobile();a.b('click','[data-fullscreen-enter]');a.b('wait','[data-room-fullscreen]')
    assert not a.js('!!document.querySelector("[data-pal-controls] button")')
    icon(1)
    wait(lambda:battle().get('menu')==1,'native magic icon did not open original magic list')
    a.b('screenshot',OUT/'surface-magic.png')
    save_continue('the original battle magic menu')
    injured=a.data()['party'][1]['hp']
    glyph=next(op for op in visible_ops(a.data()) if op['kind']==0 and op['id']==ord('气') and op['x']==35 and op['y']>=54)
    a.touch(glyph['x']+8,glyph['y']+8)
    wait(lambda:battle().get('state')==3,'touch on original healing spell did not open ally target selection')
    if battle()['target']!=1:
        a.touch(198,180);wait(lambda:battle()['target']==1,'touch on Ling HP panel did not select original healing target')
    a.touch(198,180)
    wait(lambda:battle()['players'][0]['state']==2,'original healing command was not accepted')
    assert battle()['players'][0]['action']==3 and battle()['players'][0]['target']==1
    save_continue('the first party member accepted healing for Ling')
    defend(1)
    defend(2)
    wait(lambda:ready_role(0) and a.data()['party'][1]['hp']>injured,'original accepted healing did not restore Ling in the next round',80)
    a.b('screenshot',OUT/'surface-healed.png')
    record({'check':'native original spell label and visible ally HP panel select and execute healing for Ling after a saved accepted command','passed':True,'beforeHP':injured,'afterHP':a.data()['party'][1]['hp']})
    defend(0)
    defend(1)
    wait(lambda:ready_role(2),'third original role is not owned by the same player')
    if battle()['selected']!=0:
        a.touch(39,147);wait(lambda:battle()['selected']==0,'native attack icon selection failed')
    active_enemies=[enemy for enemy in battle()['enemies'] if enemy['object']]
    assert active_enemies
    a.touch(39,147)
    if len(active_enemies)>1:
        wait(lambda:battle().get('state')in[2,4],'original attack target menu did not open')
        enemy=next(e for e in battle()['enemies'] if e['object']);a.touch(enemy['x'],enemy['y']-24)
    wait(lambda:a.data()['cash']==3,'ordinary native confirmation did not execute original attack/reward',80)
    a.b('screenshot',OUT/'surface-victory.png')
    record({'check':'native phone icons, spell/ally targets and original menu entries control all three original roles; two defend, the third attacks; original three-cash reward is issued once','passed':True,'cash':a.data()['cash'],'party':a.data()['party'],'singleEnemyNativeTarget':len(active_enemies)==1})
    size=a.js('(()=>{const r=document.querySelector("[data-pal-board] canvas").getBoundingClientRect();return [r.width,r.height]})()');assert size==[320,200],size
    REPORT['passed']=True
finally:
    if not REPORT['passed']:
        try:a.b('screenshot',OUT/'surface-failure.png');(OUT/'surface-failure.json').write_text(json.dumps(a.api('/state'),ensure_ascii=False,indent=2)+'\n')
        except Exception:pass
    if a.mobile_process:a.mobile_process.terminate();a.mobile_process.wait(timeout=5)
    try:a.b('close')
    except Exception:pass
    shutil.rmtree(a.profile,ignore_errors=True)
    (OUT/'surface-result.json').write_text(json.dumps(REPORT,ensure_ascii=False,indent=2)+'\n')
    (FIXTURE/'stop').touch();print(json.dumps(REPORT,ensure_ascii=False))
