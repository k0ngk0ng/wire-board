# 石器竞技场

1v1 至 5v5 的回合制宠物对战。每局双方共用随机候选，各玩家独立选择人物属性、出战宠、骑宠、宠物技能、装备与有限补给。选配锁定后，每轮先选择指令，再按敏捷逐个行动。

## 规则

| 内容 | 本局候选 | 每人配置 |
| --- | --- | --- |
| 人物属性 | 相同的 480 点 | 体力、腕力、耐力、速度自由分配，非负整数，总数 480 |
| 出战宠 | 从 72 种有完整战斗动画的宠物中随机 20 种 | 1 只，没有替补 |
| 骑宠 | 从 9 种有原版骑乘映射的宠物中随机 6 种 | 1 只，落马后不能重骑 |
| 宠物技能 | 攻击、防御、破防和落马术保底，另随机 10 项 | 最多 7 项，包含攻击与防御，同族技能选一档 |
| 帽子、衣服 | 每类 4 件 | 每类必选 1 件 |
| 武器 | 6 种不同武器类型 | 必选 1 件 |
| 饰品 | 6 件 | 最多 2 件，不重复 |
| 回血、回气、解除异常 | 每类 2 种选择 | 3 份回血、2 份回气、1 份解除异常 |
| 战术道具 | 4 种 | 最多 2 种，各 1 份 |

所有选择均不消耗公共库存，双方可以使用同种宠物或同件装备。角色、宠物和骑宠采用固定竞技预算，装备采用明确的竞技数值。没有捕捉、永久成长、备用宠、换宠、复活或骑宠转为出战宠。

默认选配 180 秒，每轮选择指令 25 秒，最多 25 回合；房间可调整时间和回合上限。人物全灭的一方失败，宠物与骑宠不会单独延长已经失败的一方；达到回合上限按显式和局结算。含电脑的对局沿用平台不计分规则。

## 实现与原版对照

`server/` 为独立 Go 模块，`web/` 为 React 界面。平台提供房间、鉴权、事务、幂等、观战、加载、全屏、音频、托管及战绩。游戏快照保存候选、配置、随机状态、未结算指令、消耗品、战斗事件与播放期限。

基础属性、物理伤害、防御分布、元素相克和骑乘系数以源仓库 2.5 的 `_BATTLE_NEWPOWER` 分支为依据。`tests/reference/classic.c` 保留原版伤害、属性与骑乘原语，由单独 C 程序在 E2E 中对照生产回合的实际伤害；来源摘要见 `tests/reference/source.json`。该对照覆盖基础公式，不代表已经证明整个原版引擎完全一致。

技能、装备精灵、补给、控制时长和抗连续控制窗口采用本竞技场的明确规则。`server/catalog.go` 为规则数据来源，不能从图标或技能名称推测数值。左右队形及方向使用原网页客户端的 `BATTLE_SLOT_POINTS`、`BATTLE_MONSTER_PLACE_NO` 与方向 3／7。

战斗操作沿用原版的画面内菜单：先选人物攻击、咒术、防御或道具，再点击合法目标；随后选择出战宠技能和目标，两条指令一起提交。右键、返回按钮或 Escape 可取消当前选择。战场不常驻名字，悬停可识别；点击人物或宠物可查看详情，再点同一角色、详情框或关闭按钮可收起。公共玩家信息在手机上保持横向单行，全屏只放大战斗区并保留菜单、详情和记录。

PC 的四个战斗按钮在同一横行，人物目标确认后自动进入宠物技能；不显示人物／宠物切换页签。320px 手机保持紧凑布局，操作顺序相同。

## 素材

直接引用既有 `https://cdn.ichenj.com/stoneage`，图片位于 `assets/`，音频位于 `audio/`。没有复制或上传图片、音频。`web/art.json` 只包含动画帧、URL 路径、尺寸、偏移、字节数和 SHA-256；素材加载检查摘要，复用浏览器缓存与已解码图片。

选配阶段加载本局候选预览；锁定后加载已选人物、骑乘和出战宠动画。全部真人确认这些资源就绪后才开启首轮时限，电脑自动确认，观众不阻塞。攻击、扣血、落马和死亡使用同一服务器时间轴，刷新恢复当前进度。

如需根据源仓库重新核查索引，从当前仓库执行：

```sh
python3 -B games/stoneage/tools/build_catalog.py --source /path/to/stoneage
python3 -B games/stoneage/tools/build_reference.py --source /path/to/stoneage
```

两个入口只读取源目录，输出均位于本游戏目录。来源与数量见 `source-manifest.json`。

全部可能使用的原始图片共 9,477 张，26.14 MiB；其中动画帧 9,411 张，另有装备图标 57 张、菜单图标 8 张和背景 1 张。该数量覆盖全部候选及造型，不是每局下载量。分类统计见 [asset-audit.json](asset-audit.json)，合图提升分辨率的测算见 [upscale-assessment.md](upscale-assessment.md)。

高清版本覆盖同一组 9,477 张图片，宽高均为原图的 2 倍，总计 413.73 MiB。图片保存在源 StoneAge 仓库的 `client/web/assets/hd/arena-2x-gpt-image-2.5-v1/`，发布地址沿用石器 CDN 根；`web/hd.json` 仅保存该版本地址、尺寸、字节数和摘要。动画继续使用 `art.json` 的原始逻辑尺寸、偏移和 tick；加载容量按高清文件统计。客户端代码和服务器包不包含图片正文。

生成包附带同路径映射、动画索引和 `review.html` 原图／高清同步播放页，源 StoneAge 客户端可按相同逻辑尺寸复用这些文件。高清生成的自动检查覆盖尺寸、原始透明轮廓、原色投影、局部对齐、色差和按钮状态；不把这些检查扩大解释为每帧内部细节与原版完全相同。

## 验证

失败场景先于实现登记在 [FAILURE_SCENARIOS.md](tests/FAILURE_SCENARIOS.md)。缓存及临时文件必须留在当前仓库：

```sh
. scripts/env.sh
python3 -B scripts/ci/scope.py run --game stoneage --backend
go vet ./games/stoneage/...
npm --prefix web run build
```

模块测试覆盖随机池完整性、私密选配、非法配置、全部人数电脑整局和恢复。HTTP 测试使用独立登录会话、真实 SQLite、全部五种人数完整对局、重复请求和重启。`TestStoneageNativeAutoPlayAllModesHTTP` 使用真正的平台定时调度，包含电脑席位、真人托管、手动接管和恢复；`TestStoneageNativeReferenceCombatE2E` 使用 C 编译器验证原版公式对照。

浏览器夹具和交互脚本：

```sh
. scripts/env.sh
WIRE_BOARD_STONEAGE_BROWSER=1 go test ./games/stoneage/tests \
  -run '^TestStoneageBrowserFixture$' -count=1 -timeout=35m
# 在另一个终端执行；从 .local/stoneage/browser/url 读取本次专用地址。
python3 -B games/stoneage/tests/browser.py
STONEAGE_BROWSER_PLAYERS=10 STONEAGE_BROWSER_FAIL_ASSET=1 \
  python3 -B games/stoneage/tests/browser.py
```

浏览器使用独立 agent-browser 会话和工作区 profile，检查真实鼠标、键盘、触屏、自由加点、320px 布局、原版 CDN、全屏、出招和刷新。交互验收还覆盖详情开关、生命与气力条间距、10 人横滑、画面内目标选择，以及资源失败时阻塞首轮计时并重试。结束夹具时在 `.local/stoneage/browser/` 创建 `stop` 文件。

主要证据保存在 `.local/stoneage/verification/` 和 `.local/stoneage/browser/`。公共 HTTP 夹具额外在 `.local/poker/stoneage/http/` 保存真实协议轨迹。战斗分支记录明确标记为受控初始快照，不能替代完整对局和实际浏览器结果。
