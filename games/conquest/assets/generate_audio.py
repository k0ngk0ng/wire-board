#!/usr/bin/env python3
"""Generate original short Conquest cues, without libraries or sampled media."""
import argparse
import hashlib
import io
import json
import math
from pathlib import Path
import struct
import wave

ROOT = Path(__file__).resolve().parents[3]
RATE = 22050
VERSION = 'conquest/v1'


def tone(time, start, duration, frequency, gain, fall=4.0):
    age = time - start
    if age <= 0 or age >= duration:
        return 0.0
    attack = min(1.0, age / .009)
    release = min(1.0, (duration - age) / .035)
    envelope = math.sin(attack * math.pi / 2) ** 2 * release ** 2 * math.exp(-fall * age / duration)
    # Soft fundamental with a quiet octave; no square waves or hard clipping.
    return gain * envelope * (math.sin(2 * math.pi * frequency * age) + .10 * math.sin(4 * math.pi * frequency * age))


def synthesize(name):
    if name == 'march':
        duration = .38
        voices = [(0,.21,145,.29,5.5),(.125,.21,124,.25,5.5)]
    elif name == 'capture':
        duration = .61
        voices = [(0,.34,329.63,.19,3.8),(.105,.36,415.30,.18,3.8),(.23,.36,493.88,.16,3.8)]
    elif name == 'round':
        duration = .56
        voices = [(0,.42,392,.20,3.2),(.14,.39,293.66,.18,3.2)]
    else:
        raise ValueError('Unknown cue: ' + name)
    count = round(duration * RATE)
    samples = []
    for index in range(count):
        time = index / RATE
        sample = sum(tone(time, *voice) for voice in voices)
        # Long outer fade keeps the first and last PCM samples at zero.
        edge = min(1.0, index / 100, (count - 1 - index) / 200)
        samples.append(round(sample * edge * 32767))
    output = io.BytesIO()
    with wave.open(output, 'wb') as stream:
        stream.setnchannels(1)
        stream.setsampwidth(2)
        stream.setframerate(RATE)
        stream.writeframes(struct.pack('<' + 'h' * len(samples), *samples))
    return output.getvalue(), count


def generate(destination):
    destination = destination.resolve()
    if not destination.is_relative_to(ROOT.resolve()):
        raise ValueError('Output must remain inside this worktree')
    directory = destination / 'public/conquest/v1/audio'
    directory.mkdir(parents=True,exist_ok=True)
    manifest = {
        'version': VERSION,
        'generator': 'games/conquest/assets/generate_audio.py',
        'generatorSha256': hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
        'license': 'wire-board contributors 原创，供 wire-board 项目使用；无外部采样或原版音效。',
        'source': {
            'kind': 'original-synthesis',
            'author': 'wire-board contributors',
            'externalSamples': [],
            'method': '确定性正弦与低电平倍频叠加，平滑起止和指数衰减包络；Python 标准库离线生成。',
            'reference': '声音节奏仅表达行军、占领与轮次，不复刻 Age of Conquest 商业音频。',
        },
        'format': {'container':'WAV','encoding':'PCM signed little-endian','sampleRate':RATE,'bits':16,'channels':1},
        'entries': [],
    }
    for name in ['march','capture','round']:
        raw,frames = synthesize(name)
        file = name + '.wav'
        (directory / file).write_bytes(raw)
        manifest['entries'].append({
            'file':file,
            'urlPath':'/conquest/v1/audio/' + file,
            'output':'games/conquest/assets/public/conquest/v1/audio/' + file,
            'sha256':hashlib.sha256(raw).hexdigest(),
            'bytes':len(raw),
            'frames':frames,
        })
    (destination/'manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
    return manifest


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output-dir',type=Path,default=Path(__file__).resolve().parent,help='Assets directory inside this worktree')
    arguments = parser.parse_args()
    result = generate(arguments.output_dir)
    print(json.dumps({'version':result['version'],'files':len(result['entries']),'bytes':sum(e['bytes'] for e in result['entries'])}))
