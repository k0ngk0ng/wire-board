# 迈阿密热线

2 至 4 人自由混战，默认 3 分钟。地图、造型和起始武器在平台房间中选择，参考图来自恢复的原版素材。击杀加 1 分，自杀减 1 分；最高分并列使用平台显式和局。真人断线不自动托管，电脑和主动托管经过同一输入接口。

## 内容与原版对照

| 内容 | 实现 |
| --- | --- |
| 地图 | No Talk、Full House、Push It、Overdose、Tension、Neighbors 的独立楼层，来自原版房间 17、31、33、20、49、35 |
| 造型 | Jacket、Biker、Bodyguard、The Father、Richter、Producer、Beard、Hobo，属性及碰撞体一致 |
| 武器 | 手枪、Uzi、M16、霰弹枪、小刀、棒球棒、武士刀、马格南、MP5、双管霰弹枪、消防斧、砍刀 |
| 战斗 | 有限弹药、投掷、拾取、击倒、处决、门、玻璃、连续弹丸碰撞与互杀 |
| 复活 | 2 秒复活，800 ms 出生保护，攻击或投掷解除保护 |
| 操作 | 键鼠、手机双摇杆、公共改键与实体手柄映射 |

`data/maps.json` 保留地图规则及来源对象。客户端地图图片、图集、参考图、音效和 WASM 是外部资产；Git 只保存源码、规则数据和资源元数据。原版关卡的剧情、固定敌人和上下楼触发器不参与混战，出生点、武器点、属性统一和武器平衡属于明确的多人改编。

原版样本和 REA 证据位于 `.local/hotlinemiami/research/`，来源与初步调查见[方案](../../docs/hotlinemiami-multiplayer-plan.md)。资源解析、动作组合和原版语义的证据边界以 `scripts/hotlinemiami/`、`docs/research/hotlinemiami/` 及资源清单的来源字段为准。Boss 的通用武器、组合行走动作不宣称为原版已有的完整玩家角色。

## 同步

服务器以定点整数规则推进 60 Hz，通常每两个规则步发布紧凑动态状态。本人的移动、瞄准和攻击反馈立即在本地响应；移动碰撞执行同一 Go 引擎编译的 WASM。服务器独立决定弹药、碰撞、伤害、生命和得分，不接受客户端计算的结果。

输入带对局代次、生命代次、序号和最近已见的权威帧。连续输入可以合并，攻击与离散动作边沿保留；收到与规则应用分别确认。本地预测最多 9 帧，确认后只重放未应用输入，不随普通快照重建显示时钟。断线、失焦、隐藏和取消操作释放输入，不补播后台攻击。

枪械弹丸最多补偿 6 帧权威历史，检查历史门墙和目标生命代次。超过窗口的旧参考帧不再补偿，未来帧被拒绝；近战、投掷和处决不做历史回溯。历史、内部输入队列与电脑决策数据不进入公开高频投影。每条连接有独立事件游标，缺口通过鉴权快照恢复。

## 开发与复跑

先恢复已登记的外部资源，再使用正式构建入口。资源恢复使用仓库现有业务配置，不在源码中填写站点地址或凭据。

```sh
python3 -B scripts/resource_store.py hydrate --game hotlinemiami
npm --prefix web run build
```

从已下载原版样本重新提取的入口和失败场景见 `scripts/hotlinemiami/test_extract_assets.py`、`docs/research/hotlinemiami/asset-failure-scenarios.md`。地图转换与只读一致性门禁：

```sh
go run ./games/hotlinemiami/cmd/maps --check
node games/hotlinemiami/build/runtime-delivery.mjs
```

所有测试和工具均须使用游戏自己的缓存、临时目录和报告目录。Go 检查示例：

```sh
mkdir -p .local/hotlinemiami/go-cache .local/hotlinemiami/go-mod .local/hotlinemiami/tmp .local/hotlinemiami/verification
GOCACHE="$PWD/.local/hotlinemiami/go-cache" GOMODCACHE="$PWD/.local/hotlinemiami/go-mod" GOTMPDIR="$PWD/.local/hotlinemiami/tmp" TMPDIR="$PWD/.local/hotlinemiami/tmp" GOTOOLCHAIN=local GOTELEMETRY=off go test -json -count=1 -timeout=12m ./games/hotlinemiami/engine ./games/hotlinemiami/tests > .local/hotlinemiami/verification/go-final.jsonl
node --test games/hotlinemiami/tests/*.test.mjs
node games/hotlinemiami/build/lint.mjs
python3 -B games/hotlinemiami/tests/accept_browser.py
```

浏览器入口启动正式 HTTP／WebSocket／SQLite 房间并使用 `web/dist`，默认执行自然 3 分钟结算。`HOTLINE_BROWSER_SKIP_FINISH=1` 只适合联调，不能作为完整交付的验收依据。测试账号、独立浏览器和 TCP 延迟代理均限本地；触屏测量使用桌面浏览器的原生触控模拟，不代表某款实体手机或手柄已实测。

## 验收预算与产物

| 检查 | 开发前固定预算 |
| --- | --- |
| 正常网络 | RTT 不高于 80 ms，抖动不高于 20 ms |
| 原生键鼠到画面 | P95 不高于 33 ms |
| 原生触屏到画面 | P95 不高于 50 ms |
| 可见更新 | 目标 60 FPS，帧间隔 P95 不高于 20 ms、P99 不高于 33 ms |
| 模块回调 | 8 个四人模块实例的回调 P95 不高于 8 ms |
| 同服务负载 | 8 个真实四人房间和一个不读取的 WebSocket，权威时钟落后 P95 不高于 67 ms |

验证边界和复跑入口分别记录在 `tests/server-scenarios.md`、`tests/engine-scenarios.md`、`tests/web-scenarios.md`、`tests/browser-scenarios.md` 和 `tests/lint-scenarios.md`。结果统一位于 `.local/hotlinemiami/verification/` 与 `.local/hotlinemiami/browser/reports/`，包含测试 JSON、帧与输入记录、网络配置、CSV、截图和源码摘要。

Go 完整比赛验证覆盖 6 张地图的 2／3／4 人自然对局，正式 HTTP 验证覆盖席位权限、真实 WebSocket、重复请求、托管接管和 SQLite 恢复。浏览器验证分别记录实际 WebSocket RTT、输入 ACK、权威应用、显示帧和原生事件到像素响应，不以 HTTP 耗时或稳定帧率替代操作延迟。
