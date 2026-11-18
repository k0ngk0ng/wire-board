#!/usr/bin/env python3
"""All original RNG frames through pinned C decompression and delta blitting.

Generated source/executable stay inside the workspace. The original source,
game data, nonzero initial pixels and guard bytes are independent of Go/JS.
"""
import hashlib
import json
import os
from pathlib import Path
import struct
import subprocess

ROOT = Path(__file__).resolve().parents[3]
NATIVE = ROOT / 'games/pal/native'
OUT = ROOT / '.local/pal/rng-reference'


def definition(source, name):
    start = source.index('static INT\n' + name + '(')
    body = source.index('\n{', start) + 1
    depth, end = 1, body + 1
    while depth:
        if source[end] == '{':
            depth += 1
        if source[end] == '}':
            depth -= 1
        end += 1
    return source[start:end]


identity = json.loads((NATIVE / 'reference/manifest.json').read_text())
source = (NATIVE / 'reference/rngplay.c').read_bytes()
assert hashlib.sha256(source).hexdigest() == identity['files'][0]['sha256']
yj = (NATIVE / 'upstream/yj1.c').read_text(encoding='utf-8-sig')
routine = yj[yj.index('typedef struct _YJ2_TreeNode'):yj.index('INT (*Decompress)')]
prefix = '''// GPL-3.0-only; original SDLPAL routines. See native/reference/rngplay.c.
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
typedef int INT;
typedef unsigned char *LPBYTE;
typedef const void *LPCVOID;
typedef void *LPVOID;
typedef struct {void *pixels; int pitch;} SDL_Surface;
#define SDL_SwapLE32(x) (x)
'''
driver = r'''
static uint32_t word(const unsigned char *p){return p[0]|(uint32_t)p[1]<<8|(uint32_t)p[2]<<16|(uint32_t)p[3]<<24;}
static void number(uint32_t n){unsigned char b[4]={n,n>>8,n>>16,n>>24};if(fwrite(b,1,4,stdout)!=4)exit(9);}
int main(int argc,char **argv){
 if(argc!=2)return 2;FILE *f=fopen(argv[1],"rb");if(!f)return 3;
 fseek(f,0,SEEK_END);long length=ftell(f);rewind(f);
 unsigned char *archive=malloc(length),*frame=malloc(65000),*pixels=malloc(65000);
 if(!archive||!frame||!pixels||fread(archive,1,length,f)!=(size_t)length)return 4;fclose(f);
 unsigned clips=word(archive)/4-1;
 SDL_Surface surface={pixels,320};
 for(unsigned clip=0;clip<clips;clip++){
  unsigned char *movie=archive+word(archive+clip*4);unsigned count=word(movie)/4-1;
  for(unsigned fill=0;fill<2;fill++){
   memset(pixels,0xa5,65000);for(unsigned p=0;p<64000;p++)pixels[p]=fill?255:p%251;
   for(unsigned i=0;i<count;i++){
    unsigned begin=word(movie+i*4),end=word(movie+i*4+4);if(begin==end)break;
    int size=YJ2_Decompress(movie+begin,frame,65000);
    if(size<0||PAL_RNGBlitToSurface(frame,size,&surface)!=0)return 5;
    for(unsigned p=64000;p<65000;p++)if(pixels[p]!=0xa5)return 6;
    number(clip);number(i);number(fill);if(fwrite(pixels,1,64000,stdout)!=64000)return 7;
   }
  }
 }
 free(archive);free(frame);free(pixels);return 0;
}
'''
OUT.mkdir(parents=True, exist_ok=True)
generated = OUT / 'rng-reference.c'
generated.write_text(prefix + routine + definition(source.decode('utf-8-sig'), 'PAL_RNGBlitToSurface') + driver)
env = dict(os.environ, TMPDIR=str(ROOT / '.local/tmp'))
executable = OUT / 'rng-reference'
subprocess.run(['cc', '-O2', '-std=c99', str(generated), '-o', str(executable)], cwd=ROOT, env=env, check=True)
process = subprocess.Popen([str(executable), str(ROOT / '.local/pal/original-98/rng.mkf')], stdout=subprocess.PIPE)
vectors = []
while header := process.stdout.read(12):
    assert len(header) == 12
    clip, frame, fill = struct.unpack('<III', header)
    pixels = process.stdout.read(64000)
    assert len(pixels) == 64000
    vectors.append({'clip': clip, 'frame': frame, 'fill': fill, 'sha256': hashlib.sha256(pixels).hexdigest()})
assert process.wait() == 0
assert len(vectors) == 2928
target = ROOT / 'games/pal/resources/testdata/rng-reference.json'
target.write_text(json.dumps({'reference': 'pinned SDLPAL YJ2_Decompress and PAL_RNGBlitToSurface; patterned/255 initial pixels and output guards', 'command': 'python3 -B games/pal/tests/rng-oracle.py', 'sourceSHA256': identity['files'][0]['sha256'], 'clips': 12, 'vectors': vectors}, indent=2) + '\n')
print(json.dumps({'vectors': len(vectors), 'fixture': str(target)}))
