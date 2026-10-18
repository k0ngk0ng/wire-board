"""Describe source frame resources and register observations, not a PPU runtime.

One final-picture frame has a complete independently checked band contract.
Other frames retain exact observed writes/snapshots and explicitly leave their
fetch-band derivation unresolved for the renderer integration.
"""
import hashlib,json
from pathlib import Path
from PIL import Image,ImageChops
from capture_reference import ROOT,BASE
from graphics import render_background,render_sprites

OUT=ROOT/'.local/postresult-video'

def state_fields(file):
    raw=file.read_bytes();assert raw[:4]==b'FCSX' and raw[12:16]==b'\xff'*4
    result={};position=16
    while position<len(raw):
        length=int.from_bytes(raw[position+1:position+5],'little');cursor=position+5;end=cursor+length
        while cursor+8<=end:
            name=raw[cursor:cursor+4].rstrip(b'\0').decode('ascii',errors='replace');size=int.from_bytes(raw[cursor+4:cursor+8],'little')
            result[name]=raw[cursor+8:cursor+8+size];cursor+=8+size
        position=end
    return result

def source(file):return dict(path=str(file.relative_to(ROOT)),sha256=hashlib.sha256(file.read_bytes()).hexdigest())
def main():
    colors=(BASE/'runs/graphics-right/nes-colors.rgb').read_bytes();records=[]
    for kind,local_frames in [('reward',[540,660]),('ending',[600,1800,6000,10500,12000,13500])]:
        path=BASE/f'runs/postresult-{kind}-observed';summary=json.loads((OUT/f'{kind}-summary.json').read_text())
        events=[json.loads(l) for l in (path/'events.jsonl').read_text().splitlines()]
        ram=(path/'ram.bin').read_bytes()
        for local in local_frames:
            snap=next(s for s in summary['snapshots'] if s['localFrame']==local);frame=snap['emulatorFrame'];stem=f'ppu-{frame:05d}'
            state=ram[(local-1)*2048:local*2048]
            row=dict(kind=kind,localFrame=local,emulatorFrame=frame,scene=state[0x588],phase=state[0x589]&127,screenshot=source(path/f'frame-{local:05d}.png'),ppu=source(path/f'{stem}-ppu.bin'),oam=source(path/f'{stem}-oam.bin'),ctrl=source(path/f'{stem}-ctrl.bin'),palette=snap['palette'],afterFrameCHR=snap['chrPages'],bands=None,bandStatus='unresolved; original IRQ/register events retained; do not use final CHR for entire frame')
            selected=[e for e in events if e['frame']==frame]
            before=next(e for e in selected if e['kind']=='beforeFrame')
            row['beforeFrameCycles']=before['cycles']
            row['rasterWrites']=[]
            register=None
            for event in selected:
                if event['kind']!='deviceWrite':continue
                address=event['address']
                if address==0x8000:register=event['value']&7
                if address in (0x8000,0x8001) and (address==0x8000 or register is None or register>=6):continue
                row['rasterWrites'].append(dict(address=address,value=event['value'],pc=event['pc'],cycles=event['cycles'],cyclesAfterBeforeFrame=event['cycles']-before['cycles'],**(dict(chrRegister=register) if address==0x8001 else {})))
            row['interruptSnapshots']=[dict(**item,asset=source(path/item['file'])) for item in summary['interruptSnapshots'] if item['frame']==frame]
            if kind=='ending':
                file=BASE/f'runs/postresult-ending-raster-states/before-{frame:05d}.fcs';fields=state_fields(file)
                row['beforeFrameState']=source(file);row['initialMirroring']=fields['A000'][0]
                row['initialClock']=dict(cpuCycles=int.from_bytes(fields['TSBS'],'little'),cpuRemainder=int.from_bytes(fields['ICou'],'little',signed=True),oddFrame=fields['KOOK'][0])
                assert row['initialClock']['cpuCycles']==before['cycles']
            if kind=='ending' and local==13500:
                ppu=(path/f'{stem}-ppu.bin').read_bytes();oam=(path/f'{stem}-oam.bin').read_bytes();ctrl=(path/f'{stem}-ctrl.bin').read_bytes()[0]
                image=render_background(ppu,colors).crop((0,0,256,240));sprites=render_sprites(ppu,oam,ctrl,colors);image.paste(sprites,(0,0),sprites)
                reference=Image.open(path/f'frame-{local:05d}.png').convert('RGB');diff=ImageChops.difference(image,reference);assert diff.getbbox() is None
                image.save(OUT/'phase9-offline.png')
                row['bands']=[dict(y0=0,y1=240,nametable=0x2000,scrollX=0,scrollY=0,backgroundCHR=[20,21,22,23],spriteCHR=[56,57,11,40])]
                row['bandStatus']='verified: 61440 original-frame pixels equal using existing offline tile/sprite readers'
            records.append(row)
    doc=dict(format=1,sourceIsControlledOriginalConsole=True,naturalEarnedVictory=False,rgb=source(BASE/'runs/graphics-right/nes-colors.rgb'),records=records,limitations=['Exact write cycles and per-frame saved clock state do not alone establish pixel-fetch boundaries.','Complex scene bands remain explicit null until renderer tests establish original fetch/scroll/CHR transitions.','Source captures are observations for tests; browser production must consume recovered semantic scene state and static assets, not replay recorded OAM/RAM.'])
    (OUT/'raster-reference.json').write_text(json.dumps(doc,indent=2)+'\n');print(json.dumps(dict(frames=len(records),completeBandContracts=1,partialEventContracts=len(records)-1)))
if __name__=='__main__':main()
