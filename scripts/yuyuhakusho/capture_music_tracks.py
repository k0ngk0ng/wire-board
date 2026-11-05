"""Run the frozen native survey at structure-derived per-track bounds."""
from pathlib import Path
import argparse, json
from reference import ROOT, sha
from music_score_structure import derive_song
from survey_music_sequences import run, CODES


def main(out):
    rom=(ROOT/'.local/md-yuyuhakusho/downloads/yuyuhakusho-jp.bin').read_bytes()
    scores={code:derive_song(rom,code)for code in CODES}
    bounds={code:score['per_track_two_cycle_native_bound']for code,score in scores.items()}
    assert all(4096<steps<=20000 for steps in bounds.values()),'Per-track bound needs review; do not silently extend'
    plan={'scope':'Two rhythmic cycles of every individual track, not two all-track joint periods',
          'per_code_bounds':bounds,'source_sha256':{str(path.relative_to(ROOT)):sha(path.read_bytes())for path in [Path(__file__).resolve(),ROOT/'scripts/yuyuhakusho/music_score_structure.py',ROOT/'scripts/yuyuhakusho/survey_music_sequences.py']},
          'scores':[{'code':code,'common_rhythm_quanta':score['common_rhythm_quanta'],'common_rhythm_tempo_frames':score['common_rhythm_tempo_frames'],'whole_common_two_cycle_bound':score['proposed_native_bound'],'per_track_two_cycle_native_bound':score['per_track_two_cycle_native_bound']}for code,score in scores.items()]}
    (out/'extended-plan.json').write_text(json.dumps(plan,indent=2)+'\n')
    for code in CODES:
        directory=out/f'extended-{code:02x}'
        if (directory/'survey.json').exists():
            report=json.loads((directory/'survey.json').read_text())
            assert report['pass']and report['songs'][0]['sampling']['limit']==bounds[code]
            print(json.dumps({'reuse_complete':hex(code),'steps':bounds[code]}),flush=True)
            continue
        directory.mkdir(parents=True,exist_ok=True)
        run(directory,bounds[code],[code])
    print('all eight per-track native captures complete',flush=True)

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',required=True,type=Path)
    args=parser.parse_args();out=args.out.resolve();assert out.is_relative_to(ROOT/'.local');out.mkdir(parents=True,exist_ok=True)
    main(out)
