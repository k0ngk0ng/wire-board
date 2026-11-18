#!/usr/bin/env python3
"""Record original YJ2 output hashes using the user's read-only SDLPAL source.

All generated source, executable and output stay within this workspace. This
format reference never executes the original game executable or a simulator.
"""
import hashlib
import json
import os
from pathlib import Path
import struct
import subprocess

ROOT = Path(__file__).resolve().parents[3]
SOURCE = Path('/Users/jason/Sources/github.com/sdlpal/sdlpal/yj1.c')
ORIGINAL = ROOT / '.local/pal/original-98'
OUT = ROOT / '.local/pal/oracle'
OUT.mkdir(parents=True, exist_ok=True)
source = SOURCE.read_text()
routine = source[source.index('typedef struct _YJ2_TreeNode'):source.index('INT (*Decompress)')]
prefix = '''#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
typedef int INT;
typedef const void *LPCVOID;
typedef void *LPVOID;
#define SDL_SwapLE32(x) (x)
'''
driver = '''
int main(void) {
 unsigned char *in=calloc(1,8*1024*1024), *out=calloc(1,8*1024*1024);
 if (!in || !out) return 2;
 size_t size=fread(in,1,8*1024*1024,stdin);
 if(size<4) return 3;
 unsigned int length=(unsigned int)in[0]|(unsigned int)in[1]<<8|(unsigned int)in[2]<<16|(unsigned int)in[3]<<24;
 if(!length || length>8*1024*1024) return 4;
 int result=YJ2_Decompress(in,out,8*1024*1024);
 if(result!=(int)length) return 5;
 return fwrite(out,1,length,stdout)==length?0:6;
}
'''
(OUT / 'yj2.c').write_text(source[:source.index('#include "common.h"')] + prefix + routine + driver)
env = dict(os.environ, TMPDIR=str(ROOT / '.local/tmp'))
subprocess.run(['cc', '-O2', '-std=c99', str(OUT / 'yj2.c'), '-o', str(OUT / 'yj2-reference')], cwd=ROOT, env=env, check=True)
vectors = []
for name in ['map.mkf', 'mgo.mkf', 'abc.mkf', 'f.mkf', 'fire.mkf', 'fbp.mkf']:
    data = (ORIGINAL / name).read_bytes()
    count = struct.unpack_from('<I', data)[0] // 4 - 1
    indices = sorted({1, count // 3, 2 * count // 3, count - 1})
    for index in indices:
        start, end = struct.unpack_from('<II', data, index * 4)
        chunk = data[start:end]
        if not chunk: continue
        decoded = subprocess.check_output([str(OUT / 'yj2-reference')], input=chunk, env=env)
        vectors.append({'archive': name, 'chunk': index, 'size': len(decoded), 'sha256': hashlib.sha256(decoded).hexdigest()})
manifest = json.loads((ORIGINAL / 'manifest.json').read_text())
result = {'sourceCommit': manifest['commit'], 'referenceSourceSHA256': hashlib.sha256(SOURCE.read_bytes()).hexdigest(),
          'reference': 'SDLPAL yj1.c YJ2_Decompress; GPL-3.0; copyright Wei Mingzhi and SDLPAL development team; portions by Lou Yihua',
          'command': 'python3 -B games/pal/tests/resource_oracle.py', 'vectors': vectors}
target = ROOT / 'games/pal/resources/testdata/yj2-reference.json'
target.parent.mkdir(parents=True, exist_ok=True)
target.write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
print(json.dumps({'vectors': len(vectors), 'fixture': str(target)}))
