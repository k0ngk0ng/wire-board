"""Retain actual scoped test results and source-decoded review images."""
from pathlib import Path
import gzip,hashlib,json,shutil
from PIL import Image,ImageDraw
ROOT=Path(__file__).resolve().parents[2]
PRIVATE=ROOT/'.local/contra-machine-vision'
OUT=ROOT/'docs/research/nes-contra/evidence'
def digest(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def main():
    refPath=ROOT/'games/contra/engine/testdata/bot-machine-original-art.json.gz'
    ref=json.loads(gzip.decompress(refPath.read_bytes()))
    red=(PRIVATE/'red.jsonl').read_bytes();green=(PRIVATE/'green.jsonl').read_bytes()
    assert b'undefined: BotMachineBackgroundVisionRecipes'in red
    events=[json.loads(x)for x in green.splitlines()];tests=[x['Test']for x in events if x.get('Action')=='pass'and x.get('Test')]
    assert len(tests)==3 and any(x.get('Action')=='pass'and not x.get('Test')for x in events)
    assert not any(x.get('Action')in['fail','build-fail']for x in events)
    artifacts=[]
    for name,raw in [('red',red),('green',green)]:
        p=OUT/f'bot-machine-background-{name}.jsonl.gz';p.write_bytes(gzip.compress(raw,mtime=0));artifacts.append(p)
    familyRed=PRIVATE/'red-family.jsonl'
    if familyRed.exists():
        raw=familyRed.read_bytes();assert b'missing visible machine family'in raw
        p=OUT/'bot-machine-background-red-family.jsonl.gz';p.write_bytes(gzip.compress(raw,mtime=0));artifacts.append(p)
    seen=set();positives=[]
    for c in ref['visualCases']:
        key=(c['level'],c['table'],c['code'])
        if key not in seen:seen.add(key);positives.append(c)
    for name,cases in [('visible',positives),('excluded',ref['negativeCases'])]:
        sheet=Image.new('RGB',(128*8,144*((len(cases)+7)//8)),(225,225,225));draw=ImageDraw.Draw(sheet)
        for i,c in enumerate(cases):
            scale=2 if c['height']>32 else 3
            im=Image.frombytes('RGBA',(c['width'],c['height']),bytes.fromhex(c['rgba'])).convert('RGB')
            x,y=i%8*128,i//8*144;sheet.paste(im.resize((im.width*scale,im.height*scale),Image.Resampling.NEAREST),(x+12,y))
            table={'tile-animation':'A','base':'B','update':'U'}[c['table']]
            label=f'L{c["level"]+1} {table}{c["code"]:02X}'
            if name=='visible':label+=f' K{c["kind"]:02X}'
            draw.text((x+8,y+112),label,fill=(0,0,0))
        p=OUT/f'bot-machine-background-{name}.png';sheet.save(p);artifacts.append(p)
    p=OUT/'bot-machine-background-hangar-map.png';shutil.copyfile(PRIVATE/'stage-7-boss-map-14.png',p);artifacts.append(p)
    sourceFiles=[ROOT/'games/contra/engine/bot_vision_machine_recipes.go',ROOT/'games/contra/engine/bot_vision_machine_recipes_test.go',ROOT/'scripts/contra/bot_machine_reference.py',refPath]
    report={'passed':True,'romSHA256':ref['romSHA256'],'tests':tests,'recipes':10,'patterns':16,'pixelAppearances':len(ref['visualCases']),'visibleAnchors':len(positives),'negativeImages':len(ref['negativeCases']),
            'command':"source scripts/env.sh\nmkdir -p .local/contra-machine-vision/tmp\nGOTMPDIR=$PWD/.local/contra-machine-vision/tmp go test -race ./games/contra/engine -run '^TestBotMachineBackgroundRecipes' -count=1 -json",
            'sourcePaletteProof':{'flameSegments':len(ref['flamePaletteProof']),'clawOriginalCalls':len(ref['clawPaletteObservations']),'clawPathSegments':len(ref['clawPathPaletteProof']),'launcherLocations':len(ref['launcherPaletteProof']),'clawStaticImageOwners':len(ref['clawSpawnAnchors'])},
            'files':[{'path':str(p.relative_to(ROOT)),'sha256':digest(p)}for p in sourceFiles+artifacts],
            'sourceEvidence':ref['sources'],'limitations':['Review images decode pinned original artwork, not a captured continuous independent campaign.','Carts are shared OAM sprites 2A/2B; no background recipe or moving/stationary identity is invented.','Shared closed panels, erased/exit art and unproven wall/generator fragments remain unknown.','Only this fixed recipe catalog and its pure pixel construction/classification are certified; global catalog integration and complete bot play remain separate.']}
    (OUT/'bot-machine-background-verification.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps({'passed':True,'tests':len(tests),'patterns':16,'appearances':len(ref['visualCases'])}))
if __name__=='__main__':main()
