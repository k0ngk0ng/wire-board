# 传球落地与反弹交接

实现前失败场景：下落穿过地面、恰好触地和小数高度归零；上升球误判落地；横向正负速度丢失；动画与朝向变化；落地没有清除传球人的接球限制；发射后当步落地被发射初始化重新覆盖。

固定视频运行在第 8,855 帧、第 2,949 次逻辑更新前遇到 `passed ball floor collision not reconstructed`。完整原始失败记录为 `.local/basketball/live-browser/cold-period-plan3.jsonl`。验收没有缩短时钟、替换种子或注入原世界状态。

REA 原函数档案 `ev_f4247f31fadd97f1841f96049378e4db19e2d25df3ff6603e3489f91eaefdc6c` 确认 bank 6 `$B649..$B65E`：先更新动画、执行公共运动／场地碰撞，若腾空位已清除，则设球动作为 6（下次分派进入反弹），并将排除接球者设为 255。球仍在空中时保留原排除值。完整档案、反编译和源码摘要在 `.local/basketball/rea/pass-floor/analysis/`。

`verify_pass_floor_original.py` 运行未改动的原 `$B5F4` 指令，模拟其实际 MMC3 PRG 换页写入，遇到意外设备写入则失败。基底是原纽约场地采样的 RAM，随后显式设置传球中的位置、速度、朝向等参数。因此 216 个用例属于原函数隔离执行，**不是 216 次自然对局**。其中 84 个落地转反弹，另 132 个保持飞行；比较完整球体、动画、请求、持有者与排除接球者。

先运行测试，记录原实现在第一个落地用例失败，再复用原公共球体场地物理、补入反弹动作及世界中的排除接球者清除。既有 33 次自然传球调用继续匹配。

首批 108 个用例通过后，进一步独立变化旧图像翻转位与球体朝向，扩展到 216 个用例。这暴露了传球动画错误沿用旧图像翻转位的问题；先保留 `pose-before.log` 中失败的原函数对照，再改为原 `$8726` 读取的对象朝向。没有改写原版期望结果。

```sh
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/basketball/verify_pass_floor_original.py
go test ./games/basketball/engine -run 'TestPassFloorAgainstOriginalFunction|TestPassedBallFlightAgainstOriginal|TestBallFloor' -count=1 -v
go test ./games/basketball/engine -run '^TestRuntimeColdFirstPeriodAndRestore$' -count=1 -v
```

修复后固定视频运行推进至第 9,638 帧、第 3,210 次逻辑更新，下一处未恢复分支是人物 3 与道具 5 的接触响应。该首差保存在 `.local/basketball/rea/pass-floor/cold-period-after.jsonl`，第一节及完整比赛仍未验收通过。
