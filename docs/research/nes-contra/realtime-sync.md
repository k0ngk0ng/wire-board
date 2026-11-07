# 魂斗罗实时同步修复

2026-10-28。常规同步改为输入帧、校验摘要和必要状态，不再发送静态地图、背景画面或音频历史。客户端使用已校验的 CDN 素材和本地 Wasm 重放。完整检查点只用于首次接入、重连和失步恢复；其关卡清单也只保留版本身份。

WebSocket 游标按连接独立保存，成功写入后才推进。短窗口保留少量冗余输入，缺口超过服务器保留范围时由该客户端恢复快照。游戏投影在平台锁外执行，输入与房间元数据来自同一版本的独立副本。运行中换关只限制尚未就绪席位的输入，不暂停全员，也不自动托管。首次开局仍等待所有真人完成公共加载流程。

## 验证

线上 v1.39.1 基线在约 9.7 秒内收到 3.96 MB 数据，实际输入确认中位数约 3.5 秒，95 分位约 4.6 秒，公共连接延迟约 28 毫秒。连接延迟不能代替按键响应测量。

候选版本的实际主程序与双浏览器验收通过。仅阻塞手机端的 TCP 返回数据时，电脑端继续推进 126 帧，慢端推进 0 帧；电脑端输入确认最长约 9.5 毫秒，常规消息最大 3,017 字节。恢复慢端后正常追赶。该结果来自本地测试网络，线上效果须在部署后复测。

验收同时覆盖单人、双人、初始加载、PC 键鼠、320px 原生触摸、控制器设置、全屏、观战、暂停、刷新与丢帧恢复。服务端规则、实时通信与竞态检查分别保留结果。

画面端按背景内容缓存，复用 ImageData，并把图形页校验移到加载阶段。241 帧实际战斗画面的 59,228,160 个 RGBA 字节与原实现完全一致。桌面样本的平均绘制耗时：静止画面约 0.669 → 0.139 毫秒，移动射击约 0.692 → 0.397 毫秒；不据此推断手机性能。

结果与文件摘要见 [verification.json](evidence/realtime-sync/verification.json)，完整浏览器记录见 [browser-result.json.gz](evidence/realtime-sync/browser-result.json.gz)。测试账户使用临时测试数据库，脚本退出后清理；线上烟测账户另行清理并核验。

## 复现

在仓库根目录运行，输出目录和标签须使用未占用的名称：

```sh
node web/scripts/test.mjs contra
go test ./games/contra/server ./internal/gameapi
go test -race ./internal/server -run 'Test.*Realtime|TestClientLoading'
npm --prefix web run build
python3 -B scripts/contra/accept_main.py \
  --runtime .local/contra-web-runtime/public/manifest.json \
  --out .local/contra-network-accept \
  --browser-label compact-sync-recheck
```

正式发布仍复用 main 已验证的构建，由新版本 tag 触发。此次改动不改变 CDN 素材发布规则，不把魂斗罗音频或 Wasm 加入正式服务器镜像。
