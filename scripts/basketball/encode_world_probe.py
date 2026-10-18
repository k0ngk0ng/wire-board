"""Explicit semantic initial-state encoding for controlled original-machine probes.

Offline research only. Never used by the Go game; not a per-frame replay.
"""
from compare_rest_stall_worlds import normalized

def encode_world_probe(snapshot):
 s=normalized(snapshot);p=s['playing'];w=p['world'];prep=w['preparation'];a=prep['actions'];c=p['context'];patch={};names={}
 def b(addr,value,name):patch[addr]=int(value)&255;names[addr]=name
 def seq(addr,values,name):
  for i,value in enumerate(values):b(addr+i,value,name+str(i))
 def integer(addrs,value,name):
  for i,addr in enumerate(addrs):b(addr,value>>(8*i),name)
 def obj(i,o,body):
  tag='object'+str(i)
  b(0x512+i,o['action']|(128 if o['entered'] else 0),tag+'.action');b(0x51f+i,o['request']['action']|(0 if o['request']['pending'] else 128),tag+'.request')
  b(0x5b+i,o['pose']['frame']|(128 if o['pose']['mirrored'] else 0),tag+'.pose');b(0x46b+i,128 if o['facingleft'] else 0,tag+'.facing');b(0x5f7+i,128 if o['airborne'] else 0,tag+'.air')
  anim=o['animation']
  for address,key in [(0x437,'elapsed'),(0x444,'cursor'),(0x451,'loops')]:b(address+i,anim[key],tag+'.animation.'+key)
  b(0x52c+i,anim['nextaction'] if anim['hasnextaction'] else 255,tag+'.nextAction')
  pos=body['position'];v=body['velocity']
  for addresses,value,label in [([0x68+i,0x75+i,0x82+i],pos['x'],'x'),([0x8f+i,0x9c+i],pos['depth'],'depth'),([0xa9+i,0xb6+i,0xc3+i],pos['height'],'height'),([0x478+i,0x485+i],v['x'],'vx'),([0x492+i,0x49f+i],v['depth'],'vy'),([0x4ac+i,0x4b9+i],v['height'],'vz'),([0x4c6+i,0x4d3+i],body['gravity'],'gravity')]:integer(addresses,value,tag+'.'+label)
 for i,actor in enumerate(a['actors']):
  o=actor['player'];obj(i,o,o['body']);b(0x4ec+i,255 if o['heading']==8 else o['heading']*32,'heading');b(0x6d7+i,o.get('newyorkpassage',0),'courtFlag')
  for address,key in [(0x572,'shotcycle'),(0x54c,'aimwindow'),(0x576,'jumpphase'),(0x611,'endurance'),(0x550,'linkedactor'),(0x554,'specialshot'),(0x57a,'attackvariant')]:b(address+i,actor[key],'actor.'+key)
  b(0x565+i,actor['dodge']['step'],'dodge');b(0x548+i,actor['dodge']['initialxhigh'],'dodgeX')
 for j,o in enumerate(a['auxiliary']):
  i=j+4;obj(i,o,o['body']);b(0x539+i,o['owner'],'auxOwner');b(0x642+i,o['attachment'],'auxAttachment');b(0x4ec+i,255 if o['heading']==8 else o['heading']*32,'auxHeading');b(0x6d7+i,o['newyorkpassage'],'auxCourtFlag');b(0x56b+i,o['objectlink'],'auxLink')
 obj(6,a['ball'],a['ball']['body']);b(0x511,a['ball']['bouncesremaining'],'bounces');b(0x564,a['ball']['deflection'],'deflection')
 for j,o in enumerate(a['hoops']):
  i=j+7;obj(i,o,o);b(0x539+i,o['owner'],'hoopOwner');b(0x626+i,o['scorelock'],'hoopScoreLock');b(0x577+i,o['durability'],'hoopDurability')
 for address,key in [(0x648,'balllink'),(0x56a,'posesource'),(0x53f,'holder'),(0x561,'excludedcatcher'),(0x6a8,'lasttouch'),(0x56b,'specialholder'),(0xe4,'detachedcount'),(0x571,'contactlatch'),(0x55d,'selectedhoop'),(0x6d1,'lastshooter'),(0x56e,'dunktargetobject'),(0x547,'auxiliaryrelease'),(0x569,'carriedplayer')]:b(address,a[key],key)
 b(0x55a,a.get('unownedspecialshot',0),'unownedSpecial');b(0x56c,a.get('captureobject',0),'captureCounter');seq(0x642,a['playerlinks'],'links');seq(0x48,a['random'].to_bytes(4,'big'),'rng');seq(0x6f0,a['mountflags'],'mounts')
 t=a['launchtarget'];integer([0x69f,0x6a0],t['x']//256,'targetX');b(0x6a1,t['depth']//256,'targetDepth');integer([0x6a2,0x6a3],t['height']//256,'targetHeight');b(0x6a4,a['launcharcheight']//256,'arc');integer([0x699,0x69a],a['balllanding']['x']//256,'landingX');b(0x698,a['balllanding']['depth']//256,'landingDepth');integer([0x55e,0x55f],a['shotoriginx']//256,'shotOrigin')
 team=prep['teamai']
 for i,t in enumerate(team['teams']):b(0x663+i,t['mode']|(128 if t['entered'] else 0),'teamMode');b(0x668+i,t['counter'],'teamCounter')
 for i,t in enumerate(team['players']):
  b(0x672+i,t['mode']|(128 if t['entered'] else 0),'aiMode')
  for address,key in [(0x676,'plan'),(0x66e,'counter'),(0x682,'planparameter'),(0x67a,'trackedobject'),(0x67e,'destination')]:b(address+i,t[key],'ai.'+key)
 contacts=prep['contacts'];b(0x633,contacts['rotation'],'rotation');seq(0x634,contacts['marks'],'marks');seq(0x3c,contacts['candidates'],'oldContactEvent');b(0x665,contacts['startingballaction'],'startBallAction');b(0x6a7,contacts['parity'],'contactParity')
 for address,key in [(0x546,'previousholder'),(0x696,'scoredifference'),(0xe3,'scoresounddelay'),(0x6cf,'previousballaction'),(0x6d0,'observedballaction'),(0x6f6,'selectedboard'),(0x6d6,'displayflags')]:b(address,prep[key],key)
 seq(0x66a,prep['zones'],'zones');seq(0x6dd,prep['courtscoreflags'],'courtScore');b(0x6df,prep.get('courtoutcomeflags',0),'courtOutcome')
 for i,value in enumerate(prep['scores']):integer([0x166+i,0x168+i],value,'score')
 inp=w['input']
 for i,m in enumerate(inp['memory']):
  for address,key in [(0x4f9,'tapwindow'),(0x509,'lasttap'),(0x4fd,'attacktapwindow'),(0x505,'lastattacktap'),(0x501,'buttondelay'),(0x50d,'pendingbuttons')]:b(address+i,m[key],'input.'+key)
 seq(0x4e0,inp['held'],'held');seq(0x4e4,inp['pressed'],'pressed')
 for j,h in enumerate(inp['human']):
  i=j*2
  for address,key in [(0x657,'evenstatus'),(0x653,'request'),(0x658,'status'),(0x68a,'accepted')]:b(address+i,h.get(key,0),'human.'+key)
  for address,key in [(0x65b,'buttonwindows'),(0x65f,'timers'),(0x6c6,'messages'),(0x68e,'overrides')]:seq(address+i,h[key],'human.'+key)
 camera=w['camera']
 for address,key in [(0x397,'displayedx'),(0x39b,'x'),(0x39d,'y')]:integer([address,address+1],camera[key],'camera.'+key)
 integer([0x399,0x39a],w['displayedcameray'],'displayedCameraY');b(0x3a1,camera['tilecolumn'],'tileColumn');b(0x6d5,camera['shake'],'shake');b(0x300,w['logicframe'],'logicFrame')
 projection=w['projection'];seq(0x41d,projection['draworder'],'drawOrder');b(0x41c,projection['drawcount'],'drawCount')
 for i,point in enumerate(projection['projected']):integer([0x3e7+i,0x3f4+i],point['x'],'projectionX');integer([0x401+i,0x40e+i],point['y'],'projectionY')
 ctx=c['actions'];cfg=s['config']['match']
 seq(0x3d6,c['objectkinds'],'kind');seq(0x3e3,ctx['characters'],'character');seq(0x586,ctx['teams'],'team');seq(0x539,ctx['controls'],'control');seq(0x4e8,cfg['controllerassignments'],'assignment');b(0x58b,ctx['courtid'],'court');b(0x58d,cfg['mode'],'mode');seq(0x6e4,[v for row in cfg['equipment'] for v in row],'equipment')
 seq(0x649,c['hooplinks'],'hoopLinks');seq(0x625,c['attackgrades'],'attackGrade');seq(0x621,ctx['recoverygrades'],'recoveryGrade');seq(0x666,c['humantactics'],'humanTactics');b(0x56d,c['backboardstrength']<<6,'backboardStrength');b(0x588,c['layoutid'],'scene');b(0x3a2,c['trackedobject'],'tracked')
 for i,profile in enumerate(ctx['profiles']):b(0x615+i,(profile['runspeed']-1024)//64,'runGrade');b(0x619+i,(profile['jumpvelocity']-1792)//128,'jumpGrade');b(0x61d+i,profile['shotskill'],'shotSkill')
 reserve=c['pursuitreserved']
 for address,key in [(0x55b,'action73'),(0xe5,'depth73'),(0xe7,'depth75'),(0xf4,'depth88'),(0xda,'xhigh88')]:b(address,reserve[key],'pursuit.'+key)
 b(0x55c,c['pursuitalias74']['actionbyte'],'alias74Action');b(0xe6,c['pursuitalias74']['depthbyte'],'alias74Depth')
 match=p['match'];clock=match['clock'];b(0x589,match['phase']|(128 if match['entered'] else 0),'phase');b(0x58c,match['period'],'period');b(0x6f7,match['overtimeflags'],'overtime')
 for address,key in [(0x6c2,'minutes'),(0x6c3,'tens'),(0x6c4,'ones'),(0x6c5,'videoremainder'),(0x3cd,'displayedsecond'),(0x3ce,'hudturn')]:b(address,clock[key],'clock.'+key)
 seq(0x3cb,clock['scorelowcache'],'scoreCache');seq(0x160,[v for row in match['displayedscores'] for v in row],'displayedScore');seq(0x3c7,p['tacticcache'],'tacticCache')
 return patch,names
