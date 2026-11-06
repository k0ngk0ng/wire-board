"""Acceptance checks for the independent static Root-control source audit."""
import argparse
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--audit', type=Path, required=True)
    args = parser.parse_args()
    path = args.audit.resolve()
    assert path.is_relative_to(ROOT)
    data = json.loads(path.read_text())
    assert data['pass'] and data['no_target_execution'] and data['no_native_core_loaded']
    assert data['scope']['fight_candidate_count'] == 139
    assert len(data['scope']['fight_music_ids']) == 11 and len(data['scope']['primary_ids']) == 12
    assert data['grammar']['music_definitions'] == 22 and data['grammar']['primary_definitions'] == 12
    assert data['grammar']['walk_errors'] == []
    assert data['grammar']['all_E2_events'] == data['grammar']['all_FF_events'] == []
    assert data['grammar']['fight_E2_events'] == data['grammar']['fight_FF_events'] == []
    assert data['grammar']['matches_previous_static_provenance']
    assert data['fade']['control_targets'] == {'01': 0x38916, '02': 0x38808, '03': 0x388ac, '04': 0x38a2c, '05': 0x38928}
    assert data['fade']['initial']['01'] == {'steps': 40, 'remaining': 3, 'clear_DAC_flag_byte': True}
    assert data['fade']['initial']['05'] == {'steps': 50, 'remaining': 32, 'clear_DAC_flag_byte': True}
    assert data['fade']['later_remaining_constant'] == 3
    assert data['F805']['known_direct_instruction_references'] == [] and not data['F805']['fade_period_owner_established']
    assert data['producer_search']['FF_subcommands']['03']['target'] == 0x393c4
    assert data['producer_search']['FF_subcommands']['04']['target'] == 0x393ec
    assert data['producer_search']['FF_profile_natural_reachability'] == 'not established in the audited grammar or saved natural observations'
    assert data['duck_descriptors']['ordinary_with_flag5'] == [127]
    assert data['duck_descriptors']['fight_with_flag5'] == []
    assert not data['duck_descriptors']['F0_flag5'] and data['duck_descriptors']['music_DAC_with_flag5'] == []
    clear = data['bulk_clear']
    assert (clear['full']['start'], clear['full']['count_long'], clear['full']['end_exclusive']) == (0xfff800, 232, 0xfffba0)
    assert (clear['music']['start'], clear['music']['count_long'], clear['music']['end_exclusive']) == (0xfff800, 136, 0xfffa20)
    assert clear['music']['preserves_priority'] and clear['music']['preserves_primary_secondary']
    assert clear['full']['selected_after_clear'] == clear['music']['selected_after_clear'] == 255
    assert not clear['full']['clears_scheduler'] and not clear['full']['clears_Z80_PCM'] and not clear['full']['bulk_clears_TL_cache']
    assert data['mute_order']['keyoff_values'] == [2, 6, 1, 5, 0, 4]
    assert len(data['mute_order']['TL_register_writes']) == 24
    assert data['mute_order']['PSG_values'] == [0x9f, 0xbf, 0xdf, 0xff]
    assert data['mute_order']['full_order'] == ['YM27=0', '232-long-clear', 'selected=FF', 'six-keyoff', '24-TL-mute', 'four-PSG-mute']
    assert data['mute_order']['music_reset_has_keyoff_or_PSG_mute'] is False
    for binding in data['source_bindings']:
        source = ROOT / binding['path']
        assert hashlib.sha256(source.read_bytes()).hexdigest() == binding['sha256'], binding['path']
    print(json.dumps({'pass': True, 'audit': str(path.relative_to(ROOT)), 'sources': len(data['source_bindings']), 'scope': 'Static source facts only; no runtime/state-machine acceptance'}, indent=2))

if __name__ == '__main__':
    main()
