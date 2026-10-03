#!/usr/bin/env python3
"""Acquire and verify the finite classic-DotA art shortlist. Standard library only.

Run from this directory. Files are deliberately kept beside the reviewable
manifest, not installed into the website or uploaded to object storage.
"""
import concurrent.futures
import hashlib
from html.parser import HTMLParser
import json
from pathlib import Path
import re
import struct
import time
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parent
BASE = "https://iccup.com"
HEROES = {
    "axe": ("Axe", "斧王"),
    "crystal_maiden": ("Crystal Maiden", "冰女"),
    "earthshaker": ("Earthshaker", "撼地神牛"),
    "sven": ("Rogue Knight", "流浪剑客"),
    "juggernaut": ("Juggernaut", "剑圣"),
    "drow": ("Drow Ranger", "黑暗游侠"),
    "lina": ("Slayer", "秀逗魔导士"),
    "sniper": ("Dwarven Sniper", "矮人狙击手"),
}
ITEMS = {
    "blink": ("Kelens Dagger of Escape", "跳刀"),
    "boots": ("Boots of Speed", "速度之靴"),
    "bracer": ("Bracer", "护腕"),
    "wraith_band": ("Wraith Band", "幽灵系带"),
    "null_talisman": ("Null Talisman", "空灵挂件"),
    "wand": ("Magic Wand", "魔杖"),
    "blade_mail": ("Blade Mail", "刃甲"),
    "bkb": ("Black King Bar", "黑皇杖"),
    "mekansm": ("Mekansm", "梅肯斯姆"),
    "vanguard": ("Vanguard", "先锋盾"),
    "crystalys": ("Crystalys", "水晶剑"),
    "aghanim": ("Aghanims Scepter", "阿哈利姆神杖"),
    "force_staff": ("Force Staff", "原力法杖"),
    "phase_boots": ("Phase Boots", "相位鞋"),
    "arcane_boots": ("Arcane Boots", "秘法鞋"),
    "quelling_blade": ("Quelling Blade", "补刀斧"),
    "blades_attack": ("Blades of Attack", "攻击之爪"),
}


def get(url):
    for attempt in range(3):
        try:
            req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
            with urllib.request.urlopen(req, timeout=20) as response:
                assert response.status == 200
                return response.headers.get("Content-Type", ""), response.read(2_000_000)
        except Exception:
            if attempt == 2:
                raise
            time.sleep(attempt + 1)


class ItemLinks(HTMLParser):
    def __init__(self):
        super().__init__()
        self.link = None
        self.images = {}

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        if tag == "a":
            self.link = attrs.get("href")
        if tag == "img" and self.link and self.link.startswith("items/"):
            self.images[self.link.removeprefix("items/")] = attrs.get("src", "")

    def handle_endtag(self, tag):
        if tag == "a":
            self.link = None


def hero_sources(pair):
    key, (slug, name) = pair
    page = BASE + "/dota/heroes/" + urllib.parse.quote(slug)
    _, html = get(page)
    links = re.findall(r'<img[^>]+src=[\"\x27]([^\"\x27]+)', html.decode(), re.I)
    links = list(dict.fromkeys(urllib.parse.urljoin(BASE, x) for x in links if "/upload/images/heroes/" in x))
    assert len(links) == 5, (key, links)
    return [{"id": key + ("_portrait" if i == 0 else f"_skill_{i}"),
             "group": "hero", "hero": key, "name": name, "source_page": page,
             "source": url} for i, url in enumerate(links)]


def download(entry):
    content_type, content = get(entry["source"])
    assert content_type.startswith("image/"), entry
    if content[:8] == b"\x89PNG\r\n\x1a\n":
        width, height = struct.unpack(">II", content[16:24])
        assert content[-8:-4] == b"IEND", entry
        suffix = ".png"
    elif content[:2] == b"\xff\xd8":
        suffix, pos = ".jpg", 2
        width = height = 0
        while pos < len(content):
            assert content[pos] == 255, (entry, pos)
            while content[pos] == 255:
                pos += 1
            marker = content[pos]
            pos += 1
            if marker in (0xD8, 0xD9):
                continue
            length = int.from_bytes(content[pos:pos+2], "big")
            if marker in (0xC0, 0xC1, 0xC2):
                height, width = struct.unpack(">HH", content[pos+3:pos+7])
                break
            pos += length
        assert width and height, entry
    elif content[:6] in (b"GIF89a", b"GIF87a"):
        suffix = ".gif"
        width, height = struct.unpack("<HH", content[6:10])
    else:
        raise AssertionError((entry, "Unknown image format"))
    assert width >= 48 and height >= 48, entry
    relative = "assets/" + entry["id"] + suffix
    (ROOT / relative).write_bytes(content)
    return dict(entry, file=relative, width=width, height=height,
                bytes=len(content), sha256=hashlib.sha256(content).hexdigest())


def main():
    (ROOT / "assets").mkdir(parents=True, exist_ok=True)
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        entries = [x for group in pool.map(hero_sources, HEROES.items()) for x in group]
    _, html = get(BASE + "/dota/items.html")
    parser = ItemLinks()
    parser.feed(html.decode())
    for key, (slug, name) in ITEMS.items():
        entries.append({"id": key, "group": "item", "name": name,
                        "source_page": BASE + "/dota/items.html",
                        "source": urllib.parse.urljoin(BASE, parser.images[slug])})
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        verified = list(pool.map(download, entries))
    manifest = {"verified_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                "scope": "8 classic DotA heroes (portrait + four skills) and 17 items; no Dota 2 art",
                "assets": verified}
    (ROOT / "assets.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps({"verified": len(verified), "bytes": sum(x["bytes"] for x in verified),
                      "smallest": min(x["width"] for x in verified)}, ensure_ascii=False))


if __name__ == "__main__":
    main()
