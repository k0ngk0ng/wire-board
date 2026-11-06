#!/usr/bin/env python3
"""Read back exact CI assets locally, then optionally open the release gate."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys

import cdn_delivery as cdn


def validate_build(run, jobs, source_sha, source_run):
    expected = {'id': source_run, 'head_sha': source_sha, 'head_branch': 'main',
                'event': 'push', 'path': '.github/workflows/build.yml',
                'status': 'completed', 'conclusion': 'success'}
    if any(run.get(k) != v for k, v in expected.items()):
        raise ValueError('Expected the successful main build for this exact commit')
    binary = [j for j in jobs if j.get('name') == 'Test and Linux amd64 binary']
    if len(binary) != 1 or binary[0].get('conclusion') != 'success':
        raise ValueError('The source binary job did not succeed')


def local_readback_proof(dist, report, source_sha, source_run):
    if not re.fullmatch(r'[a-f0-9]{40}', source_sha) or type(source_run) is not int or source_run <= 0:
        raise ValueError('Invalid exact source identity')
    if not report.get('passed') or report.get('stage') != 'verified' or report.get('cdnVerified') is not True or report.get('verificationLocation') != 'local':
        raise ValueError('A successful local CDN readback is required')
    base = 'https://cdn.ichenj.com/bga/web/' + source_sha + '/'
    manifest = json.loads((dist / 'game-load-manifest.json').read_bytes())
    if manifest.get('assetBase') != base or cdn.inventory(dist, base) != report.get('files'):
        raise ValueError('CDN report differs from the exact build assets')
    for name, key in [('index.html', 'indexSHA256'), ('game-load-manifest.json', 'manifestSHA256')]:
        if cdn.sha((dist / name).read_bytes()) != report.get(key):
            raise ValueError('Entry metadata changed after local verification')
    receipt = dist / 'contra-delivery.json'
    if receipt.exists() or report.get('contraDeliverySHA256'):
        if not receipt.is_file() or cdn.sha(receipt.read_bytes()) != report.get('contraDeliverySHA256'):
            raise ValueError('Runtime receipt differs from local verification')
    return dict(report, sourceSHA=source_sha, sourceRun=source_run)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--run', type=int, required=True)
    parser.add_argument('--sha', required=True)
    parser.add_argument('--repo')
    parser.add_argument('--out', type=Path)
    parser.add_argument('--mark-ready', action='store_true', help='Set the exact commit release status after successful readback')
    args = parser.parse_args()
    if os.environ.get('GITHUB_ACTIONS') == 'true':
        raise ValueError('Run CDN readback locally, outside GitHub Actions')
    if not re.fullmatch(r'[a-f0-9]{40}', args.sha) or args.run <= 0:
        raise ValueError('Invalid source identity')

    def gh(*arguments):
        return json.loads(subprocess.check_output(['gh', *arguments], text=True))

    repo = args.repo or gh('repo', 'view', '--json', 'nameWithOwner')['nameWithOwner']
    if not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repo):
        raise ValueError('Invalid repository')
    prefix = 'repos/' + repo + '/actions/runs/' + str(args.run)
    run = gh('api', prefix)
    jobs = gh('api', prefix + '/jobs?per_page=100')['jobs']
    validate_build(run, jobs, args.sha, args.run)
    artifacts = gh('api', prefix + '/artifacts?per_page=100')['artifacts']
    ids = {}
    for name in ['web-dist', 'wire-board-linux-amd64']:
        rows = [a for a in artifacts if a['name'] == name and not a['expired'] and a['size_in_bytes'] > 0]
        if len(rows) != 1:
            raise ValueError('Missing unexpired exact-build artifact: ' + name)
        ids[name] = rows[0]['id']
    out = args.out or Path('.local/cdn-local') / str(args.run)
    dist = out / 'web-dist'
    dist.mkdir(parents=True, exist_ok=False)

    def status(state, description):
        gh('api', '--method', 'POST', 'repos/' + repo + '/statuses/' + args.sha,
           '-f', 'context=cdn-local-readback', '-f', 'state=' + state,
           '-f', 'description=' + description,
           '-f', 'target_url=https://github.com/' + repo + '/actions/runs/' + str(args.run))

    if args.mark_ready:
        status('pending', 'Local CDN readback for run ' + str(args.run))
    try:
        subprocess.run(['gh', 'run', 'download', str(args.run), '--repo', repo, '--name', 'web-dist', '--dir', str(dist)], check=True)
        report_path = out / 'verified.json'
        subprocess.run([sys.executable, '-B', str(Path(__file__).with_name('cdn_delivery.py')), 'verify', '--dist', str(dist), '--report', str(report_path)], check=True)
        proof = local_readback_proof(dist, json.loads(report_path.read_text()), args.sha, args.run)
        proof.update(repository=repo, sourceArtifactIDs=ids)
        report_path.write_text(json.dumps(proof, indent=2) + '\n')
        digest = hashlib.sha256(report_path.read_bytes()).hexdigest()
        if args.mark_ready:
            status('success', f'run {args.run}; sha256={digest}')
        print(json.dumps({'passed': True, 'sourceRun': args.run, 'sourceSHA': args.sha, 'files': len(proof['files']), 'report': str(report_path), 'reportSHA256': digest, 'releaseReady': args.mark_ready}))
    except Exception:
        if args.mark_ready:
            status('failure', 'Local CDN readback failed for run ' + str(args.run))
        raise


if __name__ == '__main__':
    main()
