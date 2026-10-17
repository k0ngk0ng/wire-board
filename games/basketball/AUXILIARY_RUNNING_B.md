# 跑动无方向 B 恢复验收

本次先列失败场景，再提取原函数 oracle、编写先失败测试，最后接入实现。输入保持原 `$68` 请求。

- 32 个队伍／角色表项不能合并：handler 9 的四项、handler 8 的一项及负 handler 的 27 项须逐项验证。
- 负 handler 必须直接进入 `$23`，不能二次接受 pending request，也不能改为投掷。
- handler 9 每次发声；进入时设置道具动作 10、清 attachment、人物 loops=3。物体必须使用自己的动画、owner 当前朝向和保留的小数／高字节。
- handler 8 读取当前 actor 的 Pressed A，不能读取 Held、另一席位或把未知输入当 0。基础三帧与 cursor=3 后的释放尾帧分别验证。
- `$056F/$0570` 与两个辅助对象 ObjectLink 是同一状态；进入、释放、门装置及非默认 object4/object5 选择不得分叉。
- 辅助动作 8／9／10／11 的进入／继续、动画边界、重力反转、attachment 写入对象、owner 释放及运动先后顺序必须与原函数一致。
- attached 玩家与动态物体 Kind 使用真实世界来源，未知引用明确报错，不能默认角色 0。
- 真实原输入→人物→道具持续链使用单一初态；覆盖 pickup、跑动、无方向 B、handler8 的 A 释放，不能逐帧重置为 oracle 状态。
- Native、race、Wasm 均运行明确命名测试；保留 fixture、来源 SHA、before/after 日志，避免只命中邻接回归。

既有证据：`ev_2b5588b7f56a312eba6a999f127fbd52cb7aa05f20460fa4bfe45970af721552`、`ev_c979978930d5789a6f41a9325fc35452a28130003aae813311baad41f8c8dc18`。这是函数与显式初态验证，不宣称完整自然比赛已覆盖全部角色。

## 实现和原版结果

`world_auxiliary_running.go` 实现人物68与辅助9／10／11。辅助8使用与原3相同的投掷入口及独立的五帧动画。世界分派先接受输入请求，再处理角色表；负handler直达已解析的携物刹车逻辑，不再二次接受请求。原有4F、captured和launched分支保留。

`WorldActionContext.ActorPressedButtons` 为指针，nil代表未提供；连续世界驱动从真实 `WorldLoopMemory.Pressed` 绑定。人物handler8只读取对应席位的A边沿。`056F`和`0570`始终复用两个辅助对象的 `ObjectLink`，动作9清除0570指向对象的attachment；动作11按056F的原无符号比较选择另一侧两人中X距离更近的一人，分轴计算原速度，再清两个link和owner。这里FF不能当负数比较；新增别名probe先复现了选错一侧的差异，再修正。

动作10从真实owner对象读取loops、pose和facing，允许已知的玩家、球、道具和篮框来源，保留原整数低字节写入与小数／高字节。不能将非玩家owner替换成玩家0。原对象动画完结后才恢复owner attachment。

REA后续证据：`ev_e5908940b5fa47d5b7cfd97cfe65b1def1cd51a8283bdffbcae284f4ebf46a3c`，覆盖B0B1／B22A／B2F2／B325／A7BC／A80E。

原版采样脚本为 `scripts/basketball/probe_auxiliary_running.py`，夹具及来源SHA位于 `engine/testdata/auxiliary-running-*`。948个人物调用覆盖32角色、两个人物索引、两道具、两朝向、进入与动画边界、attached支持及残留请求；816个对象调用覆盖8／9／10／11、不同owner和链接别名。每次比较整个世界动作状态，人物调用另比较声音序列。

12条单初态真实原805C输入→E18B人物→E18B道具链共750周期、1500个动作边界。6条从持物跑动开始，6条从道具旁A拾起开始，再双击方向进入跑动，松开方向单按B。handler8另单按A释放浮空道具。测试同时断言实际拾起、跑动、目标道具动作及释放阶段，不能只有人物68请求而未推进目标道具。

这些链来自明确的Court3初态，没有中途灌回oracle，也不等于从开机菜单到完整比赛的自然流程；键盘／手机端E2E由平台验收另行记录。

先失败记录：`.local/basketball/engine/auxiliary-running-before.log`（缺上下文API）、`auxiliary-running-dispatch-before.log`（缺handler9和aux8）、`auxiliary-running-alias-before.log`（FF链接选错目标）。定向执行结果位于同目录 `auxiliary-running-after.log`，race／Wasm及邻接回归分别保留 `auxiliary-running-{race,wasm,neighbor}.log`；必须核对各测试实际运行记录，不能仅凭进程退出码判断。

## 复跑

```bash
source scripts/env.sh
PYTHONDONTWRITEBYTECODE=1 .local/board-art-env/bin/python scripts/basketball/probe_auxiliary_running.py
go test -race -v ./games/basketball/engine -run 'TestWorld.*AuxiliaryRunning'
basketball_goroot=$(go env GOROOT)
GOOS=js GOARCH=wasm go test -v -exec "$basketball_goroot/lib/wasm/go_js_wasm_exec" ./games/basketball/engine -run 'TestWorld.*AuxiliaryRunning'
```
