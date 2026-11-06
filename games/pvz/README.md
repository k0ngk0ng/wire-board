# 植物大战僵尸

PC 初代经典白天草坪的植物／僵尸攻防对战。支持 1v1 和 2v2，双方共用一块 5 行 × 9 列草坪，每位玩家独立持有资源、卡牌冷却与单位。比赛包含两个攻防回合，第二回合互换阵营。

## 玩法

植物方点击卡片后点击格子种植，使用铲子清除自己的植物。僵尸方点击兵种后点击进攻行，僵尸入场后自动行走、啃咬和使用固有能力。队友可支援任何一行；不能使用队友的钱或铲除其植物。

- 植物方使用阳光；基础阳光按同阵营人数分配，向日葵收入归种植者。
- 僵尸方使用独立脑力收入，随比赛推进逐步增加。僵尸价格、冷却和进攻入口间隔是对战设计参数。
- 2v2 的团队初始资源与基础收入按 1v1 总量分配，保持四个独立钱包。樱桃炸弹另有短暂团队使用间隔。
- 每行保留一台割草机。割草机消耗后僵尸突破房屋边界即结束当前攻防回合；守到时限则防守成功。
- 只有一队攻破时该队获胜；双方攻破时比较耗时；双方守住或用时相同为和局。换边重置场地、资源和冷却，并使用相同初始随机条件。
- 回合时限可选 3／6／10 分钟，布阵时间可选 10／20／30 秒。

首批植物：向日葵、豌豆射手、双发射手、寒冰射手、坚果墙、土豆雷、樱桃炸弹、高坚果。首批僵尸：普通、路障、铁桶、读报、撑杆、橄榄球。

PC 支持鼠标与平台方向导航／确认／取消，键位通过统一控制器设置修改。手机支持点击选卡和草坪触摸；全屏沿用平台入口。所有人数和阵营支持电脑及主动托管，真人断线保持手动控制。

## 原版依据

取证样本为 PC 英文版 `1.0.0.1051`。REA/Ghidra 已核对费用、冷却、生命、护甲溢出、弹丸伤害、生产／攻击随机计数、啃咬和减速；图像、分层动画和音效由原包提取。样本 SHA-256、Evidence ID、函数地址与重复提取方法见[取证报告](../../docs/research/pvz/README.md)和[素材说明](assets/README.md)。

原版 Windows 程序未执行，本实现未宣称逐帧动态等价。僵尸 `_ground` 动画的周期内步态采用周期均速近似；随机数使用独立、可存档的确定性实现，按已恢复的范围采样。碰撞盒、跳跃时长、僵尸经济、多人限制和胜负赛制属于平台适配。泳池、夜晚、屋顶及其余单位不在当前版本范围。

## 模块与同步

`engine/` 保存确定性整数规则；`server/` 对接平台认证、时钟、电脑、存档与结算。浏览器将同一 Go 引擎编译为 Wasm，`web/` 负责交互、资源加载和绘图。

正常同步发送有界新增操作、逻辑帧、个人资源与摘要；首次接入、换边和游标缺口使用完整快照。浏览器重演已确认操作，并叠加最多 250 毫秒的本地预测。音效只消费权威事件。慢连接使用独立游标，不阻塞服务器或其他玩家。

客户端 Wasm、图集和音效由构建流程纳入公开资产，正式发布时通过统一 OSS/CDN 清单上传、校验并从服务器包剥离；研究原包保存在 `.local/pvz-research/`，不参与发布。

## 验证

实现前的失败场景与 E2E 保存在 `tests/`。从仓库根目录运行，所有临时目录和缓存保持在工作区内：

```sh
. scripts/env.sh
go test -json -count=1 ./games/pvz/tests
npm --prefix web test -- pvz
.local/pvz-research/venv/bin/python -B games/pvz/build/verify_classic.py --output .local/pvz/asset-verification.json
npm --prefix web run build
python3 -B scripts/check_game_boundaries.py
```

素材校验使用含 Pillow 的工作区解释器；环境重建方法见[素材说明](assets/README.md)。

真实浏览器验收使用生产构建和仅在测试二进制启用的服务：

```sh
WIRE_BOARD_PVZ_BROWSER=1 go test ./games/pvz/tests -run '^TestPvZBrowserFixture$' -count=1 -timeout=35m
```

服务地址写入 `.local/pvz/browser/url`。在另一终端将该地址设为 `PVZ_BROWSER_URL`，分别运行 `python3 -B games/pvz/tests/frontend-browser.py` 和 `python3 -B games/pvz/tests/integration-interaction.py`。验收后创建 `.local/pvz/browser/stop` 退出测试服务。

可核验产物分别保存在 `.local/pvz/backend/`、`backend-http/`、`integration/`、`runtime-e2e/`、`frontend/` 和 `interaction/`。HTTP 完整电脑矩阵使用明确标记的测试时钟加速，不修改资源、位置或结果；浏览器操作与网络测量使用真实时间。手机视区和原生触摸派发属于浏览器仿真，不冒充手机真机验收。
