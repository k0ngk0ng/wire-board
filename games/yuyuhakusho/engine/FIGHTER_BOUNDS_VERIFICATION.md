# 人物边界与相机采样：实现前验证范围

恢复 `0x208d8..0x20916` 的完整返回函数。该函数先把世界 X 的高 word 按左右界顺序裁切，保留低 word 小数，再向 X／Y 两组位置样本写入完整 long。原外层只在生命不等于 −1024 时调用；这项生命门槛不移入本函数。

可能失败的场景：把 word 比较和写入改成整个定点 long 裁切；使用无符号左右界或宽化导致错误；左右界反转时改变原先左后右的顺序；把 Used 的 byte 偏移当成样本数；错误使用有符号饱和判断；达到 63 时没有回退到偏移 60；满后清空或循环缓冲，未反复覆盖最后一槽；丢弃 Y 小数或覆盖未写槽；无效调用者存储发生部分修改；观察器改变原版整帧；把一函数连续样本当作完整人物或世界更新。

先保存自然完整调用与明确私有输入的原 CPU 边界，再编写测试并保留缺实现失败，最后编写独立 Go 规则。比较完整 256-byte 人物、两组共 128 bytes 样本及计数；隔离覆盖位宽、边界反转、饱和、旧槽与连续覆盖。不屏蔽未知人物或样本字节。

## 已验证结果

4,134 次自然完整调用、1,980 次原 CPU 隔离调用与 4 条单初态流共 128 步，与独立 Go 完整状态一致。全部 3,600 帧 RAM／视频与原版基线一致，隔离函数没有许可范围外 RAM 改写。原生竞态和 Node/Wasm 的 3 项聚焦测试均通过，实际缺少 `FighterBoundsState`／`StepFighterBounds` 的红阶段先于实现。

隔离矩阵覆盖 X 高 word 的有符号极值及临界位置，小数 0／1／65535，Y 的完整 long 极值；左右界包含常规、相等和反向组合。Used=0／4／12／56／60／63／64／65／255／32768／65535 均由原 CPU 验证。比较遵循原先左后右的顺序，Used 按无符号 word 判断；达到 63 后先变成 60，写槽 15，再变成 64。

连续流只设置一次初始样本，之后仅注入声明的上游 X／Y 位置；Go 不读取原版中间 Used 或样本。32 步序列覆盖首次填满和持续覆盖末槽。函数本身不读生命，私有原版调用也包含 −1024；外层应严格按「生命不等于 −1024」决定是否调用，不能扩大成所有负生命均跳过。

低于 63 的非 4-byte 对齐偏移不属于声明的 long 样本存储，独立 API 原子失败；本轮未把损坏低偏移的跨数组硬件写入当作合法状态。计数大于等于 63 会先饱和，因此 63 和 65535 等原始奇数仍正常写最后槽，不能提前按奇数拒绝。

静态指令复用已经确认的 `fighter-stages.txt` 中完整 `0x208d8` 段，来源摘要与原样指令保留在 [fighter-bounds-core.txt](../../../docs/research/md-yuyuhakusho/static/fighter-bounds-core.txt)，没有重复运行相同分析。来源、夹具和真实测试日志见[验证记录](testdata/fighter-bounds-validation.json)。这些结果仍不是完整外层人物或世界更新。

## 复跑

```sh
PYTHONDONTWRITEBYTECODE=1 python3 scripts/yuyuhakusho/capture_fighter_bounds.py \
  --out .local/md-yuyuhakusho/fighter-bounds-replay
```

完整包执行 `go test -race -count=1 -run '^TestFighterBounds' ./games/yuyuhakusho/engine`。聚焦方式从 `games/yuyuhakusho/engine` 执行 `go test -race -count=1 fighter_bounds.go fighter_bounds_test.go`，Wasm 使用研究工具说明中的 Node 执行器；Go 缓存和临时目录均设为 worktree 内 `.local/` 路径。父阶段仍须运行整个游戏包回归。
