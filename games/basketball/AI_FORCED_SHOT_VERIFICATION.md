# 原版强制投篮计划入口

## 实现前失败场景

- mode23/24/25共用A49E，仅首次进入设置Entered、Counter=0，并按A4D1表选择4/5/7；后续保留已经改变的parameter，不重新抽签。
- 计划4先按原B306选择ShotCycle，再AimShot、mode0／nextPlan22，并按席位朝对方；和普通计划0的继续策略不同。
- 计划5只改parameter2、方向朝对方、nextPlan22；本次不能直接执行计划2的跳跃，也不消耗RNG。
- 原有计划2/7借助原B383分派复用，禁止把强制入口先改为mode6而丢失原Mode；错误时原子返回初态。
- 自然A49E入口的完整状态与隔离原函数矩阵分别保留，不把其中任一类证据冒充完整比赛。

## 证据和验收

A49E来自REA `ev_eef21fb30302a3ec59b8b3dc097be9a7fa2a559339d598963a8489a9d646759d`，B4B9/B4D2来自 `ev_8bac8418e07eaa8a98c2ac6472d25618bfc7cb159ee632963fe08b31581cafda`。原A4D1表[4,5,7]决定首次计划，后续原B383继续使用当前parameter，不重新抽取随机值。

TDD后全部10次原自然A49E调用完整WorldInputState匹配，原18,000帧RAM/PNG一致。另有1536组原A49E＋B383隔离执行，覆盖23/24/25、Entered、初始/持续parameter、角色、grade、zone与RNG，所有控制和动作命令精确匹配。自然与隔离证据分别保存，未相互替代。

完成本分支后，固定cold默认配置和seed12345678达到6000逻辑更新，重复两次结果相同，最终scores=[12,55]，轨迹SHA-256为`4a81c896e779e4bdec6ca7d8bd461c0fb92cdc20bc18a8e3ec580b4794afcef8`。原生与Wasm各重复两次均得同一摘要。这里仍未加入视频时钟，不能称为完整比赛或整个游戏已复刻完成。

本轮自然/隔离测试与Wasm日志位于 `.local/basketball/rea/ai-forced-shot/`，可重复cold结果位于 `.local/basketball/rea/cold-runtime/{native-result.json,wasm-result.json}`。

```sh
PYTHONDONTWRITEBYTECODE=1 .local/monopoly/venv/bin/python scripts/basketball/capture_ai_forced_shot.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_ai_forced_shot.py
source scripts/env.sh
go test ./games/basketball/engine -run TestAIForcedShot -count=1
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_cold_runtime.py
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_cold_runtime.py --wasm
```
