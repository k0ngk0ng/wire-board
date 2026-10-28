"""Explicit fresh campaign acceptance with isolated flags and checked results."""
import argparse
import json
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[2]
MODES = ('standard-terminal', 'assisted-clear')


def campaign_env(mode, out, base):
    if mode not in MODES:
        raise ValueError('acceptance requires standard-terminal or assisted-clear')
    env = {k: v for k, v in base.items() if not k.startswith('CONTRA_BOT_')}
    env.update(CONTRA_BOT_MODE=mode, CONTRA_BOT_TARGET='8',
               CONTRA_BOT_INFINITE_LIVES='1' if mode == 'assisted-clear' else '0',
               CONTRA_BOT_EVIDENCE=str(out))
    return env


def validate_reports(out, mode, seats, log):
    if mode not in MODES:
        raise ValueError('diagnostics are not acceptance')
    events = [json.loads(line) for line in log.read_text().splitlines() if line.startswith('{')]
    names = ['TestBotStandardCampaignE2E'] + [f'TestBotStandardCampaignE2E/players{s}' for s in seats]
    for name in names:
        actions = {e.get('Action') for e in events if e.get('Test') == name}
        if not {'run', 'pass'} <= actions or actions & {'skip', 'fail'}:
            raise ValueError(f'{name}: missing successful execution')
    results = {}
    for seat in seats:
        try:
            report = json.loads((out / f'players{seat}' / 'result.json').read_text())
        except (OSError, ValueError) as exc:
            raise ValueError(f'missing/invalid result for seat count {seat}') from exc
        expected = {'mode': mode, 'acceptance': True, 'passed': True,
                    'players': seat, 'target_level': 8, 'prefix_frames': 0,
                    'infinite_lives': mode == 'assisted-clear', 'finalized': True,
                    'source_changed_during_run': False}
        if any(report.get(k) != v for k, v in expected.items()):
            raise ValueError(f'players{seat}: invalid acceptance provenance')
        outcomes = {'cleared'} if mode == 'assisted-clear' else {'cleared', 'failed'}
        if report.get('outcome') not in outcomes or report.get('restore_checkpoints', 0) < 1:
            raise ValueError(f'players{seat}: incomplete terminal/restore verification')
        if mode == 'assisted-clear' and report.get('assisted_campaign_cleared') is not True:
            raise ValueError(f'players{seat}: assisted clear unverified')
        results[f'players{seat}'] = report
    return results


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--mode', required=True, choices=MODES)
    parser.add_argument('--out', required=True, type=Path)
    parser.add_argument('--players', type=int, choices=(1, 2))
    args = parser.parse_args()
    out = args.out.resolve()
    if not out.is_relative_to(ROOT):
        parser.error('evidence must stay in the workspace')
    out.mkdir(parents=True, exist_ok=False)
    env = campaign_env(args.mode, out, os.environ)
    if args.players:
        env['CONTRA_BOT_SEATS'] = str(args.players)
    log = out / 'go-test.jsonl'
    with log.open('w') as stream:
        code = subprocess.run(['go', 'test', '-json', './games/contra/engine', '-run',
                               '^TestBotStandardCampaignE2E$', '-count=1', '-timeout=90m'],
                              cwd=ROOT, env=env, stdout=stream, stderr=subprocess.STDOUT).returncode
    verification = {'mode': args.mode, 'test_exit': code, 'passed': False}
    try:
        results = validate_reports(out, args.mode, [args.players] if args.players else [1, 2], log)
        verification['results'] = results
        verification['passed'] = code == 0
    except ValueError as exc:
        verification['error'] = str(exc)
    (out / 'verification.json').write_text(json.dumps(verification, indent=2) + '\n')
    print(json.dumps({'out': str(out), 'passed': verification['passed']}))
    raise SystemExit(0 if verification['passed'] else 1)


if __name__ == '__main__':
    main()
