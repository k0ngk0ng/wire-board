"""Read-only bounded original Root-control source audit; never executes a CPU.

Only the existing pure score-data walker is extracted as AST. Its imports,
native sessions and command-line entry points are not evaluated.
"""
import argparse
import ast
from collections import Counter
import gzip
import hashlib
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parents[3]
DOC = ROOT / 'docs/research/md-yuyuhakusho'
ROM_SHA = 'c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06'
FIELDS = [0xf803, 0xf805, 0xf828, 0xf829, 0xf82a, 0xf82b, 0xf82c]
TOKENS = {0xe0:'stereo',0xe1:'detune',0xe2:'control-signal',0xe3:'all-key-off-end',0xe4:'pan-automation',0xe5:'context-level-add',0xe6:'fm-level-add',0xe7:'tie',0xe8:'gate',0xe9:'hardware-lfo',0xea:'shared-tempo',0xeb:'replace-first-queue',0xec:'level-add-no-immediate',0xed:'direct-ym-bank0',0xee:'direct-ym-current-voice',0xef:'fm-patch-load',0xf0:'software-modulation',0xf1:'context-pitch-control',0xf2:'track-end',0xf3:'psg-noise',0xf4:'pitch-control',0xf5:'context-tone-select',0xf6:'jump',0xf7:'loop',0xf8:'call',0xf9:'return',0xfa:'duration-multiplier',0xfb:'transpose',0xfc:'modulation-enable',0xfd:'modulation-disable',0xfe:'fm3-offsets',0xff:'extended'}

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def bind(path):
    return {'path': str(path.relative_to(ROOT)), 'sha256': sha(path), 'bytes': path.stat().st_size}

def read(path):
    raw = path.read_bytes()
    return json.loads(gzip.decompress(raw) if path.suffix == '.gz' else raw)

def fixed(rom, at, expected):
    data = bytes.fromhex(expected)
    assert rom[at:at + len(data)] == data, (hex(at), expected, rom[at:at + len(data)].hex())
    return {'address': at, 'bytes': expected}

def relative(rom, at):
    assert rom[at:at + 2] == bytes.fromhex('6000')
    return at + 2 + int.from_bytes(rom[at + 2:at + 4], 'big', signed=True)

def grammar(rom, coverage, previous, provenance, helper):
    tree = ast.parse(helper.read_text())
    nodes = [node for node in tree.body if isinstance(node, ast.Assign) and any(isinstance(target, ast.Name) and target.id == 'ARG_BYTES' for target in node.targets) or isinstance(node, ast.FunctionDef) and node.name == 'walk_track']
    assert len(nodes) == 2 and not any(isinstance(node, (ast.Import, ast.ImportFrom)) for node in nodes)
    namespace = {}
    exec(compile(ast.Module(body=nodes, type_ignores=[]), str(helper), 'exec'), namespace)
    walk, arg_bytes = namespace['walk_track'], namespace['ARG_BYTES']
    sizes = {**arg_bytes, 0xe4:None, 0xf6:2, 0xf7:4, 0xf8:2, 0xf9:0}
    all_events, headers, tracks = [], [], []
    for kind, codes, table, first in [('music', range(0x81, 0x97), 0x3a492, 0x81), ('primary', range(0xa0, 0xac), 0x3a60c, 0xa0)]:
        definitions = {row['id']: row for row in previous[kind]['definitions']}
        for code in codes:
            entry = table + (code - first) * 4
            base = int.from_bytes(rom[entry:entry + 4], 'big')
            bank = base + int.from_bytes(rom[base:base + 2], 'big')
            if kind == 'music':
                fm, psg = rom[base + 2:base + 4]
                assert (fm, psg) == (6, 3)
                multiplier, count, cursor = rom[base + 4], fm + psg, base + 6
                end = cursor + fm * 4 + psg * 6
            else:
                multiplier, count, cursor = rom[base + 2], rom[base + 3], base + 4
                end = cursor + count * 6
            headers.append({'id':code,'kind':kind,'entry':entry,'address':base,'bank':bank,'bytes':rom[base:end].hex()})
            counts = Counter()
            for voice in range(count):
                width = 4 if kind == 'music' and voice < fm else 6
                record = rom[cursor:cursor + width]
                cursor += width
                offset = record[:2] if kind == 'music' else record[2:4]
                sequence = base + int.from_bytes(offset, 'big')
                result = walk(rom, {'row': voice, 'sequence': sequence, 'duration_multiplier': multiplier})
                commands = result['prefix_commands'] + result['cycle_commands']
                counts.update(TOKENS[event['command']] for event in commands)
                tracks.append({'id':code,'kind':kind,'voice':voice,'sequence':sequence,'commands':len(commands),'notes':result['notes_scanned'],'stopped':result['stopped'],'prefix_quanta':result['cycle_start_quanta'],'cycle_quanta':result['period_quanta']})
                for event in commands:
                    token = event['command']
                    size = (5 if rom[event['source'] + 1] else 1) if token == 0xe4 else sizes[token]
                    all_events.append({'id':code,'voice':voice,'source':event['source'],'quanta':event['quanta'],'command':token,'args':rom[event['source'] + 1:event['source'] + 1 + size].hex()})
            assert dict(counts) == definitions[code]['token_counts'], (kind, code)
    assert headers == provenance['headers'][:34]
    assert all_events == provenance['tracks'], 'Original grammar/provenance drift'
    fight = set(coverage['fight_music_ids']) | set(previous['primary']['ids'])
    nested = [event for event in all_events if event['id'] in fight and event['command'] == 0xeb]
    assert len(nested) == 1 and nested[0]['id'] == 0xab and nested[0]['args'] == '7d'
    result = {'music_definitions':22,'primary_definitions':12,'music_tracks':sum(row['kind']=='music' for row in tracks),'primary_tracks':sum(row['kind']=='primary' for row in tracks),'commands':len(all_events),'fight_catalog_commands':sum(row['id'] in fight for row in all_events),'fight_catalog_tracks':sum(row['id'] in fight for row in tracks),'tracks':tracks,'walk_errors':[],
              'all_E2_events':[row for row in all_events if row['command']==0xe2], 'all_FF_events':[row for row in all_events if row['command']==0xff],
              'fight_E2_events':[row for row in all_events if row['id'] in fight and row['command']==0xe2], 'fight_FF_events':[row for row in all_events if row['id'] in fight and row['command']==0xff],
              'fight_nested_queue':nested,'matches_previous_static_provenance':True,
              'walker_bounds':{'max_commands_per_track':200000,'max_quanta':1000000,'method':'Shared loop/return words and finite/cyclic score-data grammar; no CPU or target execution'},
              'limits':['Negative results are confined to these 22 music and 12 primary definitions under the declared grammar.','The 11 fight music definitions are a subset; arbitrary computed/dynamic score entry is not excluded.']}
    return result

def listing_corpus(paths):
    result = {}
    for path in paths:
        for number, line in enumerate(path.read_text(errors='replace').splitlines(), 1):
            match = re.match(r'^([0-9a-f]{8}) (.+)$', line)
            if match:
                result.setdefault(int(match[1], 16), []).append({'text':match[2],'path':str(path.relative_to(ROOT)),'line':number})
    return result

def references(rom, listing):
    refs = []
    for pc, rows in sorted(listing.items()):
        operands = {int(value, 16) & 0xffffff for value in re.findall(r'0x([0-9a-f]+)', rows[0]['text'])}
        for field in FIELDS:
            if 0xff0000 | field in operands:
                text = rows[0]['text']
                mnemonic = text.split()[0]
                destination = bool(re.search(r'\(0xffff' + f'{field:04x}' + r'\)\.w$', text))
                write = mnemonic.startswith(('clr.', 'bset.', 'bclr.')) or destination and not mnemonic.startswith(('cmp', 'tst.', 'btst.'))
                refs.append({'field':f'{field:04X}','instruction':pc,'text':text,'access':'write-or-RMW' if write else 'read','listing':rows,'rom_prefix':rom[pc:pc+8].hex()})
    return refs

def root_audit(out):
    assert out.parent == DOC and out.name.startswith('AUDIO_ROOT_CONTROL_SOURCE_AUDIT') and out.suffix == '.json'
    rompath = ROOT / '.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin'
    rom = rompath.read_bytes()
    assert sha(rompath) == ROM_SHA
    coverage_path = DOC / 'AUDIO_GAME_DISPATCH_COVERAGE_MANIFEST.json'
    original_path = DOC / 'AUDIO_GAME_DISPATCH_ORIGINAL_IDS.json'
    provenance_path = ROOT / '.local/md-yuyuhakusho/audio-game-dispatch-original/provenance.json'
    helper = ROOT / 'scripts/yuyuhakusho/music_score_structure.py'
    coverage, previous, provenance = read(coverage_path), read(original_path), read(provenance_path)
    assert len(coverage['platform_fight_candidate_ids']) == 139 and len(coverage['fight_music_ids']) == 11 and previous['primary']['ids'] == list(range(0xa0, 0xac))
    grammar_result = grammar(rom, coverage, previous, provenance, helper)
    paths = sorted((DOC / 'static').glob('*.txt'))
    listing = listing_corpus(paths)
    refs = references(rom, listing)
    # FF dispatcher is itself corroborated by the saved original listing.
    assert listing[0x38e3a][0]['text'] == 'jmp (0x38e3e,PC,D0w*0x1)'
    ff = {f'{index:02X}':{'entry':at,'bytes':rom[at:at+4].hex(),'target':relative(rom,at)} for index,at in enumerate(range(0x38e3e,0x38e52,4))}
    assert [row['target'] for row in ff.values()] == [0x39258,0x3927c,0x393ae,0x393c4,0x393ec]
    extension_bytes = fixed(rom, 0x393c4, '4a38f82867000006544c4e7511fc0001f8281038f8298038f82a6600000a11dcf82911dcf82a4e750c380002f8286600000811fc0080f8284e75')
    extension_instructions = [
        (0x393c4,4,'tst.b Mode','read','F828'),(0x393c8,4,'beq.w init-if-zero','branch',None),(0x393cc,2,'addq.w #2,A4','skip-two-arguments',None),(0x393ce,2,'rts','return',None),
        (0x393d0,6,'move.b #1,Mode','write','F828'),(0x393d6,4,'move.b FMDelta,D0','read','F829'),(0x393da,4,'or.b PSGDelta,D0','read','F82A'),(0x393de,4,'bne.w return-without-consuming-arguments','branch',None),
        (0x393e2,4,'move.b (A4)+,FMDelta','write','F829'),(0x393e6,4,'move.b (A4)+,PSGDelta','write','F82A'),(0x393ea,2,'rts','return',None),
        (0x393ec,6,'cmpi.b #2,Mode','read','F828'),(0x393f2,4,'bne.w return','branch',None),(0x393f6,6,'move.b #0x80,Mode','write','F828'),(0x393fc,2,'rts','return',None)]
    extension = [{'instruction':at,'bytes':rom[at:at+width].hex(),'text':text,'access':access,'field':field,'evidence':'Fixed ROM templates reached by the saved FF sub-dispatch table; no new disassembler or execution'} for at,width,text,access,field in extension_instructions]
    accepted_words = {}
    for row in refs:
        target = int(row['field'],16)
        for at in range(row['instruction']+2,row['instruction']+8,2):
            if rom[at:at+2] == target.to_bytes(2,'big'):
                accepted_words[at] = {'instruction':row['instruction'],'evidence':'saved-direct-Ghidra-listing'}
    for row in extension:
        if row['field']:
            target = int(row['field'],16)
            for at in range(row['instruction']+2,row['instruction']+len(bytes.fromhex(row['bytes'])),2):
                if rom[at:at+2] == target.to_bytes(2,'big'):
                    accepted_words[at] = {'instruction':row['instruction'],'evidence':'FF-table-linked fixed ROM template'}
    word_candidates = {}
    for field in FIELDS:
        hits = [at for at in range(0,len(rom)-1,2) if rom[at:at+2] == field.to_bytes(2,'big')]
        word_candidates[f'{field:04X}'] = [{'operand_word':at,'nearby_bytes':rom[max(0,at-6):at+6].hex(),**accepted_words.get(at,{'evidence':'unclassified raw word match; not accepted as code'})} for at in hits]
    controls = {f'{index+1:02X}':relative(rom,0x37d20+index*4) for index in range(5)}
    assert controls == {'01':0x38916,'02':0x38808,'03':0x388ac,'04':0x38a2c,'05':0x38928}
    templates = [fixed(rom,0x38916,'11fc0003f80611fc0028f8044238f8404e75'),fixed(rom,0x38928,'11fc0020f80611fc0032f8044238f8404e75'),fixed(rom,0x38956,'11fc0003f806'),
                 fixed(rom,0x38a2c,'702772004eba00ec41f8f800303c00e7429851c8fffc11fc00fff8094ebaffb060001740'),
                 fixed(rom,0x38a50,'702772004eba00c81038f8003f0041f8f800303c0087429851c8fffc301f11c0f80011fc00fff8094e75'),
                 fixed(rom,0x389fa,'7402702812024eba012c58014eba012651cafff2727f74027040d04276034eba01144eba0180584051cbfff451caffea4e75'),
                 fixed(rom,0x3a18e,'41f900c0001110bc009f10bc00bf10bc00df10bc00ff4e75')]
    full_count = int.from_bytes(rom[0x38a3a:0x38a3c],'big')+1
    music_count = int.from_bytes(rom[0x38a64:0x38a66],'big')+1
    assert full_count == 232 and music_count == 136
    full_start = music_start = 0xff0000 | int.from_bytes(rom[0x38a36:0x38a38],'big')
    assert full_start == 0xff0000 | int.from_bytes(rom[0x38a60:0x38a62],'big')
    known_fields = {0xf800:'Root.Priority',0xf801:'score.TempoRemaining',0xf802:'score.TempoPeriod',0xf803:'E2-signal; no producer in audited grammar',0xf804:'Root.FadeSteps',0xf805:'unassigned; fade-period role not established',0xf806:'Root.FadeRemaining',0xf807:'World.AudioPause',0xf809:'Root.Selected',0xf80a:'World.Queue[0]',0xf80b:'World.Queue[1]',0xf80c:'World.Queue[2]',0xf80d:'World.Queue[3]',0xf828:'Root.ControlMode',0xf829:'Root.FMDelta',0xf82a:'Root.PSGDelta',0xf82b:'Root.DuckPhase',0xf82c:'Root.ChoiceBits'}
    groups = [{'bulk_index':index,'address':full_start+4*index,'fields':[{ 'byte_offset':offset,'field':known_fields.get(0xf800+4*index+offset,'unassigned header byte')} for offset in range(4)]} for index in [0,1,2,3,10,11]]
    voice_domains = [{'name':name,'first_slot':first,'rows':count,'start':0xfff840+first*0x30,'end_exclusive':0xfff840+(first+count)*0x30,'first_bulk_long':16+first*12,'last_bulk_long':16+(first+count)*12-1} for name,first,count in [('music',0,10),('primary',10,6),('secondary',16,2)]]
    ordinary = [{'id':code,'descriptor':0x380a8+(code-6)*8,'flags':rom[0x380a8+(code-6)*8+5]} for code in range(6,128)]
    duck = [row['id'] for row in ordinary if row['flags']&32]
    assert duck == [row['id'] for row in previous['pcm']['ordinary'] if row['requests_duck']]
    fight = set(coverage['platform_fight_candidate_ids'])
    music_dac = [{'id':code,'descriptor':0x37724+(code-129)*8,'flags':rom[0x37724+(code-129)*8+5]} for code in range(129,154)]
    f0_flags = rom[0x380a8+122*8+5]
    tl = [{'bank':bank,'register':0x40+channel+4*operator,'value':0x7f} for channel in [2,1,0] for operator in range(4) for bank in [0,1]]
    input_paths = [rompath,coverage_path,original_path,provenance_path,helper,DOC/'AUDIO_GAME_DISPATCH_ORIGINAL_EVIDENCE.json',DOC/'AUDIO_GAME_DISPATCH_RESOURCES_INDEX.json',ROOT/'games/yuyuhakusho/data/audio-score.json.gz',DOC/'AUDIO_ROOT_CONTROL_SOURCE_AUDIT_FAILURES.md',DOC/'AUDIO_ROOT_CONTROL_SOURCE_AUDIT_CHECK.py',DOC/'AUDIO_ROOT_CONTROL_SOURCE_AUDIT_RED.json',Path(__file__).resolve()] + paths
    result = {'schema':1,'pass':True,'no_target_execution':True,'no_native_core_loaded':True,'no_new_Ghidra_run':True,'no_production_modified':True,
              'scope':{'fight_candidate_count':139,'fight_candidate_ids':coverage['platform_fight_candidate_ids'],'fight_music_ids':coverage['fight_music_ids'],'primary_ids':previous['primary']['ids'],'all_music_header_ids':list(range(0x81,0x97))},
              'rom_sha256':ROM_SHA,'grammar':grammar_result,
              'F803':{'E2_dispatch_entry':{'address':0x38dc0,'target':relative(rom,0x38dc0),'bytes':rom[0x38dc0:0x38dc4].hex()},'actual_handler':{'instruction':0x38e74,'bytes':rom[0x38e74:0x38e7a].hex(),'semantic':'E2 copies one score-data byte to the signal field'},'producer_in_fight_grammar':False,'other_entry_paths_not_excluded':True},
              'F805':{'known_direct_instruction_references':[row for row in refs if row['field']=='F805'],'raw_aligned_word_candidates':word_candidates['F805'],'fade_period_owner_established':False,'later_interval_is_immediate_constant':True,'indirect_or_external_usage_not_excluded':True,'bulk_reset_zeroes_byte':True},
              'producer_search':{'saved_listing_files':len(paths),'saved_instruction_addresses':len(listing),'known_direct_references':refs,'FF_subcommands':ff,'FF_handler_bytes':extension_bytes,'FF_handler_instructions':extension,'aligned_word_candidates':word_candidates,
                 'aligned_search_range':[0,len(rom)],'instruction_evidence_policy':'Accept saved direct-Ghidra instructions, or exact bounded ROM templates reached from an already corroborated sub-dispatch table. Other word matches remain unclassified.',
                 'FF_profile_natural_reachability':'not established in the audited grammar or saved natural observations',
                 'classification':{'F828':'Header writes 2/0 plus FF03 writes1 and FF04 writes80; FF profile not observed in audited score grammar.', 'F829_F82A':'Automatic-duck defaults A/2 and restore clears; FF03 may load score-data deltas only when prior deltas are both zero.', 'F82B':'Accepted PCM producers at37e56/37fc6/38076 request1; header acknowledges2 and restores0. Relevance to139fight is separately checked against descriptor flag5.', 'F82C':'Dual-slot accepted A sets bit0, accepted B clears bit0; bulk reset zeroes it.'},
                 'unresolved':['No whole-ROM computed/dynamic call-graph proof.','Raw word candidates outside accepted instruction evidence are not called code or ruled unreachable.','Saved static listings do not themselves prove match/menu execution; FF producers remain unclassified for those scene labels.']},
              'duck_descriptors':{'ordinary_count':len(ordinary),'F0_count':1,'music_DAC_count':len(music_dac),'ordinary_flag5_descriptors':[{'id':row['id'],'address':row['descriptor'],'bytes':rom[row['descriptor']:row['descriptor']+8].hex(),'flags':row['flags']} for row in ordinary if row['flags']&32],'F0_descriptor':{'address':0x380a8+122*8,'bytes':rom[0x380a8+122*8:0x380a8+123*8].hex()},'ordinary_with_flag5':duck,'fight_with_flag5':sorted(fight&set(duck)),'ordinary_flags':ordinary,'music_DAC_with_flag5':[row['id'] for row in music_dac if row['flags']&32],'music_DAC_flags':music_dac,'F0_flag5':bool(f0_flags&32),'F0_flags':f0_flags,'fight_has_7F':127 in fight,'acceptance_conditions':'New descriptor flag5 set, prior PCM mode flag5 clear, and current duck phase !=2; descriptor presence does not prove arbitration acceptance or actual duck.'},
              'fade':{'control_targets':controls,'initial':{'01':{'steps':40,'remaining':3,'clear_DAC_flag_byte':True},'05':{'steps':50,'remaining':32,'clear_DAC_flag_byte':True}},'later_remaining_constant':3,'dispatch_selected_reset':{'read':0x37cb6,'clear_before_dispatch':0x37cba,'value':255,'ROM':fixed(rom,0x37cb4,'7e001e38f80911fc00fff809')},'source_templates':templates[:3], 'remaining_boundary':'Nonzero remaining is decremented then returns; remaining1->0 does not fade in that same invocation.','terminal_boundary':'If remaining was zero, steps decrements; steps1->0 branches directly to full04 reset before a level step.','no_PCM_stop_in_01_05_bodies':True},
              'bulk_clear':{'full':{'entry':0x38a2c,'clear_instruction':0x38a3c,'start':full_start,'count_long':full_count,'end_exclusive':full_start+full_count*4,'store_addresses':[full_start+4*index for index in range(full_count)],'selected_after_clear':255,'clears_scheduler':False,'clears_Z80_PCM':False,'bulk_clears_TL_cache':False},
                            'music':{'entry':0x38a50,'clear_instruction':0x38a66,'start':music_start,'count_long':music_count,'end_exclusive':music_start+music_count*4,'store_addresses':[music_start+4*index for index in range(music_count)],'selected_after_clear':255,'preserves_priority':True,'preserves_primary_secondary':True,'priority_save_read':0x38a58,'priority_restore_write':0x38a6e},
                            'header_range':[0xfff800,0xfff840],'voice_row_bytes':48,'voice_domains':voice_domains,'cross_owner_long_groups':groups,'outside_clear':{'Scheduler':[0xfff560,0xfff565],'Z80_PCM':[0xa00000,0xa02000],'TL_cache':[0xfffbe0,0xfffc00]},'templates':templates[3:5]},
              'mute_order':{'full_order':['YM27=0','232-long-clear','selected=FF','six-keyoff','24-TL-mute','four-PSG-mute'],'keyoff_values':[2,6,1,5,0,4],'keyoff_bank':0,'keyoff_register':0x28,'TL_register_writes':tl,'PSG_values':[0x9f,0xbf,0xdf,0xff], 'music_reset_order':['YM27=0','priority-save','136-long-clear','priority-restore','selected=FF','return'],'music_reset_has_keyoff_or_PSG_mute':False,'music_start_outer_calls':[0x38808,0x388ac,0x38a50],'templates':templates[5:],'evidence':'Static expansion of fixed loop bounds only, not observed IO or master-cycle proof'},
              'bounded_control_idle_closure':{'established_for_current_fight_profile':True,'premises':['Root begins at the existing full clear/initialization, with control mode, deltas and duck phase zero.','Platform preparation skips original menus and the only request candidates are the139 audited IDs plus the closed score queue edge.','The11 fight music definitions and12 primary definitions use neitherE2 norFF under the validated bounded grammar.','No external/unsupported state assignment or computed entry outside that explicit domain is introduced.'],'steps':['01/05 write only fade remaining/steps and clear the DAC flag byte;04 and music-reset zero the control fields.','Each of the three PCM duck-request stores requires new descriptor flag5; none of the139 candidates has that flag.','Mode1/80 requests originate inFF03/FF04, which have no command in the audited data grammar.','With mode0 and duck phase0,38c2e returns through its idle path and does not create either request.'],'conclusion':'The control/duck callee38c2e remains idle under these premises. This does not make tempo/fade/pause or the whole header idle.','choice_bits_can_change':True,'whole_ROM_or_original_menu_unreachability_claim':False},'corrections':[{'earlier_assumption':'Legal fight PCM flags may trigger automatic duck within the139-candidate profile.','correction':'The valid7F descriptor can request duck, but7F is absent from the current139-candidate fight closure. Private7F vectors test a broader callable profile, not an established fight producer.','production_state_graph_supplied':False}],'source_bindings':[bind(path) for path in dict.fromkeys(input_paths)],'limitations':['No production state graph or implementation is supplied.','No new native capture, CPU execution, tools installation, PCM or playback acceptance.','Absence within the explicit grammar/listing/operand search domains is not global unreachability.','F805 has no established fade-period role; indirect/external uses are not excluded.','Automatic duck is not established by the current139fight candidate closure because the only ordinaryflag5descriptor is7F outside that closure.']}
    for binding in result['source_bindings']:
        assert sha(ROOT/binding['path']) == binding['sha256']
    out.write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps({'pass':True,'fight_candidates':139,'grammar_commands':grammar_result['commands'],'E2_events':len(grammar_result['all_E2_events']),'FF_events':len(grammar_result['all_FF_events']),'ordinary_duck_ids':duck,'fight_duck_ids':result['duck_descriptors']['fight_with_flag5'],'manifest':str(out.relative_to(ROOT))},indent=2))

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out',type=Path,required=True)
    args = parser.parse_args()
    root_audit(args.out.resolve())
