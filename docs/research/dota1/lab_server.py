#!/usr/bin/env python3
"""Local, in-memory feasibility demo. Never connects to production or OSS."""
from dataclasses import asdict
from functools import partial
from http.server import HTTPServer, SimpleHTTPRequestHandler
import json
from pathlib import Path
import random
import secrets
import time
import urllib.parse

from simulate import CARDS, Game, HEROES, HP, ITEMS, Order, choose

ROOT = Path(__file__).resolve().parent
ROOMS = {}
NAMES = dict(zip(HEROES, ("斧王", "冰女", "撼地神牛", "流浪剑客", "剑圣", "黑暗游侠", "秀逗魔导士", "矮人狙击手")))
SKILLS = {
    "axe": [("狂战士之吼", 1, "护盾 +4、推进 +2；替同路友军承受突袭。"), ("战斗饥渴", 2, "对目标造成 3 点魔法伤害，推进 +1。"), ("淘汰之刃", 4, "目标生命 ≤4 时造成 7 点伤害，否则 6 点。")],
    "crystal_maiden": [("冰霜新星", 1, "同路敌人各受 2 点魔法伤害，推进 −1；自己推进 +1。"), ("冰封禁制", 2, "目标受 4 点魔法伤害，推进 −1；自己推进 +1。"), ("极寒领域", 4, "同路敌人各受 4 点魔法伤害。")],
    "earthshaker": [("沟壑", 1, "目标受 3 点魔法伤害，推进 −2；自己推进 +1。"), ("强化图腾", 2, "目标受 4 点物理伤害；自己推进 +1。"), ("回音击", 4, "同路敌人各受 2＋敌人数的魔法伤害，最多 5。")],
    "sven": [("风暴之锤", 1, "目标受 3 点魔法伤害，推进 −2。"), ("战吼", 3, "同路友军各获得 3 点护盾；自己推进 +2。"), ("神之力量", 4, "目标受 6 点物理伤害。")],
    "juggernaut": [("剑刃风暴", 1, "同路敌人各受 3 点魔法伤害；本轮魔免，推进 +1。"), ("治疗守卫", 2, "同路友军各恢复 3 点生命；自己推进 +1。"), ("无敌斩", 4, "同路敌人各受 4 点魔法伤害。桌游改编效果。")],
    "drow": [("霜冻之箭", 1, "目标受 3 点物理伤害，推进 −2。"), ("沉默", 2, "同路敌人的技能和大招失效；自己推进 +2。"), ("射手天赋", 4, "目标受 6 点物理伤害。桌游改编为主动牌。")],
    "lina": [("龙破斩", 1, "同路敌人各受 3 点魔法伤害；自己推进 +1。"), ("光击阵", 2, "同路敌人各受 2 点魔法伤害，推进 −1；自己推进 +1。"), ("神灭斩", 4, "目标受 6 点魔法伤害。")],
    "sniper": [("散弹", 1, "同路敌人各受 1 点魔法伤害；自己推进 +2。"), ("爆头", 2, "同路一个目标受 4 点物理伤害。"), ("暗杀", 4, "任意路一个目标受 6 点魔法伤害。")],
}
COMMON = ["同路推进 +3；换路推进 +2；从泉水出发 +1。", "对同路目标造成 3 点物理伤害，推进 +1。",
          "本轮获得 3 点护盾，推进 +2。", "存活到轮末，额外获得 3 金币。",
          "先承受攻击；存活后回泉水、回满生命、收回行动牌，可购买一件装备。"]


def payload(room):
    out = {k: room[k] for k in ("id", "n", "phase", "teams", "seats")}
    out["names"], out["skills"], out["common"] = NAMES, SKILLS, COMMON
    out["items"] = {k: {"price": v[0]} for k, v in ITEMS.items()}
    if "game" in room:
        game, player = room["game"], room["player"]
        out.update(game=game.public(), player=player, seat_order=room["seat_order"],
                   legal=[asdict(a) for a in game.legal(player)] if not game.finished else [],
                   history=game.log[-5:], winner=game.winner, finished=game.finished,
                   reason=game.end_reason)
    return out


def command(data):
    action = data["action"]
    if action == "new":
        n = int(data["n"])
        if n not in (1, 2, 3):
            raise ValueError("人数必须为 2、4 或 6。")
        identifier = secrets.token_hex(8)
        room = {"id": identifier, "n": n, "phase": "waiting", "teams": None,
                "rng": random.Random(secrets.randbits(64)),
                "seats": [{"name": "你" if p == 0 else f"电脑 {p}", "bot": p != 0} for p in range(2*n)]}
        if len(ROOMS) > 10:
            ROOMS.pop(next(iter(ROOMS)))
        ROOMS[identifier] = room
        return payload(room)
    room = ROOMS[data["id"]]
    n, rng = room["n"], room["rng"]
    if action == "start" and room["phase"] == "waiting":
        sides = [0] * n + [1] * n
        rng.shuffle(sides)
        room.update(phase="teams", teams=sides)
    elif action == "teams" and room["phase"] == "teams":
        sides = data["teams"]
        if len(sides) != 2*n or sides.count(0) != n or sides.count(1) != n:
            raise ValueError("两队人数必须相等。")
        room.update(phase="heroes", teams=sides)
    elif action == "hero" and room["phase"] == "heroes":
        hero = data["hero"]
        if hero not in HEROES:
            raise ValueError("未知英雄。")
        ids = sorted(range(2*n), key=lambda p: room["teams"][p])
        player = ids.index(0)
        roster = rng.sample([h for h in HEROES if h != hero], 2*n-1)
        roster.insert(player, hero)
        game = Game(n, roster)
        game.first = rng.randrange(2)
        room.update(phase="playing", game=game, player=player, seat_order=ids)
    elif action in ("act", "auto") and room["phase"] == "playing":
        game = room["game"]
        if game.finished or data["round"] != game.round:
            raise ValueError("本轮已经结算，请刷新状态。")
        choices = game.legal(room["player"])
        selected = choices[int(data["choice"])] if action == "act" else None
        view, orders = game.public(), [None] * (2*n)
        for team in (0, 1):
            proposals = {}
            if selected and game.heroes[room["player"]].team == team:
                proposals[room["player"]] = selected
                orders[room["player"]] = selected
            for offset in range(n):
                p = team*n + (offset+game.round) % n
                if p not in proposals:
                    orders[p] = choose(view, p, "balanced", rng, proposals)
                    proposals[p] = orders[p]
        game.resolve(orders)
    elif action != "state":
        raise ValueError("当前阶段无法执行此操作。")
    return payload(room)


class Handler(SimpleHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/":
            self.path = "/review.html"
        super().do_GET()

    def do_POST(self):
        try:
            if self.path != "/api" or int(self.headers.get("Content-Length", 0)) > 8192:
                raise ValueError("无效请求。")
            data = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            result, status = command(data), 200
        except (KeyError, ValueError, IndexError, AssertionError) as err:
            result, status = {"error": str(err) or "操作不合法。"}, 400
        body = json.dumps(result, ensure_ascii=False).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *_):
        pass


if __name__ == "__main__":
    server = HTTPServer(("127.0.0.1", 18764), partial(Handler, directory=str(ROOT)))
    print("DOTA1 规则验证样板：http://127.0.0.1:18764（仅本机；关闭后不保留牌局）", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        server.server_close()
