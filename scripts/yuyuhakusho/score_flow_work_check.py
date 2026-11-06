"""Regression contract: unknown or source-bearing Work never passes selection."""
import argparse
from score_flow_entry import select


def call(work):
    return select({'variants': [{'entries': ['continue'], 'work': work}]}, 'continue')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--red', action='store_true')
    args = parser.parse_args()
    invalid = [[{'kind': 'unpriced-unknown-work'}], [{'kind': 'jump', 'target': 123}],
               [{'kind': 'duration-scale', 'additions': 256}],
               [{'kind': 'parameter-read', 'role': 'unrestored-role'}]]
    for work in invalid:
        try:
            call(work)
        except ValueError:
            continue
        raise AssertionError(('unsupported Work silently selected', work))
    print('pass: unknown Work, source target, oversized repeat and unknown role reject')
