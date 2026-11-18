"""Decode pinned font initializers in PAL_InitFont's ISO-then-CN order.

Bitmap bytes are only used by the client exporter. The rule builder retains
only widths; original C font rendering is a separate verification build.
"""
import re

def numbers(text):
    text=re.sub(r'/\*.*?\*/|//[^\n]*','',text,flags=re.S)
    return [int(value,0) for value in re.findall(r'0x[0-9a-fA-F]+|\b\d+\b',text)]

def array(text,pattern):
    match=re.search(pattern+r'\s*=\s*\{(.*?)\n\};',text,re.S)
    if not match:raise ValueError('pinned font initializer changed: '+pattern)
    return match[1]

def metrics(vendor):
    text=(vendor/'fontglyph.h').read_text()
    values=numbers(array(text,r'unsigned char font_width\[65536\]'))
    if len(values)>65536:raise ValueError('invalid original font widths')
    values.extend([0]*(65536-len(values)))
    values[:256]=[16]*256
    for code in re.findall(r'\{\s*(0x[0-9a-fA-F]+),\s*\{',(vendor/'fontglyph_cn.h').read_text()):
        value=int(code,16);value=value-0xf900+0xd800 if value>=0xf900 else value
        values[value]=32
    return bytes(values)

def glyph_data(vendor):
    text=(vendor/'fontglyph.h').read_text()
    rows=re.findall(r'\{([^{}]*)\}',array(text,r'static unsigned char unicode_font\[65536\]\[32\]'))
    if len(rows)>65536:raise ValueError('invalid original glyph count')
    glyphs=bytearray(65536*32)
    for i,row in enumerate(rows):
        values=numbers(row)
        if len(values)>32:raise ValueError('invalid original glyph width')
        glyphs[i*32:i*32+len(values)]=bytes(values)
    ascii=(vendor/'ascii.h').read_text()
    iso=numbers(array(ascii,r'static unsigned char iso_font\[\]'))
    if len(iso)!=256*15:raise ValueError('invalid original ISO font')
    for i in range(256):
        for y in range(15):glyphs[i*32+y]=int(f'{iso[i*15+y]:08b}'[::-1],2)
        glyphs[i*32+15]=0
    for code,row in re.findall(r'\{\s*(0x[0-9a-fA-F]+),\s*\{([^{}]+)\}\s*\}',(vendor/'fontglyph_cn.h').read_text()):
        code=int(code,16);code=code-0xf900+0xd800 if code>=0xf900 else code
        values=bytes(numbers(row))
        if len(values)!=32:raise ValueError('invalid original CN glyph')
        glyphs[code*32:(code+1)*32]=values
    small=bytes(numbers(array(ascii,r'static unsigned char iso_font_8x8\[256\]\[8\]')))
    if len(small)!=256*8:raise ValueError('invalid original 8px font')
    return bytes(glyphs),metrics(vendor),small
