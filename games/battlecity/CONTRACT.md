# 坦克大战（battlecity）规则与验收契约

本文是实现前的行为契约。规则、协议和失败场景先于实现确定；实现代码必须与本文一致，测试记录保存在 `.local/battlecity/` 与 `docs/battlecity-verification.md`。

## 1. 范围

- 服务端权威的 2／4／6 人实时对战：1v1、2v2、3v3；两队人数相等。
- 上下两个大本营；敌方炮弹命中大本营即获胜；坦克无限重生。
- 开局选坦克；5 种原版坦克类型；随机道具；12 张双基地地图；房主创建时选图。
- 支持电脑玩家、托管/取消托管、观战、断线重连、恢复存档、团队结果和平台积分。
- 不做原版闯关、合作、关卡编辑器和模拟器运行时；不做 3 人/5 人等非偶数人数。

## 2. 房间、队伍与阶段

- 后端 `MinPlayers=2`、`MaxPlayers=6`，只接受 2／4／6；前端 `playerCounts:[2,4,6]`、`requiresFullTable:true`。
- 座位 `0..teamSize-1` 为上方队伍，`teamSize..2*teamSize-1` 为下方队伍；`teamSize=players/2`。
- 阶段：`select`（15 秒）→ `countdown`（3 秒）→ `playing`（最多 10 分钟）→ `finished`。
- 选坦克超时由服务端随机补齐；电脑玩家立即选择。全部选完后可提前进入倒计时。
- 地图选项 `{map:"classic-01".."classic-12"|"random"}`；`random` 在 Start 时由服务端抽取并写入状态。

## 3. 地图与坐标

- 逻辑战场 13×13 格，每格 16×16 px，总 208×208 px。
- 碰撞以 8×8 子格（26×26）处理；砖/钢每格拆成 2×2 子格。
- 上方基地 `(6,0)`，下方基地 `(6,12)`；各为 16×16 格；周围 5 格砖墙保护圈。
- 出生点：上方 `(2,2)`、`(6,2)`、`(10,2)`；下方 `(2,10)`、`(6,10)`、`(10,10)`。
  - 1v1 使用中心点 `(6,2)`／`(6,10)`。
  - 2v2 使用两侧点 `(2,2)`、`(10,2)`／`(2,10)`、`(10,10)`。
  - 3v3 使用全部三个点。
- 地形码：`0` 空地、`1` 砖、`2` 钢铁、`3` 水、`4` 树、`5` 冰。
- 12 张地图必须 180° 旋转对称；出生格清晰、基地保护圈完整、双方基地可达（砖块可破坏，水/钢不可穿越）。

## 4. 坦克

| 类型 | 速度 px/s | HP | 子弹 px/s | 初始弹数 | 说明 |
|---|---:|---:|---:|---:|---|
| balanced 均衡 | 44 | 1 | 220 | 1 | 原版玩家坦克 |
| basic 基础 | 34 | 1 | 200 | 1 | 慢速基础型 |
| fast 快速 | 62 | 1 | 200 | 1 | 高速 |
| power 火力 | 38 | 1 | 300 | 1 | 高弹速、装填更快 |
| armor 装甲 | 28 | 4 | 200 | 1 | 受击降级，慢速 |

- 坦克 16×16、四方向；转向对齐 8 px 网格；撞砖/钢/水/基地/其他坦克被阻挡；树可穿越并遮挡；冰面有短距离滑行。
- 开火为按住自动连发，受装填时间与场上自己的子弹数限制；子弹 6×6，swept AABB。
- 子弹互消；敌方子弹伤害坦克；友军子弹不伤害队友、不伤害己方基地。
- 子弹命中砖块摧毁命中的 8×8 子格；钢铁阻挡（科技 3 可破钢）；水/树/冰不阻挡；敌方子弹命中基地即摧毁基地。
- 阵亡 4 秒后重生；出生保护 2 秒；出生点被占用按顺序换点，全部占用则等待。科技等级阵亡清零，坦克类型保留。

## 5. 科技与随机道具

- 星：科技 +1（上限 3）。科技 1 提升子弹速度；科技 2 允许两发；科技 3 可击碎钢铁。
- 手雷：摧毁所有存活敌方坦克，不伤基地；手雷击杀不触发掉落。
- 头盔：拾取者 8 秒无敌。
- 铲子：己方基地保护圈变钢铁 20 秒，结束后恢复砖墙。
- 坦克：下一次阵亡立即重生并获得 3 秒护盾。
- 时钟：冻结所有敌方坦克 8 秒（不能移动/开火）。
- 普通子弹击毁敌方坦克约 30% 概率在死亡位置掉落随机道具；场上最多 4 个；25 秒未拾取消失；位置不可用找最近可用格。

## 6. 胜负与结果

- 任意基地被摧毁，对方队伍立即获胜；同一 100 ms 帧内双方基地都被摧毁为显式和局。
- 比赛 10 分钟上限；到时双方基地存活为显式和局。
- `Result.Team=true`；胜方全队 `Ranks=1` 且进入 `Winners`，负方 `Ranks=2`；和局 `Draw=true`、无人获胜、全员 rank 1。
- `Scores` 至少包含每席击杀数；`Details` 记录地图、坦克类型、击杀、基地摧毁者/时间、结束原因。
- 含电脑席位由平台不计分；和局所有真人积分变化为 0。

## 7. 服务端协议

- 输入只接受控制意图，绝不接受位置、HP、命中、得分或结果。
- 选坦克：`{"type":"select","epoch":"...","tank":"balanced|basic|fast|power|armor"}`；选定后锁定。
- 操作：`{"type":"input","epoch":"...","seq":12,"move":"none|up|down|left|right","fire":true}`。
- 严格 JSON、未知字段拒绝、最大 8 KiB；`seq` 单调且跳跃有上限；`epoch` 必须匹配。
- 每席 `Pending` 最多 64 条，按接收时间应用到 20 ms 子步；`lostInputMS=750` 无新输入回到中立。
- 100 ms 帧、5 个 20 ms 子步；`OnTimer` 从 `ServerAt+100` 追赶至当前时刻（自然受 10 分钟上限约束），保证返回的 `SystemDeadline` 仍在未来。
- 上限：子弹 32、道具 4、事件 64、日志 80、坦克 6。
- `View` 不暴露 RNG、未来掉落、待处理输入；观战 `viewer=-1` 无控制。
- `envelope`：select 只把未选角席位放入 Actors/Attention、`Deadline=0`、`SystemDeadline=SelectUntil`；countdown 无 Actors；playing Actors 为全席、Attention 显式空、Deadline 最早 IdleAt；finished 清空期限并返回 Result。
- `AcceptStale` 校验 epoch/seq/阶段/席位；`Restore` 只修复越界/不可能序号；`Reclaim` 只清空该席控制并刷新 IdleAt。

## 8. 前端

- 共享创建/等待/玩家栏/结果浮层；游戏只提供 Cover/Options/Rules/PlayerSummary/Board。
- Node 选择 2／4／6 人；`Options` 提供 12 张地图 + 随机及缩略图。
- Canvas 逻辑 208×208，桌面最大 3×，320 px 手机约 312 px（坦克约 24 px）；`image-rendering: pixelated`。
- 队伍 HUD 显示每队 `teamSize` 人的坦克类型、HP、重生、击杀；公共玩家栏的 `PlayerSummary` 同步关键资源。
- 键盘 WASD/方向键 + 空格/J；手机画布下方独立方向键 + 开火键，多指、失焦/隐藏/卸载释放。
- 客户端只发控制意图；本地坦克预测仅外观，远端插值；命中/基地/结果以服务端为准。
- 音效全部通过 `BoardProps.audio`；BGM 用 `audio.music`，事件用 `playWithFallback`；不创建播放器/AudioContext；事件游标按 `room.id:epoch` 去重。
- 结果浮层可关闭/重开；不插入结算横条；减少动态效果、对比度、320 px 均须验收。

## 9. 电脑与恢复

- BotAction 使用与真人相同的 action；只读公开视图；选坦克用确定性哈希；导航 A*（砖可破、水/钢不可过），会破砖、守家、追敌、捡道具；计算缓存且有界。
- 必须覆盖 1v1、2v2、3v3，全电脑、人机混合、托管/取消托管、恢复存档、12 张地图、自然摧毁基地或 10 分钟和局。
- 断线/刷新/服务重启保留地图、坦克、基地、子弹、道具、重生/护盾/铲子计时、科技和输入序号；不重播旧动画/音效。

## 10. 失败场景（先写测试）

1. 上架前创建/加入/观战被拒；管理员上架后开放；部署不自动上架。
2. 人数 1、3、5、7；非法地图/坦克；未知字段；超长请求；重复开始。
3. 选坦克：重复选择、超时补齐、电脑选择、双方重复类型、选角期间移动/开火被拒。
4. 输入：错误 epoch、seq 回退/跳跃、重放、结算后输入、六席并发、断线/失焦中立。
5. 地形：砖 8×8 破坏、钢、水阻挡、树遮挡、冰滑行、坦克互撞、边界。
6. 子弹：高速不穿透、子弹互消、友军不伤害、己方基地免疫、敌方基地一击、同一帧双基地和局。
7. 道具：六种效果、30% 掉落、上限 4、25 秒消失、手雷不掉落、科技死亡清零、铲子 20 秒、时钟 8 秒。
8. 重生：4 秒、2 秒护盾、出生点占用换点、基地被毁立即结束、10 分钟和局。
9. 1v1/2v2/3v3 的出生点子集、队伍结果、Rank/Winner/Score 长度。
10. 恢复：地图、坦克、基地、子弹、道具、计时、序号；观战隐私；事件上限。
11. 电脑：三种人数 × 12 图完整对局；人机混合；托管；恢复；不得读 RNG/未来掉落。
12. 前端：2/4/6 席布局；320 px；资源可见；坦克 ≥24 px；键盘/触屏/多指；Canvas 实际绘制；动画；减少动态效果；对比度；结果浮层。
13. 素材：manifest/SHA-256/MIME、CDN 回读、atlas 帧边界、音频非静音/无削波/循环、CDN 失败回退。
14. 安全：客户端不能提交位置/HP/胜负；管理员接口权限。

## 11. 素材契约

- 商业媒体只放 OSS 与 `.local`，不进 Git；前缀 `bga/v20261002-1/battlecity/v1/`。
- `atlas.json` 帧名固定：
  - 地形 8×8：`brick`、`steel`、`water`、`trees`、`ice`。
  - 基地 16×16：`base-top`、`base-bottom`、`base-destroyed`。
  - 坦克 16×16：`tank-<class>-<top|bottom>-<up|right|down|left>-<0|1>`，class 为 `balanced|basic|fast|power|armor`。
  - 子弹 6×6：`bullet-<top|bottom>`；爆炸 16×16：`explosion-0..2`；护盾 16×16：`shield-0..1`。
  - 道具 16×16：`powerup-star|grenade|helmet|shovel|tank|clock`。
- 音频键：`bgm-battle`、`bgm-start`、`sfx-fire`、`sfx-brick`、`sfx-steel`、`sfx-explode`、`sfx-powerup`、`sfx-shield`、`sfx-base`、`sfx-win`、`sfx-lose`。
- `manifest.json` 记录字节、SHA-256、尺寸/时长、源/生成器；`sources.json` 记录 ROM/归档/生成器哈希和授权说明；`labels.json` 记录 UI 名称。
- 封面为原创 SVG，不使用 ROM 标题画面/盒装封面/英文标语。

## 12. 验收命令

```sh
. scripts/env.sh
python3 -B scripts/check_game_boundaries.py
npm --prefix web run build
npm --prefix web test -- battlecity
go test -count=1 ./games/battlecity/...
go test -race -count=1 -timeout=20m ./games/battlecity/...
go vet ./...
python3 -B scripts/battlecity/verify-assets.py --dist
python3 -B scripts/battlecity/verify-assets.py --oss
python3 -B scripts/battlecity/check-browser.py
python3 -B scripts/ci/scope.py run --game battlecity --backend
python3 -B scripts/ci/scope.py run --game shared
```
