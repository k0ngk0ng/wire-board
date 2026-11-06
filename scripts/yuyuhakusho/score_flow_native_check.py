"""Frozen acceptance for existing native parser-read/flow evidence reuse."""
from pathlib import Path
import argparse
import hashlib
import json


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--report', type=Path, required=True)
    args = parser.parse_args()
    report = json.loads(args.report.read_text())
    assert report['pass'] and report['new_cpu_capture'] is False
    assert report['definitions'] == 23 and report['voice_tracks'] == 115
    assert report['native_driver_ticks'] == 122810
    assert report['native_token_reads'] == 199477
    assert report['native_control_before'] == 26130 and report['native_control_after'] == 26105
    assert report['native_note_commits'] == 127691
    assert report['shared_words_and_stack_comparisons'] == 26130
    assert report['partial_native_suffixes'] == []
    assert all(report['flow_counts'][name] > 0 for name in ('jump', 'loop', 'call', 'return'))
    assert report['catalog_json_sha256'] == '735d012d3d3140e76f8f756f0ca757bdc4b8deeb5a66750dd4610e41e06af830'
    print(json.dumps({'pass': True, 'native_token_reads': 199477, 'native_control_before': 26130,
                      'native_control_after': 26105, 'native_note_commits': 127691,
                      'report_sha256': hashlib.sha256(args.report.read_bytes()).hexdigest(),
                      'scope': 'Existing native read/control/commit prefix equality; no new native execution or timing claim.'}, indent=2))
