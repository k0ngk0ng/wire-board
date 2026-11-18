#!/usr/bin/env python3
"""Build the pinned SDLPAL rule core with the platform's in-memory bridge.

Generated objects and Wasm stay in .local; no client runtime is embedded into
the server. --vendor-from reads a local source checkout without changing it.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
from font_data import metrics
from runtime_archive import PURPOSES, output_directory, read_stamp, retain_runtime, stamp_wasm

ROOT = Path(__file__).resolve().parents[3]
NATIVE = ROOT / 'games/pal/native'
SOURCES = ['global.c', 'script.c', 'play.c', 'game.c', 'fight.c', 'battle.c', 'palcommon.c', 'map.c', 'yj1.c', 'res.c', 'scene.c', 'palette.c', 'ui.c', 'uigame.c', 'itemmenu.c', 'magicmenu.c', 'uibattle.c']

def original_definition(source,name):
    match=re.search(r'(?m)^(?:static )?(?:VOID|void|BOOL|WORD|INT|int)\s+'+name+r'\(',source)
    if not match:raise SystemExit('original function definition changed: '+name)
    body=source.index('\n{',match.end())+1;depth=1;end=body+1
    while depth and end<len(source):
        if source[end]=='{':depth+=1
        if source[end]=='}':depth-=1
        end+=1
    if depth:raise SystemExit('unbalanced original function: '+name)
    return match.start(),body,end

def observe_menu(source,name,kind,args='',role='-1'):
    start,body,end=original_definition(source,name)
    head=source[start:body].split('/*++')[0].rstrip()
    original=source[start:end].replace(name+'(','WB_Original'+name+'(',1)
    returns=bool(re.match(r'(?:static )?(?:WORD|BOOL|INT|int)\b',head))
    call='WB_Original'+name+'('+args+')'
    wrapper=head+'\n{\n wb_menu_push('+str(kind)+','+role+');\n '
    wrapper+=('int result='+call+';' if returns else call+';')+'\n wb_menu_pop();\n'
    if returns:wrapper+=' return result;\n'
    wrapper+='}\n'
    return source[:start]+original+'\n'+wrapper+source[end:]


def tools():
    base = ROOT / '.local/pal/toolchains'
    sdk = Path(os.environ['PAL_WASI_SDK']) if os.environ.get('PAL_WASI_SDK') else next(base.glob('wasi-sdk-34.0-*/bin/clang'), None)
    if sdk and sdk.is_dir():
        sdk = sdk / 'bin/clang'
    optimizer = Path(os.environ.get('PAL_WASM_OPT', str(base / 'binaryen-version_133/bin/wasm-opt')))
    if not sdk or not sdk.is_file() or not optimizer.is_file():
        raise SystemExit('WASI SDK 34 and Binaryen 133 are required under .local/pal/toolchains or PAL_WASI_SDK/PAL_WASM_OPT')
    return sdk, optimizer


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--vendor-from', type=Path)
    parser.add_argument('--reference-font', action='store_true', help='build a verification-only core with the original C font rasterizer')
    parser.add_argument('--battle-fixture', type=int, help='verification only: call original PAL_StartBattle for this enemy team')
    parser.add_argument('--rng-fixture', type=int, choices=[1], help='verification only: original sword lesson RNG ranges')
    parser.add_argument('--ending-fixture', action='store_true', help='verification only: original Windows ending sequence; never a campaign runtime')
    parser.add_argument('--ending-credits', action='store_true', help='verification only: include the original SDLPAL additional credit page after the ending sequence')
    parser.add_argument('--game-over-fixture', action='store_true', help='verification only: original failure script and held reload request')
    parser.add_argument('--fixture-party', type=int, choices=[1,2,3], default=3)
    parser.add_argument('--fixture-roles', help='verification only: explicit original role IDs, e.g. 0,2; determines party size')
    parser.add_argument('--fixture-field', type=int, default=0, help='verification only: original battlefield ID (default original initial value 0)')
    parser.add_argument('--test-battle-hp', type=int, help='development only: refill party HP/maxHP at battle entry; separate build and incompatible saves')
    parser.add_argument('--out', type=Path, help='candidate output directory inside the workspace; active cores are not replaced when using a separate directory')
    args = parser.parse_args()
    if args.game_over_fixture and (args.ending_fixture or args.ending_credits or args.rng_fixture is not None or args.battle_fixture is not None or args.test_battle_hp is not None):
        parser.error('game-over fixture requires an independent original-profile build')
    if args.ending_credits and not args.ending_fixture:
        parser.error('ending credits fixture requires an independent ending fixture')
    if args.ending_fixture and (args.reference_font or args.rng_fixture is not None or args.battle_fixture is not None or args.test_battle_hp is not None):
        parser.error('ending fixture must use an independent original-profile build')
    if args.test_battle_hp is not None:
        if not 1000 <= args.test_battle_hp <= 9999:
            parser.error('test battle HP must be between 1000 and 9999')
        if args.reference_font or args.rng_fixture is not None:
            parser.error('development health cannot be used for reference/RNG acceptance')
    if args.rng_fixture is not None and args.battle_fixture is not None:
        parser.error('RNG and battle fixtures are independent')
    if args.battle_fixture is not None and not 0<=args.battle_fixture<500:
        parser.error('battle fixture team is outside the original table bounds')
    if not 0<=args.fixture_field<58 or (args.fixture_field and args.battle_fixture is None):
        parser.error('fixture field requires a battle fixture and original battlefield ID 0-57')
    roles=list(range(args.fixture_party))
    if args.fixture_roles is not None:
        if args.battle_fixture is None or not re.fullmatch(r'[0-5](,[0-5]){0,2}',args.fixture_roles):
            parser.error('fixture roles require a battle fixture and 1-3 original role IDs')
        roles=[int(role) for role in args.fixture_roles.split(',')]
        if len(set(roles))!=len(roles):parser.error('fixture roles cannot repeat')
        args.fixture_party=len(roles)
    compiler, optimizer = tools()
    vendor = NATIVE / 'upstream'
    common = ['-O1', '-g0', '-ffunction-sections', '-fdata-sections', '-fno-strict-aliasing', '-fwrapv', '-ffile-prefix-map=' + str(ROOT) + '=.',
              '-DPAL_CLASSIC=1', '-DPAL_HAS_MP3=0', '-DPAL_HAS_OGG=0', '-DPAL_HAS_OPUS=0', '-DPAL_HAS_GLSL=0',
              '-Dfread=wb_fread', '-Dfwrite=wb_fwrite', '-Dfseek=wb_fseek', '-Dftell=wb_ftell', '-Dfclose=wb_fclose',
              '-Dfree=wb_free',
              '-I' + str(NATIVE / 'include')]
    if args.vendor_from:
        source = args.vendor_from.resolve()
        commit = subprocess.check_output(['git', '-C', str(source), 'rev-parse', 'HEAD'], text=True).strip()
        if subprocess.check_output(['git', '-C', str(source), 'status', '--porcelain'], text=True).strip():
            raise SystemExit('reference checkout has local changes; cannot claim a pinned source identity')
        paths = set(SOURCES + ['LICENSE', 'video.c', 'text.c', 'font.c', 'fontglyph.h', 'fontglyph_cn.h', 'ascii.h', 'palcfg.c'])
        for name in SOURCES:
            command = [str(compiler), *common, '-I' + str(source), '-MM', str(source / name)]
            dependencies = subprocess.check_output(command, text=True).replace('\\\n', ' ').split()[1:]
            for dependency in dependencies:
                candidate = Path(dependency)
                if candidate.is_relative_to(source):
                    paths.add(str(candidate.relative_to(source)))
        identities = []
        for name in sorted(paths):
            data = (source / name).read_bytes()
            target = vendor / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(data)
            identities.append({'path': name, 'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()})
        (NATIVE / 'upstream-manifest.json').write_text(json.dumps({'repository': 'sdlpal/sdlpal', 'commit': commit, 'license': 'GPL-3.0-only', 'files': identities}, indent=2) + '\n')
    identity = json.loads((NATIVE / 'upstream-manifest.json').read_text())
    for item in identity['files']:
        data = (vendor / item['path']).read_bytes()
        if len(data) != item['size'] or hashlib.sha256(data).hexdigest() != item['sha256']:
            raise SystemExit('upstream source identity mismatch: ' + item['path'])
    out = ROOT / ('.local/pal/native-reference' if args.reference_font else '.local/pal/native')
    if args.game_over_fixture:
        out=ROOT/'.local/pal/game-over-fixture'/('reference' if args.reference_font else 'original')
    if args.rng_fixture is not None:
        out=ROOT/'.local/pal/rng-fixtures'/(str(args.rng_fixture)+('-reference' if args.reference_font else ''))
        common+=['-DWB_RNG_FIXTURE_CLIP='+str(args.rng_fixture)]
    if args.battle_fixture is not None:
        out=ROOT/'.local/pal/battle-fixtures'/('team-'+str(args.battle_fixture)+'-party-'+str(args.fixture_party)+('-roles-'+'-'.join(map(str,roles)) if args.fixture_roles is not None else '')+('-field-'+str(args.fixture_field) if args.fixture_field else '')+('-reference' if args.reference_font else ''))
        common+=['-DWB_BATTLE_FIXTURE_TEAM='+str(args.battle_fixture),'-DWB_BATTLE_FIXTURE_PARTY='+str(args.fixture_party),'-DWB_BATTLE_FIXTURE_ROLES='+','.join(map(str,roles)),'-DWB_BATTLE_FIXTURE_FIELD='+str(args.fixture_field)]
    if args.test_battle_hp is not None:
        out=ROOT/'.local/pal/test-health'/('hp-'+str(args.test_battle_hp))/('campaign' if args.battle_fixture is None else 'battle-'+out.name)
    if args.out is not None:
        try:
            out=output_directory(ROOT,args.out)
        except ValueError as error:
            parser.error(str(error))
    purpose='game-over-fixture' if args.game_over_fixture else 'ending-fixture' if args.ending_fixture else 'font-reference' if args.reference_font else 'rng-fixture' if args.rng_fixture is not None else 'battle-fixture' if args.battle_fixture is not None else 'campaign'
    profile='original-98' if args.test_battle_hp is None else 'development-health:'+str(args.test_battle_hp)
    stamp={'format':1,'profile':profile,'purpose':purpose,'sourceCommit':identity['commit']}
    common+=['-DWB_BUILD_PURPOSE='+str(PURPOSES[purpose]),'-DWB_TEST_BATTLE_HP='+str(args.test_battle_hp or 0)]
    if args.ending_fixture:common+=['-DWB_ENDING_FIXTURE=1']
    if args.ending_credits:common+=['-DWB_ENDING_CREDITS_FIXTURE=1']
    if args.game_over_fixture:common+=['-DWB_GAME_OVER_FIXTURE=1']
    if (out/'rules.wasm').exists() and (out/'build.json').exists():
        old_metadata=json.loads((out/'build.json').read_text())
        # Historical isolated fixtures were never campaign runtimes. Keep
        # every classified campaign/new artifact before replacing its output.
        if read_stamp((out/'rules.wasm').read_bytes()) is not None or (old_metadata.get('battleFixture') is None and old_metadata.get('rngFixture') is None and not old_metadata.get('referenceFont') and 'original font raster' not in old_metadata.get('scope','')):
            retain_runtime(ROOT,out/'rules.wasm',out/'build.json')
    out.mkdir(parents=True, exist_ok=True)
    (out/'derived').mkdir(exist_ok=True)
    if args.reference_font:common+=['-DWB_REFERENCE_FONT=1']
    common += ['-I' + str(vendor), '-I' + str(out / 'derived')]
    objects = []
    commands = []
    video=(vendor/'video.c').read_text()
    effects='#include "main.h"\n#include "wb_video.h"\n#include "wb_raster.h"\n'
    for name in ['VIDEO_UpdateScreen','VIDEO_FadeScreen','VIDEO_ShakeScreen','VIDEO_SwitchScreen']:
        definition=re.search(r'(?m)^VOID\n'+name+r'\(',video)
        if not definition: raise SystemExit('original video effect changed: '+name)
        body=video.index('\n{',definition.end())+1
        depth=1;end=body+1
        while depth and end<len(video):
            if video[end]=='{': depth+=1
            if video[end]=='}': depth-=1
            end+=1
        if depth: raise SystemExit('invalid original video effect body')
        piece=video[definition.start():end]
        if name=='VIDEO_UpdateScreen':piece=piece.replace(name+'(','WB_Original'+name+'(',1)
        if name=='VIDEO_FadeScreen':
            needle='         for (k = rgIndex[j]; k < gpScreen->pitch * gpScreen->h; k += 6)'
            if piece.count(needle)!=1:raise SystemExit('original physical fade blending changed')
            piece=piece.replace(needle,'         wb_raster_blend_surfaces(gpScreen,gpScreenBak,i,rgIndex[j]);\n'+needle)
            for needle in ['            SDL_SoftStretch(gpScreenBak, &srcrect, gpScreenReal, &dstrect);','            SDL_SoftStretch(gpScreenBak, NULL, gpScreenReal, &dstrect);']:
                if piece.count(needle)!=1:raise SystemExit('original physical fade presentation changed')
                piece=piece.replace(needle,'            wb_raster_begin_display(0,1);\n'+needle)
        effects+=piece+'\n'
    video_source=out/'derived'/'video-effects.c'
    video_source.parent.mkdir(parents=True,exist_ok=True);video_source.write_text(effects)
    rng_identity=json.loads((NATIVE/'reference/manifest.json').read_text())
    rng_item=next(item for item in rng_identity['files'] if item['path']=='rngplay.c')
    rng_bytes=(NATIVE/'reference/rngplay.c').read_bytes()
    if rng_identity['commit']!=identity['commit'] or len(rng_bytes)!=rng_item['size'] or hashlib.sha256(rng_bytes).hexdigest()!=rng_item['sha256']:
        raise SystemExit('original RNG source identity mismatch')
    rng=rng_bytes.decode('utf-8-sig').replace('\r\n','\n')
    start,body,end=original_definition(rng,'PAL_RNGPlay')
    rng=rng[:body+1]+'\n   int wb_first_frame=iStartFrame;\n   wb_rng_begin();'+rng[body+1:]
    needle='      VIDEO_UpdateScreen(NULL);'
    if rng.count(needle)!=1:raise SystemExit('original RNG frame observation changed')
    rng=rng.replace(needle,'      wb_rng_frame(iNumRNG,wb_first_frame,iStartFrame);\n'+needle)
    needle='\t  PAL_DelayUntilPC(iTime);'
    if rng.count(needle)!=1:raise SystemExit('original RNG timing changed')
    rng=rng.replace(needle,'      wb_rng_wait_begin(iTime);\n'+needle+'\n      wb_rng_wait_end();')
    rng_source=out/'derived/rngplay.c';rng_source.write_text('#include "wb_rng.h"\n'+rng)
    # Keep the original text functions and their ordering. Only presentation
    # observations and an authenticated shared-read boundary are inserted.
    original_text=(vendor/'text.c').read_text()
    first=original_text.index('LPWSTR\nPAL_UnescapeText(')
    last=original_text.index('\nWCHAR\nPAL_GetInvalidChar(',first)
    text=original_text[first:last]
    text=text.replace('PAL_ClearDialog(\n','WB_OriginalClearDialog(\n',1)
    edits=[
      ('   g_TextLib.bIcon = 0;\n   g_TextLib.posIcon = 0;\n   g_TextLib.nCurrentDialogLine = 0;', '   wb_dialog_start(iNumCharFace);\n   g_TextLib.bIcon = 0;\n   g_TextLib.posIcon = 0;\n   g_TextLib.nCurrentDialogLine = 0;'),
      ('   PAL_ClearKeyState();\n\n   while (TRUE)', '   PAL_ClearKeyState();\n   wb_dialog_wait(fMaxSeconds);\n\n   while (TRUE)'),
      ('   g_TextLib.fUserSkip = FALSE;\n}\n\nstatic VOID\nPAL_DialogWaitForKey(', '   g_TextLib.fUserSkip = FALSE;\n   wb_dialog_wait_end();\n}\n\nstatic VOID\nPAL_DialogWaitForKey('),
      ('      VIDEO_RestoreScreen(gpScreen);\n      VIDEO_UpdateScreen(NULL);', '      VIDEO_RestoreScreen(gpScreen);\n      wb_dialog_page();\n      VIDEO_UpdateScreen(NULL);'),
      ('   x = PAL_X(g_TextLib.posDialogText);', '   wb_dialog_message(lpszText);\n   x = PAL_X(g_TextLib.posDialogText);'),
    ]
    for before,after in edits:
        if text.count(before)!=1:raise SystemExit('original text observation site changed: '+before)
        text=text.replace(before,after)
    # Original battle rewards/level messages format localized wide strings.
    # Keep the reference formatter, including its bounds/format semantics;
    # unresolved dependencies still use the host's explicit unsupported trap.
    start,body,end=original_definition(original_text,'PAL_swprintf')
    text+='\n'+original_text[start:end]+'\n'
    prelude='''#include "main.h"
#include "wb_ui.h"
#include <errno.h>
#define FONT_COLOR_DEFAULT 0x4F
#define FONT_COLOR_YELLOW 0x2D
#define FONT_COLOR_RED 0x1A
#define FONT_COLOR_CYAN 0x8D
#define FONT_COLOR_CYAN_ALT 0x8C
#define FONT_COLOR_RED_ALT 0x17
BOOL g_fUpdatedInBattle = FALSE;
static wchar_t internal_wbuffer[PAL_GLOBAL_BUFFER_SIZE];
'''
    text_source=out/'derived'/'text-dialogue.c';text_source.write_text(prelude+text)
    values=metrics(vendor)
    (out/'derived'/'wb_font_metrics.h').write_text('static const unsigned char wb_font_width[65536]={'+','.join(map(str,values))+'};\n')
    config=(vendor/'palcfg.c').read_text()
    layout=config[config.index('static const SCREENLAYOUT screen_layout = {'):config.index('\n\tfor (PALCFG_ITEM i = PALCFG_ALL_MIN')]
    (out/'derived'/'wb_screen_layout.h').write_text('static void wb_init_screen_layout(void){\n'+layout+'\ngConfig.ScreenLayout=screen_layout;gConfig.dwWordLength=10;\n}\n')
    extra=[]
    if args.reference_font:
        font=(vendor/'font.c').read_text()
        reference='''#include "main.h"
#define _FONT_C
#include "fontglyph.h"
#include "fontglyph_cn.h"
#include "ascii.h"
static int _font_height=16;
static char offset_x[65536],offset_y[65536];
char *font_offset_x=offset_x,*font_offset_y=offset_y;
'''
        for name in ['reverseBits','PAL_LoadISOFont','PAL_DrawCharOnSurface','PAL_CharWidth']:
            definition=re.search(r'(?m)^(?:static )?(?:uint8_t|void|int)\s+'+name+r'\(',font)
            if not definition:raise SystemExit('original font definition changed: '+name)
            start=font.index('{',definition.end());depth=1;end=start+1
            while depth and end<len(font):
                if font[end]=='{':depth+=1
                if font[end]=='}':depth-=1
                end+=1
            body=font[definition.start():end]
            if name=='PAL_DrawCharOnSurface':body=body.replace(name,'WB_ReferenceDrawCharOnSurface',1)
            if name=='PAL_CharWidth':body=body.replace(name,'WB_ReferenceCharWidth',1)
            reference+=body+'\n'
        macro=font[font.index('#define PAL_LOAD_INTERNAL_FONT'):font.index('    size_t fonts')]
        reference+=macro+'\nvoid WB_InitReferenceFont(void){PAL_LoadISOFont();PAL_LOAD_INTERNAL_FONT(fontglyph_cn,16);}\n'
        reference_source=out/'derived'/'font-reference.c';reference_source.write_text(reference);extra=[reference_source]
    ending_item=next(item for item in rng_identity['files'] if item['path']=='ending.c')
    ending_bytes=(NATIVE/'reference/ending.c').read_bytes()
    if len(ending_bytes)!=ending_item['size'] or hashlib.sha256(ending_bytes).hexdigest()!=ending_item['sha256']:
        raise SystemExit('original ending source identity mismatch')
    for source in [*(vendor / name for name in SOURCES), NATIVE / 'bridge.c',NATIVE/'audio.c',NATIVE/'raster.c',NATIVE/'ending_media.c',NATIVE/'game_over.c',NATIVE/'battle_assets.c',NATIVE/'reference/ending.c',video_source,text_source,rng_source,*extra]:
        target = out / (source.name + '.o')
        if source.name == 'script.c':
            # Track original trigger depth without changing its instructions or
            # recursive call sites. Original source and transformation stay explicit.
            text, count = re.subn(r'(?m)^PAL_RunTriggerScript\(', 'WB_OriginalRunTriggerScript(', source.read_text(encoding='utf-8-sig'))
            if count != 1:
                raise SystemExit('original trigger definition changed; review adapter')
            text=observe_menu(text,'PAL_AdditionalCredits',13)
            derived = out / 'derived' / source.name
            derived.parent.mkdir(parents=True, exist_ok=True)
            derived.write_text('#include "main.h"\n#include "wb_ui.h"\n'+text)
            source = derived
        elif source.name=='global.c':
            text,count=re.subn(r'(?m)^PAL_ReloadInNextTick\(', 'WB_OriginalPAL_ReloadInNextTick(',source.read_text(encoding='utf-8-sig'))
            if count!=1:raise SystemExit('original native reload definition changed')
            derived=out/'derived'/source.name;derived.write_text(text);source=derived
        elif source.name=='palette.c':
            text=source.read_text()
            for name in ['PAL_SetPalette','PAL_FadeIn','PAL_FadeOut','PAL_SceneFade','PAL_FadeToRed']:
                start,body,end=original_definition(text,name);piece=text[start:end]
                if name=='PAL_SetPalette':piece=piece.replace('      VIDEO_SetPalette(p);','      wb_palette_set(iPaletteNum,fNight,64);\n      VIDEO_SetPalette(p);')
                elif name=='PAL_FadeIn':
                    piece=piece.replace('      VIDEO_SetPalette(newpalette);','      wb_palette_set(iPaletteNum,fNight,j);\n      VIDEO_SetPalette(newpalette);')
                    piece=piece.replace('   VIDEO_SetPalette(palette);','   wb_palette_set(iPaletteNum,fNight,64);\n   VIDEO_SetPalette(palette);')
                elif name=='PAL_FadeOut':
                    offset=body-start+1;piece=piece[:offset]+'\n   wb_palette_begin_out();'+piece[offset:]
                    piece=piece.replace('      VIDEO_SetPalette(newpalette);','      wb_palette_out(j);\n      VIDEO_SetPalette(newpalette);')
                    piece=piece.replace('   VIDEO_SetPalette(newpalette);\n}', '   wb_palette_out(0);\n   VIDEO_SetPalette(newpalette);\n}')
                elif name=='PAL_FadeToRed':
                    offset=body-start+1
                    piece=piece[:offset]+'\n   wb_game_over_begin();\n   wb_raster_replace_index(gpScreen,0x4F,0x4E);'+piece[offset:]
                    piece=piece.replace('      VIDEO_SetPalette(newpalette);','      wb_palette_red(gpGlobals->wNumPalette,gpGlobals->fNightPalette,i+1);\n      VIDEO_SetPalette(newpalette);')
                else:piece=piece.replace('         VIDEO_SetPalette(newpalette);','         wb_palette_set(iPaletteNum,fNight,i);\n         VIDEO_SetPalette(newpalette);')
                if piece==text[start:end]:raise SystemExit('original palette observation site changed: '+name)
                text=text[:start]+piece+text[end:]
            derived=out/'derived'/source.name;derived.write_text('#include "main.h"\n#include "wb_palette.h"\n#include "wb_game_over.h"\n#include "wb_raster.h"\n'+text);source=derived
        elif source.name == 'scene.c':
            text=source.read_text()
            # Observe the original private walk phase without changing its
            # initialization or any gesture operation. Prediction needs both
            # idle phases, which cannot be inferred from the standing sprite.
            phase='   static int       s_iThisStepFrame = 0;'
            if text.count(phase)!=1:raise SystemExit('original walking phase changed')
            text=text.replace(phase,'')
            text='static int s_iThisStepFrame = 0;\nint wb_walk_phase(void){return s_iThisStepFrame;}\n'+text
            start,body,end=original_definition(text,'PAL_UpdateParty')
            piece=text[start:end]
            if piece.count('g_InputState.dir')!=7:raise SystemExit('original party direction input changed')
            piece=piece.replace('g_InputState.dir','direction')
            offset=body-start+1
            piece=piece[:offset]+'\n   PALDIRECTION direction = wb_input_direction();'+piece[offset:]
            text=text[:start]+piece+text[end:]
            # A campaign battle leaves the offscreen graph active. Reset it
            # before the first field tile is blitted, not after map rendering.
            start,body,end=original_definition(text,'PAL_MakeScene')
            text=text[:body+1]+'\n   wb_scene_reset();'+text[body+1:]
            edits=[
                ('   a = index;','   if(lpSurface==gpScreen&&!gpGlobals->fInBattle)wb_scene_wave(gpGlobals->wScreenWave,index,gpGlobals->sWaveProgression);\n   wb_raster_wave(lpSurface,gpGlobals->wScreenWave,index);\n   a = index;'),
                ('      PAL_AddSpriteToDraw(lpBitmap,', '''      wb_scene_sprite(i <= gpGlobals->wMaxPartyMemberIndex ? gpGlobals->g.PlayerRoles.rgwSpriteNum[gpGlobals->rgParty[i].wPlayerRole] : gpGlobals->rgParty[i].wPlayerRole,
         gpGlobals->rgParty[i].wFrame, gpGlobals->rgParty[i].wPlayerRole, i <= gpGlobals->wMaxPartyMemberIndex ? 0 : 2,
         gpGlobals->rgParty[i].x - PAL_RLEGetWidth(lpBitmap) / 2, gpGlobals->rgParty[i].y + 4,
         gpGlobals->rgParty[i].y + gpGlobals->wLayer + 10);
      PAL_AddSpriteToDraw(lpBitmap,'''),
                ('      PAL_AddSpriteToDraw(lpFrame, x, y, lpEvtObj->sLayer * 8 + 2);', '''      wb_scene_sprite(lpEvtObj->wSpriteNum, lpEvtObj->wDirection * lpEvtObj->nSpriteFrames + iFrame,
         i + 1, 1, x, y - lpEvtObj->sLayer * 8 - 2, y);
      PAL_AddSpriteToDraw(lpFrame, x, y, lpEvtObj->sLayer * 8 + 2);'''),
                ('               if (lpTile != NULL && iTileHeight > 0 && (dy + iTileHeight) * 16 + dh * 8 >= sy)', '''               if (iTileHeight > 0 && (dy + iTileHeight) * 16 + dh * 8 >= sy)
                  wb_scene_cover(dx, dy, dh, l, iTileHeight);
               if (lpTile != NULL && iTileHeight > 0 && (dy + iTileHeight) * 16 + dh * 8 >= sy)'''),
            ]
            for before,after in edits:
                if text.count(before)!=1:raise SystemExit('original scene observation site changed')
                text=text.replace(before,after)
            derived=out/'derived'/source.name;derived.parent.mkdir(parents=True,exist_ok=True)
            derived.write_text('#include "wb_scene.h"\n#include "main.h"\n#include "wb_ui.h"\n#include "wb_raster.h"\n'+text);source=derived
        elif source.name=='battle.c':
            text=source.read_text()
            if args.test_battle_hp is not None:
                # Explicit user-authorized development fixture. The normal
                # build, upstream files and original resources stay unchanged.
                needle='   SHORT          sPrevWaveProgression;'
                if text.count(needle)!=1:raise SystemExit('original battle entry changed; review development-health fixture')
                health='\n   // DEVELOPMENT ONLY: not original-difficulty acceptance.\n'
                health+='   for (i = 0; i <= gpGlobals->wMaxPartyMemberIndex; i++)\n   {\n'
                health+='      w = gpGlobals->rgParty[i].wPlayerRole;\n'
                health+='      gpGlobals->g.PlayerRoles.rgwMaxHP[w] = '+str(args.test_battle_hp)+';\n'
                health+='      gpGlobals->g.PlayerRoles.rgwHP[w] = '+str(args.test_battle_hp)+';\n   }\n'
                text=text.replace(needle,needle+health)
            edits=[
                ('   pSrc = g_Battle.lpBackground->pixels;','   wb_raster_background();\n   pSrc = g_Battle.lpBackground->pixels;'),
                ('         for (k = rgIndex[j]; k < gpScreen->pitch * gpScreen->h; k += 6)','         wb_raster_blend(i,rgIndex[j]);\n         for (k = rgIndex[j]; k < gpScreen->pitch * gpScreen->h; k += 6)'),
            ]
            for before,after in edits:
                if text.count(before)!=1:raise SystemExit('original battle presentation site changed')
                text=text.replace(before,after)
            derived=out/'derived'/source.name;derived.write_text('#include "main.h"\n#include "wb_raster.h"\n'+text);source=derived
        elif source.name == 'palcommon.c':
            text=source.read_text()
            edits=[
              ('      return (int)fread(lpBuffer, 1, uiChunkLen, fp);','      int read = (int)fread(lpBuffer, 1, uiChunkLen, fp);\n      wb_ui_resource(lpBuffer, read, uiChunkNum, fp);\n      return read;'),
            ]
            for before,after in edits:
                if text.count(before)!=1:raise SystemExit('original UI observation site changed')
                text=text.replace(before,after)
            text,count=re.subn(r'(?m)^PAL_RLEBlitToSurfaceWithShadow\(', 'WB_OriginalRLEBlitToSurfaceWithShadow(',text)
            if count!=1:raise SystemExit('original RLE definition changed')
            for name in ['PAL_MKFDecompressChunk','PAL_FBPBlitToSurface','PAL_RLEBlitWithColorShift','PAL_RLEBlitMonoColor']:
                text,count=re.subn(r'(?m)^'+name+r'\(', 'WB_Original'+name+'(',text)
                if count!=1:raise SystemExit('original image definition changed: '+name)
            derived=out/'derived'/source.name;derived.write_text('#include "main.h"\n#include "wb_ui.h"\n'+text);source=derived
        elif source.name in ('fight.c','uibattle.c'):
            text=source.read_text()
            if source.name=='fight.c':
                text,count=re.subn(r'(?m)^PAL_BattleCommitAction\(', 'WB_OriginalBattleCommitAction(',text)
                if count!=1:raise SystemExit('original battle commit definition changed')
                edits=[
                    ('if (g_InputState.dwKeyPress & kKeyRepeat)\n      {\n         g_Battle.fRepeat = TRUE;', 'if (!wb_multiplayer() && (g_InputState.dwKeyPress & kKeyRepeat))\n      {\n         g_Battle.fRepeat = TRUE;'),
                    ('else if (g_InputState.dwKeyPress & kKeyForce)\n      {\n         g_Battle.fForce = TRUE;', 'else if (!wb_multiplayer() && (g_InputState.dwKeyPress & kKeyForce))\n      {\n         g_Battle.fForce = TRUE;'),
                    ('if (g_Battle.UI.wActionType == kBattleActionFlee)\n   {\n      g_Battle.fFlee = TRUE;', 'if (!wb_multiplayer() && g_Battle.UI.wActionType == kBattleActionFlee)\n   {\n      g_Battle.fFlee = TRUE;'),
                ]
            else:
                edits=[
                    *[(f'g_InputState.dir == kDir{direction}', f'wb_input_direction() == kDir{direction}') for direction in ['North','South','West','East']],
                    ('if (g_InputState.dwKeyPress & kKeyAuto)', 'if (!wb_multiplayer() && (g_InputState.dwKeyPress & kKeyAuto))'),
                    ('else if (g_InputState.dwKeyPress & kKeyMenu)\n            {\n               g_Battle.rgPlayer', 'else if (!wb_multiplayer() && (g_InputState.dwKeyPress & kKeyMenu))\n            {\n               g_Battle.rgPlayer'),
                    ('g_Battle.UI.fAutoAttack = TRUE;', 'g_Battle.UI.fAutoAttack = !wb_multiplayer();'),
                ]
            for before,after in edits:
                if text.count(before)!=1:raise SystemExit('original multiplayer battle boundary changed: '+before)
                text=text.replace(before,after)
            derived=out/'derived'/source.name;derived.write_text('#include "main.h"\n#include "wb_ui.h"\n'+text);source=derived
        elif source.name in ('uigame.c','itemmenu.c','magicmenu.c','ui.c'):
            text=source.read_text()
            if source.name=='uigame.c':
                text,count=re.subn(r'(?m)^PAL_OpeningMenu\(', 'WB_OriginalOpeningMenu(',text)
                if count!=1:raise SystemExit('original opening menu definition changed')
                start,body,end=original_definition(text,'PAL_SystemMenu')
                text=text[:start]+'static BOOL PAL_SystemMenu(void){return wb_platform_system();}\n'+text[end:]
                start,body,end=original_definition(text,'PAL_QuitGame')
                text=text[:start]+'VOID PAL_QuitGame(void){wb_platform_system();}\n'+text[end:]
                for name,kind,call_args in [('PAL_InGameMenu',1,''),('PAL_PlayerStatus',2,''),('PAL_InGameMagicMenu',3,''),('PAL_InventoryMenu',4,''),('PAL_EquipItemMenu',6,'wItem'),('PAL_ItemUseMenu',7,'wItemToUse'),('PAL_BuyMenu',8,'wStoreNum'),('PAL_SellMenu',9,'')]:
                    text=observe_menu(text,name,kind,call_args)
                needle='      iPlayerRole = gpGlobals->rgParty[iCurrent].wPlayerRole;'
                if text.count(needle)!=1:raise SystemExit('original status role changed')
                text=text.replace(needle,needle+'\n      wb_menu_role(iPlayerRole);')
            elif source.name=='itemmenu.c':text=observe_menu(text,'PAL_ItemSelectMenu',5,'lpfnMenuItemChanged,wItemFlags')
            elif source.name=='magicmenu.c':text=observe_menu(text,'PAL_MagicSelectionMenu',10,'wPlayerRole,fInBattle,wDefaultMagic','wPlayerRole')
            else:
                text,count=re.subn(r'(?m)^PAL_ReadMenu\(', 'WB_OriginalReadMenu(',text)
                if count!=1:raise SystemExit('original read-menu definition changed')
                needle='      SDL_Delay(50);'
                if text.count(needle)!=1:raise SystemExit('original menu clock changed')
                text=text.replace(needle,'      wb_menu_list(rgMenuItem,nMenuItem,wCurrentItem);\n'+needle)
            derived=out/'derived'/source.name;derived.write_text('#include "main.h"\n#include "wb_ui.h"\n'+text);source=derived
        text=source.read_text(encoding='utf-8-sig')
        if re.search(r'\bPAL_DelayUntil\(',text):
            text=re.sub(r'\bPAL_DelayUntil\(', 'WB_DelayUntil(',text)
            derived=out/'derived'/(source.stem+'-wait.c')
            derived.write_text('#include "wb_wait.h"\n'+text)
            source=derived
        command = [str(compiler), *common, '-c', str(source), '-o', str(target)]
        commands.append(command)
        subprocess.run(command, cwd=ROOT, check=True)
        objects.append(str(target))
    plain, result = out / 'plain.wasm', out / 'rules.wasm'
    command = [str(compiler), '-mexec-model=reactor', *objects, '-Wl,--export-all', '-Wl,--allow-undefined',
               '-Wl,--initial-memory='+str(16777216 if args.reference_font else 8388608), '-Wl,--max-memory=134217728', '-Wl,-z,stack-size=262144', '-o', str(plain)]
    commands.append(command)
    subprocess.run(command, cwd=ROOT, check=True)
    command = [str(optimizer), str(plain), '--asyncify', '--pass-arg=asyncify-imports@pal.yield', '-o', str(result)]
    commands.append(command)
    subprocess.run(command, cwd=ROOT, check=True)
    data = stamp_wasm(result.read_bytes(),stamp)
    result.write_bytes(data)
    (out / 'build.json').write_text(json.dumps({'sourceCommit': identity['commit'], 'runtimeSHA256': hashlib.sha256(data).hexdigest(), 'runtimeBytes': len(data), 'testBattleHP': args.test_battle_hp, 'referenceFont': args.reference_font, 'endingFixture': args.ending_fixture, 'endingCreditsFixture': args.ending_credits, 'gameOverFixture': args.game_over_fixture, 'rngFixture': args.rng_fixture, 'battleFixture': args.battle_fixture, 'fixtureParty': args.fixture_party if args.battle_fixture is not None else None, 'fixtureRoles': roles if args.battle_fixture is not None else None, 'fixtureBattleField': args.fixture_field if args.battle_fixture is not None else None, 'scope': 'verification-only original failure script, never a campaign runtime' if args.game_over_fixture else 'verification-only original Windows ending and SDLPAL additional credits; never a campaign runtime' if args.ending_credits else 'verification-only original Windows ending; never a campaign runtime' if args.ending_fixture else 'development-only increased party HP; never original-difficulty acceptance or production' if args.test_battle_hp is not None else 'verification-only original segmented RNG playback' if args.rng_fixture is not None else 'verification-only isolated original battle' if args.battle_fixture is not None else 'verification-only original font raster; never a production payload' if args.reference_font else 'headless original rule core; not yet production game', 'commands': commands}, indent=2) + '\n')
    retain_runtime(ROOT,result,out/'build.json')
    print(json.dumps({'runtime': str(result), 'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest()}))


if __name__ == '__main__':
    main()
