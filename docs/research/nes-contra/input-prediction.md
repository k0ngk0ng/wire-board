# 魂斗罗本地输入预测

魂斗罗原先先等待服务端执行，再重放确认帧并排队显示。v1.39.2 线上记录中的一次组合按键，输入 ACK 约 25 毫秒，对应输入帧显示约 250～297 毫秒；这两项不能混作同一个操作延迟指标。热血篮球已有本地预测，魂斗罗此次补齐这一环节。

## 实现边界

- 确认状态继续逐帧校验服务器摘要。另在 Wasm 内复制独立状态用于预测，复制和校正不通过网络传输完整存档。
- 玩家本地按键驱动预测画面。确认帧到达后，从确认状态重新复制并重放尚未确认的本地输入，保持显示进度；预测最多领先 12 帧，确认数据停滞超过 250 毫秒时停止预测并释放本地输入。
- 生命、结果和平台音频只使用确认状态。预测不执行终局提交，不提前显示生命消耗、死亡或关卡切换，也不把推测音效送给平台播放器。
- 暂停、观战、禁用控制及恢复存档使用确认画面。恢复过程等待图形准备完成；失焦、隐藏、取消和卸载清理预测输入与独立状态。
- 保留高刷新率两次绘制之间的短按边沿，直到下一次 60 Hz 预测步消耗。发送队列中过期的普通输入丢弃，最终松开动作仍须发送。
- 常规网络更新沿用 v1.39.2 的紧凑输入协议，不增加地图、画面或音频历史。

## 验收

使用只含本次魂斗罗与规范改动的独立构建，排除并行开发中的控制器实现。测试在真实主程序、双浏览器和标准开局中执行，不修改游戏状态或生命数量。

将一个客户端的服务器返回数据和 ACK 阻断，再通过原生键盘／触摸输入方向。测量角色区域内原版配色像素的位置变化，并确认权威帧仍未更新，不能仅凭预测帧号或逻辑坐标宣布响应。

| 场景 | 按下到角色像素变化 |
| --- | ---: |
| PC 键盘 | 43.3 毫秒 |
| 320px 触屏 | 28.0 毫秒 |

上述为本地测试网络的两次受控测量，不代表所有设备或公网延迟的分位数。预测期间可见帧间隔中位数和 95 分位均约 16.7 毫秒；单独阻塞慢端时，快端输入 ACK 最大约 6.3 毫秒。

完整回归覆盖单人、双人、初始加载、暂停、生命损失、全屏、设置重映射、观战、刷新及隐藏时恢复。83 项魂斗罗前端检查和运行桥竞态测试通过；另外使用真实 Go/Wasm 验证独立复制、校正、会话复用、预测上限、双人输入和高刷新率短按。

来源摘要与结果见 [verification.json](evidence/input-prediction/verification.json)，完整浏览器记录见 [browser-result.json.gz](evidence/input-prediction/browser-result.json.gz)。

## 复现

在仓库根目录运行，输出路径和标签须使用未占用的名称：

```sh
mkdir -p .local/tmp
export GOCACHE="$PWD/.local/go-build"
export GOMODCACHE="$PWD/.local/go-mod"
export GOTMPDIR="$PWD/.local/tmp"
node web/scripts/test.mjs contra
go test -race ./games/contra/runtime
npm --prefix web run build
python3 -B scripts/contra/accept_main.py \
  --runtime .local/contra-web-runtime/public/manifest.json \
  --out .local/contra-prediction-accept \
  --browser-label prediction-recheck
```
