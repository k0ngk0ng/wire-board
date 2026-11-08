#!/usr/bin/env python3
"""Read PNG headers and simulate atlas packing. No images or API calls are made."""
import argparse
from collections import defaultdict
import hashlib
import json
from math import ceil
from pathlib import Path
import struct

ROOT = Path(__file__).resolve().parents[3]
BASE = ROOT / 'games/stoneage'
MAX_PIXELS = 8_294_400
SHAPES = [(2880, 2880), (3840, 2160), (2160, 3840)]


def pack(items, width, height):
    """First-fit decreasing shelves; deterministic, without rotating sprites."""
    bins = []
    for key, w, h in sorted(items, key=lambda r: (-r[2], -r[1], r[0])):
        assert w <= width and h <= height, (key, w, h, width, height)
        choices = [(width - shelf['x'] - w, i, j)
                   for i, atlas in enumerate(bins)
                   for j, shelf in enumerate(atlas['shelves'])
                   if h <= shelf['height'] and shelf['x'] + w <= width]
        if choices:
            _, i, j = min(choices)
            atlas, shelf = bins[i], bins[i]['shelves'][j]
        else:
            i = next((i for i, atlas in enumerate(bins)
                      if atlas['usedHeight'] + h <= height), len(bins))
            if i == len(bins):
                bins.append(dict(shelves=[], usedHeight=0, entries=[]))
            atlas = bins[i]
            shelf = dict(x=0, y=atlas['usedHeight'], height=h)
            atlas['shelves'].append(shelf)
            atlas['usedHeight'] += h
        atlas['entries'].append(dict(file=key, x=shelf['x'], y=shelf['y'],
                                     width=w, height=h))
        shelf['x'] += w
    return bins


def validate(bins, items, width, height):
    expected = {key: (w, h) for key, w, h in items}
    seen = set()
    for atlas in bins:
        entries = atlas['entries']
        for i, e in enumerate(entries):
            key, x, y, w, h = (e[k] for k in ('file', 'x', 'y', 'width', 'height'))
            assert key not in seen and expected[key] == (w, h)
            assert x >= 0 and y >= 0 and x + w <= width and y + h <= height
            seen.add(key)
            for q in entries[:i]:
                assert (x + w <= q['x'] or q['x'] + q['width'] <= x
                        or y + h <= q['y'] or q['y'] + q['height'] <= y), (e, q)
    assert seen == set(expected), 'missing or duplicate source images'


def scenario(name, groups, dims, scale, gutter, shapes, keep):
    candidates = []
    excluded = []
    for ow, oh in shapes:
        assert ow % 16 == oh % 16 == 0
        assert max(ow, oh) <= 3840 and max(ow, oh) <= min(ow, oh) * 3
        assert 655_360 <= ow * oh <= MAX_PIXELS
        width, height = ow // scale, oh // scale
        too_large = [key for keys in groups.values() for key in keys
                     if dims[key][0] + 2 * gutter > width
                     or dims[key][1] + 2 * gutter > height]
        if too_large:
            excluded.append(dict(size=[ow, oh], imagesTooLarge=sorted(too_large)))
            continue
        bins = []
        for group, keys in sorted(groups.items()):
            items = [(key, dims[key][0] + 2 * gutter, dims[key][1] + 2 * gutter)
                     for key in keys]
            packed = pack(items, width, height)
            validate(packed, items, width, height)
            for atlas in packed:
                bins.append(dict(group=group, entries=atlas['entries']))
        candidates.append((len(bins), ow, oh, bins))
    if not candidates:
        return dict(strategy=name, scale=scale, requests=None,
                    reason='不能在该尺寸内完整保留所有原图，必须切分大图或使用更大输出',
                    excludedCandidates=excluded)
    count, ow, oh, bins = min(candidates, key=lambda c: c[:3])
    reserved_pixels = sum((w + 2 * gutter) * (h + 2 * gutter)
                          for w, h in dims.values())
    lower_bound = ceil(reserved_pixels * scale * scale / (ow * oh))
    result = dict(strategy=name, scale=scale, sourceGutterPerEdge=gutter,
                  outputAtlas=[ow, oh], requests=count, outputImages=count,
                  areaLowerBoundWithGutters=lower_bound,
                  candidateCounts=[dict(size=[w, h], requests=n) for n, w, h, _ in candidates],
                  excludedCandidates=excluded,
                  algorithm='first-fit decreasing shelves; no rotation; validated coverage, bounds and non-overlap',
                  provenMinimum=count == lower_bound)
    if keep:
        result['atlases'] = [dict(group=a['group'], entries=[dict(
            file=e['file'], sourceCrop=[e['x'] + gutter, e['y'] + gutter,
                                       dims[e['file']][0], dims[e['file']][1]],
            outputCrop=[(e['x'] + gutter) * scale, (e['y'] + gutter) * scale,
                        dims[e['file']][0] * scale, dims[e['file']][1] * scale])
            for e in a['entries']]) for a in bins]
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--source', type=Path, required=True)
    parser.add_argument('--output', type=Path, default=BASE / 'upscale-estimate.json')
    parser.add_argument('--manifest', type=Path)
    args = parser.parse_args()
    for out in (args.output, args.manifest):
        if out:
            assert out.resolve().is_relative_to(ROOT), 'outputs must stay in workspace'
    art_path = BASE / 'web/art.json'
    art = json.loads(art_path.read_text())
    assets = args.source.resolve() / 'client/web/assets/original'
    dims, aliases, canonical = {}, defaultdict(list), {}
    for key, metadata in sorted(art['resources'].items()):
        with (assets / key).open('rb') as stream:
            header = stream.read(24)
        assert header[:8] == b'\x89PNG\r\n\x1a\n', key
        size = struct.unpack('>II', header[16:24])
        representative = canonical.setdefault(metadata['sha256'], key)
        aliases[representative].append(key)
        if representative in dims:
            assert dims[representative] == size
        dims[representative] = size
    groups = defaultdict(set)
    owners = {}
    for graphic, actions in sorted(art['sprites'].items(), key=lambda pair: int(pair[0])):
        for action in actions:
            for frame in action['frames']:
                key = canonical[art['resources'][frame['file']]['sha256']]
                assert (frame['width'], frame['height']) == dims[key]
                owners.setdefault(key, 'sprite-' + graphic)
    for key in dims:
        groups[owners.get(key, 'background' if key.startswith('battle/') else 'icons')].add(key)
    global_group = {'all': set(dims)}
    pixels = sum(w * h for w, h in dims.values())
    results = []
    manifests = []
    for scale in (2, 4):
        for name, split, gutter, shapes in [
            ('global-tight-4K', global_group, 0, SHAPES),
            ('global-4K', global_group, 4, SHAPES),
            ('per-sprite-4K', groups, 4, SHAPES),
            ('global-1440p', global_group, 4, [(2560, 1440), (1440, 2560)]),
        ]:
            result = scenario(name, split, dims, scale, gutter, shapes, bool(args.manifest))
            if 'atlases' in result:
                manifests.append(dict(strategy=name, scale=scale,
                                      outputAtlas=result['outputAtlas'],
                                      sourceGutterPerEdge=gutter, atlases=result.pop('atlases')))
            results.append(result)
    report = dict(
        scope='当前竞技场全部可能使用的原始 PNG；仅头部读取和排布模拟，无合成图片或 API 调用',
        modelFamily='gpt-image-2.5',
        models=['gpt-image-2.5-sunburst', 'gpt-image-2.5-flare'],
        uniqueFilePaths=len(art['resources']), distinctSHA256=len(dims),
        duplicateFilePaths=len(art['resources']) - len(dims), sourcePixels=pixels,
        sourceDimensions=dict(maxWidth=max(w for w, _ in dims.values()),
                              maxHeight=max(h for _, h in dims.values())),
        theoreticalAreaLowerBounds=[dict(scale=s, outputPixels=pixels*s*s,
                                         requests=ceil(pixels*s*s/MAX_PIXELS)) for s in (2, 4)],
        requestAssumption='每张独立 atlas 使用一次 edits 请求，n=1；不含试样、重试或返工',
        scenarios=results,
        limitations=[
            '面积下限忽略留白和几何排布；模拟结果是可排布数量，不是最优性或模型质量证明。',
            '4K 输出属于超过 2560×1440 的实验分辨率。',
            'n 最多 10 代表同一请求的多个输出；官方未承诺与多个输入一一对应编辑。',
            '生成式增强不能保证坐标、透明边界、姿态和跨帧一致性，须先验证试样。',
        ],
        sources=[
            'https://developers.openai.com/api/docs/guides/image-generation',
            'https://developers.openai.com/api/reference/resources/images/methods/edit',
            'https://developers.openai.com/api/docs/models/gpt-image-2.5-sunburst',
            'https://developers.openai.com/api/docs/models/gpt-image-2.5-flare',
        ],
        manifestSHA256=hashlib.sha256(art_path.read_bytes()).hexdigest(),
        assetsCopied=0, generationAPICalls=0)
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    if args.manifest:
        args.manifest.parent.mkdir(parents=True, exist_ok=True)
        args.manifest.write_text(json.dumps(dict(aliases=aliases, scenarios=manifests),
                                            ensure_ascii=False, separators=(',', ':')) + '\n')
    print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == '__main__':
    main()
