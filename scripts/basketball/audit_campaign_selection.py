"""Source-bound campaign/menu audit. No engine or product assets are written.
--capture records only missing declared controller scenarios; complete matches
are not changed or shortened. Existing matching evidence is reused.
"""
import argparse,hashlib,json
from pathlib import Path
from capture_reference import ROOT,BASE,run
OUT=ROOT/'.local/basketball/rea/campaign-audit'
ROM=BASE/'basketball.nes'
SHA='0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
def digest(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def state(path,frame):
 d=(path/'ram.bin').read_bytes();r=d[(frame-1)*2048:frame*2048];assert len(r)==2048
 return dict(scene=r[0x588],phase=r[0x589],mode=r[0x58d],period=r[0x58c],court=r[0x58b],teams=list(r[0x586:0x588]),characters=list(r[0x3e3:0x3e7]),controllers=list(r[0x4e8:0x4ec]),tactics=list(r[0x666:0x668]),winBits=list(r[0x6d2:0x6d4]),rewardSet=r[0x6d4],inventory=list(r[0xd3:0xe3]),equipment=[list(r[0x6e4+3*i:0x6e7+3*i]) for i in range(4)],grades=[[r[a+i] for a in [0x615,0x619,0x61d,0x621,0x625]] for i in range(4)],passwordDigits=list(r[0x3cb:0x3d1]))
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--capture',action='store_true');args=parser.parse_args()
 OUT.mkdir(parents=True,exist_ok=True);assert digest(ROM)==SHA
 scenarios=json.loads((ROOT/'scripts/basketball/campaign_selection_scenarios.json').read_text());runs={};records=[]
 for spec in scenarios:
  path=BASE/'runs'/spec['name'];complete=(path/'capture-complete.json').exists() and (path/'scenario.json').exists() and json.loads((path/'scenario.json').read_text())==spec
  if not complete:
   assert args.capture, 'missing matching evidence: '+spec['name']
   specpath=OUT/(spec['name']+'.json');specpath.write_text(json.dumps(spec,indent=2)+'\n');run(specpath)
  assert json.loads((path/'capture-complete.json').read_text())=={'status':'complete'}
  assert (path/'ram.bin').stat().st_size==spec['frames']*2048
  records.append(dict(name=spec['name'],frames=spec['frames'],controllerOnly=True,loadStateSHA256=digest(BASE/'states'/f"slot-{spec['load_slot']}.fcs"),scenarioSHA256=digest(path/'scenario.json'),ramSHA256=digest(path/'ram.bin'),screenshots={str(f):digest(path/f'frame-{f:05d}.png') for f in spec['captures']},states={str(f):state(path,f) for f in spec['captures']}));runs[spec['name']]=path
 solo=state(runs['campaign-audit-first-match-confirm'],2400);assert (solo['scene'],solo['phase'],solo['mode'],solo['court'],solo['period'])==(1,129,0,1,0);assert solo['controllers']==[0,255,255,255]
 coop=state(runs['campaign-audit-coop-first-match'],2000);assert (coop['scene'],coop['phase'],coop['mode'])==(1,129,1);assert coop['controllers']==[0,1,255,255]
 names=['ガンガンせめろ！','マークしろ！','みせるプレーだ！','おまかせ','ビシビシぼうぎょ！'];frames=[1490,1590,1690,1790,1390];prg=ROM.read_bytes()[16:131088];b3=prg[3*16384:4*16384];fixed=prg[-16384:];tactics=[]
 for i,(name,frame) in enumerate(zip(names,frames),1):
  s=state(runs['campaign-audit-tactic-labels'],frame);assert s['tactics'][0]==i
  pointer=int.from_bytes(b3[0xb729-0x8000+(i-1)*2:0xb72b-0x8000+(i-1)*2],'little')
  tactics.append(dict(profile=i,originalName=name,sourcePointer=f'bank3:{pointer:04X}',encodedGlyphs=list(b3[pointer-0x8000:pointer-0x8000+9]),normalMenuFrame=frame))
 loaded=state(runs['campaign-audit-password-equip-commit'],1200);assert loaded['period']==2 and loaded['winBits']==[1,0] and loaded['inventory']==[1]+[255]*15
 equip=state(runs['campaign-audit-password-equip-commit'],2800);assert equip['equipment'][0]==[1,255,255] and equip['inventory']==[255]*16
 removed=state(runs['campaign-audit-equipment-remove'],3600);assert removed['equipment'][0]==[255]*3 and removed['inventory']==[1]+[255]*15
 home=state(runs['campaign-audit-equipped-home-match'],4700);assert (home['scene'],home['phase'],home['mode'],home['court'],home['period'])==(1,129,0,0,2);assert home['equipment'][0]==[1,255,255]
 swap_before=state(runs['campaign-audit-equipment-swap-final'],1200);swap_first=state(runs['campaign-audit-equipment-swap-final'],2600);swap_last=state(runs['campaign-audit-equipment-swap-final'],3700)
 assert swap_before['winBits']==[1,1] and swap_before['inventory'][:2]==[0,1]
 assert swap_first['equipment'][0]==[0,255,255] and swap_first['inventory'][0]==1
 assert swap_last['equipment'][0]==[1,255,255] and swap_last['inventory'][0]==0
 reference=BASE/'runs/match-clock-complete';continuation=runs['campaign-audit-loss-continue'];old=(reference/'ram.bin').read_bytes();assert (continuation/'ram.bin').read_bytes()[:len(old)]==old
 assert (reference/'frame-24000.png').read_bytes()==(continuation/'frame-24000.png').read_bytes()
 loss=state(continuation,24000);resume=state(continuation,26400);assert loss['scene']==7 and loss['phase']==139 and resume['scene']==2 and resume['phase']==129 and resume['period']==0
 rewards=[]
 for i in range(4):
  pointer=int.from_bytes(fixed[0xff99-0xc000+i*2:0xff9b-0xc000+i*2],'little');rewards.append(dict(set=i,pointer=f'{pointer:04X}',items=list(fixed[pointer-0xc000:pointer-0xc000+14])))
 equipment=[]
 for i in range(32):
  pointer=int.from_bytes(fixed[0xc2c8-0xc000+i*2:0xc2ca-0xc000+i*2],'little');raw=list(fixed[pointer-0xc000:pointer-0xc000+9]);equipment.append(dict(id=i,slot='shirt' if i<10 else 'pants' if i<21 else 'shoes',namePointer=f'{pointer:04X}',encodedGlyphs=raw,scope='source name/slot classification; not all items naturally awarded/equipped'))
 meta=dict(status='passed',scope='original controller-only menus, first match openings, password-derived equipment flows, and natural loss continuation; no full campaign win claim',romSHA256=SHA,sourceScenariosSHA256=digest(ROOT/'scripts/basketball/campaign_selection_scenarios.json'),runs=records,tactics=tactics,normalTacticProfiles=[1,2,3,4,5],menuDefaultTactic=4,campaignCityOrder=list(b3[0x82be-0x8000:0x82c6-0x8000]),rewardTables=rewards,equipment=equipment,lossReferencePrefixIdentical=True,lossReferenceSHA256=digest(reference/'ram.bin'),ordinaryRewardMissingIDs=sorted(set(range(32))-{x for row in rewards for x in row['items']}))
 (OUT/'verified-selection.json').write_text(json.dumps(meta,indent=2,ensure_ascii=False)+'\n');print(json.dumps({k:meta[k] for k in ['status','scope','normalTacticProfiles','menuDefaultTactic','campaignCityOrder','ordinaryRewardMissingIDs','lossReferencePrefixIdentical']},ensure_ascii=False))
if __name__=='__main__':main()
