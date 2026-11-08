#!/usr/bin/env python3
"""Offline atlas preparation, slicing and QA. Never calls an image API."""
import argparse
import hashlib
import json
from pathlib import Path
from statistics import mean

import cv2
import numpy as np
from PIL import Image, ImageChops, ImageDraw, ImageStat

ROOT = Path(__file__).resolve().parents[3]
PROCESSING_VERSION = 3


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    p = argparse.ArgumentParser()
    p.add_argument('operation', choices=['prepare', 'extract', 'verify'])
    p.add_argument('--source', type=Path, required=True)
    p.add_argument('--work', type=Path, required=True)
    p.add_argument('--output', type=Path, required=True)
    p.add_argument('--batch', type=int)
    p.add_argument('--generated', type=Path)
    args = p.parse_args()
    source = args.source.resolve()
    work, output = args.work.resolve(), args.output.resolve()
    assert work.is_relative_to(ROOT) and output.is_relative_to(source / 'client/web/assets/hd')
    assets = source / 'client/web/assets/original'
    plan = json.loads((work / 'small-plan.json').read_text())
    original = json.loads((ROOT / 'games/stoneage/web/art.json').read_text())
    ui_files = {key: (pair, state) for pair in original.get('ui', {}).values()
                for state, key in pair.items()}
    output.mkdir(parents=True, exist_ok=True)
    (work / 'inputs').mkdir(exist_ok=True)
    if args.operation == 'prepare':
        batches = plan['batches'] if args.batch is None else [plan['batches'][args.batch]]
        for batch in batches:
            path = work / f"inputs/atlas-{batch['id']:03d}.png"
            if path.exists():
                continue
            canvas = Image.new('RGBA', tuple(plan['sourceAtlas']))
            for e in batch['entries']:
                x, y, w, h = e['crop']
                with Image.open(assets / e['file']) as im:
                    assert im.size == (w // 2, h // 2)
                    canvas.alpha_composite(im.convert('RGBA'), (x // 2, y // 2))
            canvas.resize(tuple(plan['targetAtlas']), Image.Resampling.NEAREST).save(path)
        print(json.dumps({'prepared': len(batches)}))
        return
    if args.operation == 'extract':
        assert args.batch is not None and args.generated is not None
        batch = plan['batches'][args.batch]
        result_path = work / f"batch-{args.batch:03d}-result.json"
        previous = json.loads(result_path.read_text()) if result_path.exists() else {}
        previous_files = {f['file']: f['sha256'] for f in previous.get('files', [])}
        if previous.get('passed') and previous.get('processingVersion') == PROCESSING_VERSION:
            print(json.dumps({'batch': args.batch, 'cached': True}))
            return
        generated = Image.open(args.generated).convert('RGBA')
        generated_rgb = np.asarray(generated.convert('RGB'))
        expected = tuple(plan['targetAtlas'])
        assert generated.size == expected, ('unexpected output dimensions', generated.size, expected)
        report = dict(batch=args.batch, model=plan['model'], passed=False,
                      processingVersion=PROCESSING_VERSION,
                      generatedSHA256=sha(args.generated), generatedSize=list(generated.size),
                      alphaMethod='original alpha resized 2x with Lanczos; generated RGB; original neutral RGB(20,20,20) shadows restored',
                      checks='exact output size; registered bounded crop; nonempty alpha; mean RGB error <=40 and body error <=45; original mask and logical size retained',
                      limitations='These checks cannot prove correct interior pose or temporal consistency; compare animation preview.',
                      files=[], warnings=[], manualReviews=[])
        reviews_path = work / 'manual-reviews.json'
        reviews = json.loads(reviews_path.read_text()) if reviews_path.exists() else {}
        contact = Image.new('RGB', (1200, 600), (38, 47, 48))
        draw = ImageDraw.Draw(contact)
        sample_step = max(1, len(batch['entries']) // 6)
        samples = batch['entries'][::sample_step][:6]
        frames = []
        for e in batch['entries']:
            x, y, w, h = e['crop']
            assert x >= 0 and y >= 0 and x + w <= generated.width and y + h <= generated.height
            with Image.open(assets / e['file']) as im:
                im = im.convert('RGBA')
                assert im.size == (w // 2, h // 2)
                source_sha = original['resources'][e['file']]['sha256']
                assert sha(assets / e['file']) == source_sha
                source_up = im.resize((w, h), Image.Resampling.LANCZOS)
                alpha = source_up.getchannel('A')
                template = np.asarray(source_up)
                body = ((template[:, :, 3] > 200) & (np.max(template[:, :, :3], axis=2) > 40))
                offset_x = offset_y = 0
                body_error = 0.0
                ui_icon = ui_files.get(e['file'])
                if np.any(body) and not ui_icon:
                    mx, my = min(48, max(12, w // 3)), min(20, max(12, h // 5))
                    left, top = max(0, x - mx), max(0, y - my)
                    right, bottom = min(generated.width, x + w + mx), min(generated.height, y + h + my)
                    search = generated_rgb[top:bottom, left:right]
                    score = cv2.matchTemplate(search, template[:, :, :3], cv2.TM_SQDIFF_NORMED,
                                              mask=body.astype(np.uint8) * 255)
                    score = np.nan_to_num(score, nan=100.0, posinf=100.0)
                    _, _, location, _ = cv2.minMaxLoc(score)
                    rx, ry = left + location[0], top + location[1]
                    def rgb_error(px, py):
                        return float(np.abs(generated_rgb[py:py+h, px:px+w].astype(float)
                                            - template[:, :, :3])[body].mean())
                    current_error, aligned_error = rgb_error(x, y), rgb_error(rx, ry)
                    if aligned_error < current_error - 0.75:
                        offset_x, offset_y = rx - x, ry - y
                        body_error = aligned_error
                    else:
                        body_error = current_error
                frame = generated.crop((x + offset_x, y + offset_y, x + offset_x + w, y + offset_y + h))
                shadow = Image.new('L', im.size)
                shadow.putdata([255 if r == g == b == 20 and a else 0
                                for r, g, b, a in im.get_flattened_data()])
                frame = Image.composite(source_up, frame, shadow.resize((w, h), Image.Resampling.LANCZOS))
                frame.putalpha(alpha)
                assert frame.getbbox() is not None
                small = frame.resize(im.size, Image.Resampling.LANCZOS).convert('RGB')
                mask = im.getchannel('A').point(lambda v: 255 if v >= 200 else 0)
                error = mean(ImageStat.Stat(ImageChops.difference(small, im.convert('RGB')), mask).mean)
                icon_check = None
                if ui_icon:
                    pair, state = ui_icon
                    with Image.open(assets / pair['normal']) as normal:
                        normal = np.asarray(normal.convert('RGBA').resize((w, h), Image.Resampling.LANCZOS))
                    glyph = ((normal[:, :, 2].astype(int) - np.maximum(normal[:, :, 0], normal[:, :, 1]).astype(int) > 70)
                             & (normal[:, :, 3] > 200))
                    rendered = np.asarray(frame)
                    channel = 2 if state == 'normal' else 0
                    others = [n for n in (0, 1, 2) if n != channel]
                    actual = ((rendered[:, :, channel].astype(int) - np.max(rendered[:, :, others], axis=2).astype(int) > 70)
                              & (normal[:, :, 3] > 200))
                    union = np.logical_or(glyph, actual).sum()
                    iou = float(np.logical_and(glyph, actual).sum() / max(1, union))
                    icon_check = dict(state=state, expectedColor='blue' if state == 'normal' else 'red',
                                      glyphIoU=round(iou, 5), passed=iou >= 0.70)
                    if not icon_check['passed']:
                        report['warnings'].append(dict(file=e['file'], icon=icon_check))
                elif error > 40 or body_error > 45:
                    review_key = f"{args.batch}:{e['file']}:{report['generatedSHA256']}"
                    review = reviews.get(review_key)
                    if review and review.get('accepted') and error <= 40 and body_error <= 55:
                        report['manualReviews'].append(dict(file=e['file'], **review))
                    else:
                        report['warnings'].append(dict(file=e['file'], meanRGBError=round(error, 3), bodyError=round(body_error, 3)))
                frames.append((e, frame, im, source_sha, error, body_error, offset_x, offset_y, icon_check))
        if report['warnings']:
            result_path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
            raise SystemExit(f"Batch {args.batch}: color/placement review required before exporting")
        for e, frame, im, source_sha, error, body_error, offset_x, offset_y, icon_check in frames:
            for key in plan['aliases'][e['file']]:
                path = output / key
                assert not path.exists() or previous_files.get(key) == sha(path), ('refusing to overwrite an unowned/changed file', key)
                path.parent.mkdir(parents=True, exist_ok=True)
                frame.save(path, compress_level=9)
                report['files'].append(dict(file=key, sourceSHA256=original['resources'][key]['sha256'],
                                            sha256=sha(path), bytes=path.stat().st_size,
                                            physicalSize=list(frame.size), logicalSize=list(im.size),
                                            registrationOffset=[offset_x, offset_y], bodyRGBError=round(body_error, 3),
                                            iconCheck=icon_check,
                                            meanRGBError=round(error, 3)))
            if e in samples:
                col = samples.index(e)
                for row, picture in enumerate((im.resize(frame.size, Image.Resampling.NEAREST), frame)):
                    picture = picture.copy()
                    picture.thumbnail((190, 260))
                    contact.paste(picture, (col * 200 + (190 - picture.width) // 2, row * 300 + 25), picture)
                    draw.text((col * 200 + 5, row * 300 + 5), 'Original 2x' if row == 0 else 'GPT restored', fill='white')
        contact.save(work / f"batch-{args.batch:03d}-comparison.png")
        report['passed'] = True
        result_path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
        print(json.dumps({'batch': args.batch, 'files': len(report['files']), 'passed': True,
                          'maxMeanRGBError': round(max(f[4] for f in frames), 3)}))
        return
    reports = [json.loads(p.read_text()) for p in sorted(work.glob('batch-*-result.json'))]
    files = {}
    for report in reports:
        assert report['passed'], report['batch']
        for entry in report['files']:
            key = entry['file']
            assert key not in files
            path = output / key
            assert sha(path) == entry['sha256']
            with Image.open(path) as im:
                assert list(im.size) == entry['physicalSize']
            files[key] = entry
    expected = set(original['resources'])
    manifest = dict(status='complete' if set(files) == expected else 'partial',
                    scale=2, model=plan['model'], sourceRoot='assets/original',
                    pathMapping='same relative PNG paths as original; no originals overwritten',
                    rendering='physical pixels are 2x; retain original logical frame sizes, offsets, ticks, actions and directions',
                    alphaMethod='original alpha sampled at 2x; model RGB locally registered; original neutral shadows restored',
                    frames=files, expectedFiles=len(expected), exportedFiles=len(files),
                    missingFiles=sorted(expected - set(files)),
                    sourceArtManifestSHA256=sha(ROOT / 'games/stoneage/web/art.json'))
    (output / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({k: manifest[k] for k in ('status', 'expectedFiles', 'exportedFiles')}))


if __name__ == '__main__':
    main()
