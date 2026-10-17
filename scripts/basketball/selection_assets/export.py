"""Offline preparation-selection assets from pinned original sprite/court exports.

No ROM, emulator, captured RAM or screenshots are runtime dependencies. This
module does not decide which gameplay configurations the server may accept.
"""
import argparse
import hashlib
import io
import json
from pathlib import Path
from PIL import Image

ROOT = Path(__file__).resolve().parents[3]
ROM_SHA = '0fa28f60abb8d2ccbbeee90e79e3fa160a5cf32feb90f47ebf1a89168cff569b'
BODY_CROP = (48, 61, 80, 101)
PORTRAIT_CROP = (52, 61, 76, 85)
COURT_CROP = (0, 144, 512, 432)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def relative(path):
    return path.resolve().relative_to(ROOT).as_posix()


def source_file(directory, name, expected):
    path = (directory/name).resolve()
    if not path.is_relative_to(directory.resolve()):
        raise ValueError('source image leaves its declared directory')
    data = path.read_bytes()
    if sha(data) != expected:
        raise ValueError(f'source image digest mismatch: {path.name}')
    image = Image.open(io.BytesIO(data)).convert('RGBA')
    image.load()
    return image, relative(path)


def encode_image(files, name, image):
    encoded = io.BytesIO()
    image.save(encoded, format='PNG')
    data = encoded.getvalue()
    files[name] = data
    return dict(file=name, width=image.width, height=image.height, sha256=sha(data))


def encode_json(doc):
    return (json.dumps(doc, ensure_ascii=False, indent=2)+'\n').encode()


def build(atlas_dir, courts_dir):
    atlas_path=atlas_dir/'atlas.json'; atlas_raw=atlas_path.read_bytes(); atlas=json.loads(atlas_raw)
    if atlas['source']['rom_sha256'] != ROM_SHA:
        raise ValueError('ROM source identity mismatch in sprite atlas')
    selected={}
    for sheet in atlas['sheets']:
        if sheet['facing'] != 0 or sheet['priority'] != 'forward':
            continue
        key=(sheet['team'],sheet['character'])
        if key in selected:
            raise ValueError(f'duplicate sprite key {key}')
        selected[key]=sheet
    if set(selected) != {(t,c) for t in range(8) for c in range(4)}:
        raise ValueError('sprite inventory must contain exactly eight teams and four character slots')
    doc=dict(schemaVersion=1, accepted_for_release=False, requiresAuthoritativePreparationValidation=True,
             usage='room-preparation-selection', source=dict(rom_sha256=ROM_SHA, spriteAtlas=relative(atlas_path),
             spriteAtlasSHA256=sha(atlas_raw), spriteDecoder=atlas['source'],
             representation='Original gameplay idle pose 0, not reconstructed character-selection menu art.'),
             courts=[], teams=[], limitations=[
                 'Asset coverage is not gameplay support or permission to publish.',
                 'Only the New York court name is retained as verified; other labels are explicit numeric placeholders.',
                 'Character slots are original team-local IDs 0..3; table entry 4 is not a selectable-player claim.',
                 'All player thumbnails share the original idle pose, direction, palette and pixel scale.',
                 'Court previews crop lower scenery; they are not complete playable screenshots or IRQ/render acceptance.',
                 'Original source distribution permission and OSS publication remain unaccepted.'
             ])
    files={}
    for court in range(8):
        directory=courts_dir/str(court); manifest_path=directory/'manifest.json'
        raw=manifest_path.read_bytes(); source=json.loads(raw); palette=court+1
        if source['court']!=court or source['source']['rom_sha256']!=ROM_SHA:
            raise ValueError('ROM source identity or court identity mismatch')
        filename=f'court-{palette}.png'; asset=source['assets'][filename]
        image,image_path=source_file(directory,filename,asset['sha256'])
        if image.size!=(512,432) or list(image.size)!=asset['size']:
            raise ValueError('court dimensions do not match the original static export')
        preview=encode_image(files,f'courts/court-{court}.png',image.crop(COURT_CROP))
        doc['courts'].append(dict(id=court,displayName='纽约' if court==1 else f'球场 {court+1}',nameVerified=court==1,
            preview=preview,source=dict(court=court,palette=palette,sourceImage=image_path,sourceImageSHA256=asset['sha256'],
            sourceManifest=relative(manifest_path),sourceManifestSHA256=sha(raw),crop=list(COURT_CROP),
            originalSize=list(image.size),originalTables=source['source'])))
    for team in range(8):
        row=dict(id=team,displayName=f'球队 {team+1}',nameVerified=False,characters=[])
        for character in range(4):
            sheet=selected[(team,character)]
            candidates=[f for f in sheet['frames'] if f['pose']==0]
            if len(candidates)!=1:
                raise ValueError('exactly one original idle frame is required')
            frame=candidates[0]; image,image_path=source_file(atlas_dir,sheet['file'],sheet['sha256'])
            x,y,w,h=frame['rect']
            if (w,h)!=(128,128) or frame['origin']!=[64,96] or x<0 or y<0 or x+w>image.width or y+h>image.height:
                raise ValueError('idle frame geometry differs from the verified original atlas')
            tile=image.crop((x,y,x+w,y+h)); bounds=tile.getbbox()
            if not bounds or list(bounds)!=frame['bounds'] or not (BODY_CROP[0]<=bounds[0] and BODY_CROP[1]<=bounds[1] and BODY_CROP[2]>=bounds[2] and BODY_CROP[3]>=bounds[3]):
                raise ValueError('full-body crop would clip original player pixels')
            stem=f'players/team-{team}-character-{character}'
            body=encode_image(files,stem+'-body.png',tile.crop(BODY_CROP).resize((128,160),Image.Resampling.NEAREST))
            portrait=encode_image(files,stem+'-portrait.png',tile.crop(PORTRAIT_CROP).resize((96,96),Image.Resampling.NEAREST))
            row['characters'].append(dict(id=f'team-{team}-character-{character}',team=team,character=character,
                displayName=f'球员 {character+1}',nameVerified=False,fullBody=body,portrait=portrait,
                source=dict(atlas=relative(atlas_path),sourceImage=image_path,sourceImageSHA256=sheet['sha256'],
                team=team,character=character,pose=0,facing=0,priority='forward',frameRect=frame['rect'],origin=frame['origin'],
                bodyCrop=list(BODY_CROP),portraitCrop=list(PORTRAIT_CROP),scale=4,resampling='nearest-neighbor')))
        doc['teams'].append(row)
    gates=dict(schemaVersion=1,authority='evidence-only-not-an-allowlist',requiredPolicyOwner='server preparation validator',
        releaseReady=False,assetCoverage=dict(courts=list(range(8)),teams=list(range(8)),charactersPerTeam=list(range(4))),
        verifiedMatchBaseline=dict(courts=[0,1,2],teams=[0,1],characters=[0,1,0,1],seed='0x12345678',
            scope='Historical content-audit baseline only; later acceptance must be supplied by the server policy owner.',
            source='games/basketball/CONTENT_COVERAGE.md'),
        requirements=['Validate the entire court/team/character/mode/equipment tuple on the authoritative server.',
            'Do not enable a character or court because an image is present.',
            'Keep development asset availability separate from production registration and administrator listing.',
            'Do not derive labels, identities or gameplay support from array position or filename.'])
    files['manifest.json']=encode_json(doc);files['support-gates.json']=encode_json(gates)
    return files


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--atlas-dir',type=Path,default=ROOT/'.local/basketball/static-atlas')
    parser.add_argument('--courts-dir',type=Path,default=ROOT/'.local/basketball/courts')
    parser.add_argument('--output',type=Path,default=ROOT/'.local/basketball/selection-assets')
    args=parser.parse_args()
    if not args.output.resolve().is_relative_to(ROOT):
        raise ValueError('selection output must remain inside the workspace')
    files=build(args.atlas_dir.resolve(),args.courts_dir.resolve())
    # All inputs and pixel bounds are validated before any export is written.
    args.output.mkdir(parents=True,exist_ok=True)
    for name,data in files.items():
        path=args.output/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(data)
    print(json.dumps(dict(courts=8,teams=8,characters=32,images=72,accepted_for_release=False,
                         output=relative(args.output),manifestSHA256=sha(files['manifest.json']))))


if __name__=='__main__':
    main()
