#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-or-later
"""Extract the original front engine loops as verified, lossless FLAC assets.

Uses speedtools only as an offline format reader. No parser is shipped to the
game. The selected BNK loop interval is preserved byte-for-byte, not normalized,
resampled, faded or re-synthesized.
"""

import argparse
import hashlib
import json
import math
from pathlib import Path
import shutil
import subprocess

from speedtools.car_data import CarData
from speedtools.utils import get_path_case_insensitive


CARS=['P911','FM50','LDIA','MCF1','CCOR']
MAPPING_SOURCE='https://github.com/e-rk/HighStakesRE/blob/a87d7c952e36251f5ba371550d09ee26684d82b3/core/car/car_engine_audio.gd'


def sha(data):
    return hashlib.sha256(data).hexdigest()


def encode(value):
    return json.dumps(value,ensure_ascii=False,separators=(',',':'),allow_nan=False).encode()+b'\n'


def export_car(root,ident,output):
    directory=get_path_case_insensitive(root,root/'Data/Cars'/ident)
    source=CarData(directory,root)
    model=ident.lower()
    folder=output/'audio'/model
    folder.mkdir(parents=True)
    performance=source.performance
    idle=performance['engine_minimum_rpm']
    redline=performance['engine_redline_rpm']
    samples=[]
    for engine in source.engine_audio:
        if engine.patchnum>=64:
            continue
        if len(engine.streams)!=1:
            raise ValueError(f'{ident}/{engine.patchnum}: multiple native streams require an explicit selection')
        stream=engine.streams[0]
        if stream.compression.name!='PCM' or stream.num_channels!=1:
            raise ValueError(f'{ident}/{engine.patchnum}: unsupported source compression/channels')
        original=stream.audio_samples
        stride=stream.num_channels*2
        if len(original)!=stream.decompressed_samples*stride:
            raise ValueError(f'{ident}/{engine.patchnum}: source PCM frame count mismatch')
        start=stream.loop_start
        end=stream.loop_end+1 if stream.loop_end is not None else 0
        if not 0<=start<end<=stream.decompressed_samples:
            raise ValueError(f'{ident}/{engine.patchnum}: source loop interval is absent or invalid')
        loop=original[start*stride:end*stride]
        filename=f'engine-{engine.patchnum}.flac'
        target=folder/filename
        command=['ffmpeg','-nostdin','-v','error','-f','s16le','-ar',str(stream.sample_rate),
                 '-ac',str(stream.num_channels),'-i','pipe:0','-map_metadata','-1',
                 '-c:a','flac','-compression_level','12','-sample_fmt','s16',
                 '-metadata_header_padding','0','-fflags','+bitexact','-flags:a','+bitexact',str(target)]
        subprocess.run(command,input=loop,capture_output=True,check=True)
        decoded=subprocess.run(['ffmpeg','-nostdin','-v','error','-i',str(target),
                                '-f','s16le','-acodec','pcm_s16le','pipe:1'],capture_output=True,check=True).stdout
        if decoded!=loop:
            raise ValueError(f'{ident}/{engine.patchnum}: FLAC round-trip changed native PCM')
        tables=[]
        for table in engine.tables:
            if len(table.pitch)!=512 or len(table.volume)!=512:
                raise ValueError(f'{ident}/{engine.patchnum}: incomplete native controller table')
            # Continuous equal-tempered pitch is an independent approximation,
            # not a copy of the game's fixed-point cents lookup table.
            base_cents=stream.detune+(60-stream.root_key)*100
            rates=[round(2**((base_cents+(value-64)*stream.pitch_bend_range*100/64)/1200),8)
                   for value in table.pitch]
            tables.append({'kind':table.table_type.name.lower(), 'pitch':table.pitch,
                           'volume':table.volume, 'rate':rates})
        data=target.read_bytes()
        samples.append({'patch':engine.patchnum,'path':f'audio/{model}/{filename}',
                        'sampleRate':stream.sample_rate,'channels':stream.num_channels,'bits':16,
                        'frames':end-start,'duration':round((end-start)/stream.sample_rate,9),
                        'bytes':len(data),'sha256':sha(data),'pcmSha256':sha(loop),
                        'loopStart':0,'loopEnd':end-start,
                        'rootKey':stream.root_key,'pitchBendRange':stream.pitch_bend_range,
                        'detune':stream.detune,'timbreVolume':stream.volume,
                        'tables':tables,
                        'source':{'pcmSha256':sha(original),'frames':stream.decompressed_samples,
                                  'loopStart':start,'loopEndInclusive':end-1,
                                  'compression':stream.compression.name,
                                  'selection':'front engine patch; original loop markers; no processing'}})
    metadata={'version':1,'id':model,'name':source.fedata.name if source.fedata else ident,
              'idleRPM':idle,'redlineRPM':redline,
              'mapping':{'authority':'community-reconstruction','source':MAPPING_SOURCE,
                         'index0RPM':idle//2,'rpmPerIndex':round(((redline//2+redline)*100)/(115*512),12),
                         'indexCount':512,'indexRounding':'nearest','indexClamp':[0,511],
                         'rateModel':'continuous equal-tempered approximation of native pitch controls',
                         'rateLimit':'original fixed-point cents LUT, random detune and exact log-volume mixing not ported'},
              'source':{'vivSha256':sha(get_path_case_insensitive(directory,directory/'car.viv').read_bytes()),
                        'members':['careng.bnk','careng.ctb','careng.ltb']},
              'samples':samples,
              'limitations':['Only front-engine perspective loops are selected; no rear spatial mix.',
                             'No native runtime audio output measurement is available.',
                             'Loop end follows the native BNK marker as an inclusive sample index.']}
    (folder/'engine.json').write_bytes(encode(metadata))
    return {'metadata':f'audio/{model}/engine.json','loops':len(samples),
            'audioBytes':sum(sample['bytes']for sample in samples),
            'pcmBytes':sum(sample['frames']*sample['channels']*2 for sample in samples)}


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--game-root',type=Path,required=True)
    parser.add_argument('--out',type=Path,required=True)
    parser.add_argument('--cars',nargs='+',default=CARS)
    args=parser.parse_args()
    repository=Path(__file__).resolve().parents[2]
    output=args.out.resolve()
    if not output.is_relative_to(repository):parser.error('output must stay inside this repository')
    if output.exists():parser.error('output must be a new directory; existing data is never overwritten')
    stage=output.with_name(output.name+'.building')
    if stage.exists():parser.error('staging path already exists')
    stage.mkdir(parents=True)
    try:
        models={ident.lower():export_car(args.game_root.resolve(),ident,stage)for ident in args.cars}
        assets=[]
        for path in sorted(p for p in stage.rglob('*')if p.is_file()):
            data=path.read_bytes()
            assets.append({'path':path.relative_to(stage).as_posix(),'bytes':len(data),'sha256':sha(data),
                           'mime':'audio/flac'if path.suffix=='.flac'else'application/json'})
        manifest={'version':1,'generator':'wire-board nfs-audio 1','parser':'speedtools 0.38.0',
                  'encoding':'FLAC lossless from original 16-bit PCM loop intervals',
                  'models':models,'assets':assets,'totalBytes':sum(a['bytes']for a in assets)}
        (stage/'audio-manifest.json').write_bytes(encode(manifest))
        stage.rename(output)
        print(json.dumps({'output':str(output.relative_to(repository)),'assets':len(assets),
                          'loops':sum(m['loops']for m in models.values()),'bytes':manifest['totalBytes']}))
    except BaseException:
        shutil.rmtree(stage)
        raise


if __name__=='__main__':main()
