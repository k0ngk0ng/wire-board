"""Export source-derived post-result resources. No ROM or CPU in browser assets."""
import argparse,hashlib,json
from pathlib import Path
from static_sprites import SpriteROM,DIGEST
from postresult_art import PRESETS,BG_PALETTES,SPRITE_PALETTES,POSE_BOUNDS,SPRITE_PAGES,decode_background,palette_groups,actor_profiles,index_atlas,background_preview

ROOT=Path(__file__).resolve().parents[2]

def main():
    p=argparse.ArgumentParser();p.add_argument('--out',type=Path,default=ROOT/'.local/postresult-assets');args=p.parse_args()
    out=args.out.resolve()
    if not out.is_relative_to(ROOT.resolve()):raise ValueError('Export must remain in worktree')
    out.mkdir(parents=True,exist_ok=True)
    rom=SpriteROM((ROOT/'.local/nes-basket/basketball.nes').read_bytes())
    colors=(ROOT/'.local/nes-basket/runs/graphics-right/nes-colors.rgb').read_bytes()
    if hashlib.sha256(colors).hexdigest()!='da30843b331e2e4474395866bc9871fcd60a7d716b3c8ee5ce3c89ee46e8c91e':raise ValueError('Expected pinned retained console RGB lookup')
    backgrounds=[decode_background(rom,preset) for preset in PRESETS]
    court_starts=[rom.prg[16384+0x3ec5+court] for court in range(8)]
    retained_court_pages={page for first in court_starts for page in range(first,first+4)}
    pages=sorted(set(SPRITE_PAGES)|retained_court_pages|{page for b in backgrounds for first in b['field_banks'] for page in (first,first+1)})
    manifest=dict(format=1,accepted_for_release=False,source=dict(rom_sha256=DIGEST,rgb_sha256=hashlib.sha256(colors).hexdigest(),backgrounds='fixed D8C4/DB8B descriptors; DC15..DC96 plus DCC3/DCD4/DCE5/DD05 decompression',palettes='bank4 962F (background),97AB (two sprite groups); 9526 universal-background normalization',actors='bank2 8000 kind pointers; D079/D0C6; kind19 fixed C410..C44E',chr='ROM CHR region, 1 KiB page indices; scene stores in bank3 B995 and bank4 BD8E'),retainedCourtCHR=dict(source='bank1 BEC5[court], reused by reward field',starts=court_starts),backgrounds=backgrounds,backgroundPalettes={str(i):palette_groups(rom,i,False) for i in BG_PALETTES},spritePalettes={str(i):palette_groups(rom,i,True) for i in SPRITE_PALETTES},actorProfiles={str(k):actor_profiles(rom,k) for k in POSE_BOUNDS},rgbPalette=[list(colors[i:i+3]) for i in range(0,192,3)],assets=[],indexAtlas=dict(file='chr-indices.png',pages=pages,tileSize=8,tilesPerPage=64,pixelValues=[0,1,2,3],rect='[localTile*8,pages.indexOf(page)*8,8,8]',note='L-mode palette indices, not display-ready grayscale; sprite index0 is transparent, background index0 uses universal palette0'),limitations=['Controlled original resource decoding, not earned victory/ending or full-console pixel capture.','Reward26 writes only six tile rows; retained attributes/palettes must be supplied by presentation. No fabricated full reward preview.','Credits25 targets2800; preserve original mirroring/scroll/IRQ state instead of flattening all backgrounds into one screen.','Previews show complete unscrolled source nametables under descriptor palettes only; omit sprites, text, masking, fades, IRQ bank/scroll splits and timing.','Sprite palettes and CHR pages change during ending; retain page/indices and apply current state.','Kind19 profile geometry is original; special tile substitution, OAM limits, priority and final screen projection require renderer integration.','No OSS upload, production wiring or release acceptance.'])
    def asset(name,image):
        file=out/name;image.save(file)
        manifest['assets'].append(dict(file=name,sha256=hashlib.sha256(file.read_bytes()).hexdigest(),size=list(image.size),mode=image.mode))
    asset('chr-indices.png',index_atlas(rom,pages))
    for b in backgrounds:
        if b['complete_nametable']:
            name=f'background-{b["preset"]}-preview.png';asset(name,background_preview(rom,b,colors));b['preview']=name
    (out/'manifest.json').write_text(json.dumps(manifest,separators=(',',':'))+'\n')
    print(json.dumps(dict(output=str(out),backgrounds=len(backgrounds),poses=sum(map(len,manifest['actorProfiles'].values())),chr_pages=len(pages),assets=len(manifest['assets']),manifest_sha256=hashlib.sha256((out/'manifest.json').read_bytes()).hexdigest())))
if __name__=='__main__':main()
