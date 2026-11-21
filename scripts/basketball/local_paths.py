from pathlib import Path

LEGACY_CAPTURE_NAMES = frozenset({'nes-basket', 'nes-basketball'})


def original_path(root, source):
    root = Path(root).resolve()
    path = Path(source)
    if path.is_absolute():
        path = path.relative_to(root)
    if len(path.parts) >= 2 and path.parts[0] == '.local' and path.parts[1] in LEGACY_CAPTURE_NAMES:
        path = Path('.local/basketball/research/original', *path.parts[2:])
    target = (root / path).resolve()
    if not target.is_relative_to(root):
        raise ValueError('Original evidence must stay inside the workspace')
    return target
