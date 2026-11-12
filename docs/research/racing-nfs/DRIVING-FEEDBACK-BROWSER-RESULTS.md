# 赛车驾驶反馈：浏览器验收记录

计圈、连续音频、驾驶舱与控制语义验收使用的浏览器构建 `index.html` SHA-256 为 `27ddd412ec8422257aa1a4504ac0fde5900a607118e82b5d6f79543a615301e7`，原字节身份保存在 `.local/racing-followup/semantic-build-27ddd412-identity.json`。后续仅修改 `ClassicGauges.tsx`、`classic-gauges.ts`、`TrackCanvas.tsx` 的展示读写及 `style.css`，提供紧凑数字、小号挡位和仪表位置；布局、健康操作图和隔离性能另以新构建重验。其余 90 项源码摘要逐项相同，物理、计圈、音频和输入逻辑保持原字节。公开机器摘要分别标记证据所属构建，没有将旧截图或旧性能冒称新结果。长圈期间没有重新构建或修改运行时。

## 真人离道、计圈和 R

四条赛道均从原起点以真人席位普通 HTTP `action/drive` 控制驾驶，没有启用托管、调用电脑策略或写入车辆／圈数／时钟。每轨两次实际离开路缘后自行返回，原生 R、2 秒后再次 R、真实页面刷新后继续驾驶，最终完成三圈。

| 赛道 | 首次路缘外最大距离 | 再次路缘外最大距离 | 距原截止时间的完赛余量 | 第二次 R 间隔 |
| --- | ---: | ---: | ---: | ---: |
| France | 17.342 m | 17.149 m | 133.747 s | 2137 ms |
| Coastal | 23.806 m | 23.811 m | 25.689 s | 2166 ms |
| UK | 14.428 m | 14.467 m | 10.582 s | 2166 ms |
| Park | 22.509 m | 22.444 m | 233.978 s | 2105 ms |

R 前实速分别为 22.326、10.716、15.895、12.595 m/s。快速重复按 R 保持原 `lastReset`／`resetUntil` 并显示「正在复位」；两秒动画结束后、不足五秒再次 R 均成功。已完成的一圈、epoch、起始时间和原截止时间在 R 与刷新后保持一致。截图保留真实 1/3、2/3、3/3 和完赛画面。

距离按原地形真实可达区域测得。测试没有以任意横向距离设置计圈门槛，也没有为达到 20 m 穿过固定墙、屋顶或缺地面。铺装巡航目标 22 m/s 是普通驾驶输入计划；弯道制动、离道 4 m/s 和比赛期限保持不变。控制计划见 [DRIVING-FEEDBACK-CIRCUITS-FAST.json](DRIVING-FEEDBACK-CIRCUITS-FAST.json)。

完整证据位于 `.local/racing-nfs-browser/final-circuit-<track>-03/`。原 Python 报告因省略的 `autoPlay:false` 字段触发校验器 `KeyError` 而保留红色状态；原始真人三圈数据和截图已完整输出。修正校验器读取协议默认值后，单独的 `final-circuit-<track>-03-recheck.json` 重新执行全部严格断言，四轨均通过，并绑定原生工件、原红报告和新校验器的 SHA-256。没有改写原报告或重新解释失败为通过。

可重复执行的工件重验：

```sh
python3 -B scripts/racing/recheck-circuits.py \
  --track uk \
  --run .local/racing-nfs-browser/final-circuit-uk-03 \
  --build-identity .local/racing-followup/semantic-build-27ddd412-identity.json \
  --out .local/racing-nfs-browser/uk-new-recheck.json
```

## 指针、胎印和驾驶舱

`final-feedback-01` 验证真实油门／急刹期间两枚 SVG 指针数值和旋转角、F5 隐藏／恢复、320 px 下 F7 镜面与触控避让。原生急刹写入真实地形上的胎印，观测到材质 1；实际 world quad 独立投影到 compositor PNG，三处可见轮迹中心比相邻路面暗约 46、48、65 个亮度级。PC 与 320 px PNG 均保留。

该项为 27ddd412 的圆表／胶印功能证据。最新窄竖屏不再挂载圆表，只显示 speed／rpm／gear 数字；当前可见模式、读数更新和紧凑视野由新布局与健康操作验收覆盖。

`final-cockpit-02` 验证五车原 steering／RPM／MPH 部件的真实输入响应、左右转向的可见 PNG 变化及 PC／320 px 视图，共 20 项检查。MCF1 没有原 MPH 针，保持原设计。几何和 UV 的原件绑定由既有 GLB 测试与浏览器部件观测共同核验。

`final-safety-01` 覆盖多指释放、丢失释放、拖出、键盘／Gamepad 多源输入、重复键、普通点击、禁选和聊天编辑边界。完整报告及清理记录保留在对应目录。

并发负载下的一张 320 px 布局图带同步暂停告警，只作为布局证据；不能作为网络稳定性或性能证据。独立健康操作图与性能数据在隔离阶段另行采集，不裁去告警冒充健康画面。

最终 UI 与联网验收使用 `4bac7f3b039b253cc9373f33d9bf65e6b359dd443aa78efab4ec07ddf2a36012`。`final-gauge-layout-04` 的 16 项检查、13 张完整视图全部通过：PC 圆表整组 260 px、右边距 16 px、底边与地图齐平；小号 `1`／`R` 为 12 px，实际圆盘、挡位底板与聊天按钮互不遮挡。320／390／430 px 竖屏及 320×480 px 矮竖屏只挂载 108×44 px 数字面板，地图 74×54 px；F7 关／开均与地图同顶。844×390／844×520 px 横屏保留 160 px 小圆表，与 HUD、按键和真实镜面分离。

地图与数字面板合计占画布约 5.04%（320×844）、4.07%（390×844）、3.67%（430×844）、6.92%（320×480），全部低于 8%，中央道路条带不被二者遮挡。数字最不利白路背景对比度最低 9.76∶1。最终健康触屏油门图无同步告警，可见读数为 57 km/h、3500 rpm、挡位 2，确认 compact 数字确实随车况更新。

## 连续发动机

五车各保留 36 秒实际浏览器混合流：怠速、W＋手刹驻车负载、W 加速、S 刹停和余下怠速。录音采样唯一原生 `HTMLAudio.srcObject` 混合音轨；该流已包含各声部 gain／pitch，试听 WAV 只乘一次实测原生 master，没有二次变速、重复声部混合或响度归一化。

五车原始声部集合分别为 5／8／3／5／6，原 FLAC SHA 全部匹配，真实 `BufferSource.start` 次数各为 1。稳定怠速和驻车负载以 2 ms RMS 统计，小于该段中位数 5% 的最长连续近静音均为 0 ms，满足事先冻结的 ≤20 ms 工程门槛。完整录音开头约 6 ms 的编码／起始区间单独保留；这些数字不冒称主观听感评价。

统一选择映射及五个可试听 WAV 位于 `.local/racing-nfs-browser/final-continuous-selected-analysis-01/`；独立复核为 `.local/racing-nfs-reference/final-continuous-review.json`。前四车来自 `final-continuous-five-01`，CCOR 来自 `final-ccor-isolated-01`，均为同一最终构建。

原 CCOR 并发录音有约 3 秒真实状态接收中断，触发既有失效保护停止原 sink，保留为断流生命周期红证据；不用于连续输出通过结论。隔离复录后完整 36 秒、原生静音／恢复与 owner 清理均通过。更早原生 FLAC／相同 PCM WAV loop 的约 126 ms 周期洞也保留，不能将换容器描述成连续修复。

## 联网与隔离性能

`final-feedback-perf-03` 与 `final-latency-04` 在全部长圈和其他浏览器验收结束后顺序隔离执行，均绑定最终 4bac 构建并通过。统计分别保存输入到像素、RTT、ACK、双向 WebSocket 应用 payload 字节数／消息数／每秒字节数和状态到达间隔，不从动态 view 的单条 8 KiB 预算推算流量。

| 场景 | 实测值 |
| --- | --- |
| 六车 PC 绘制 | 59.985 FPS，帧间隔 P95 18.1 ms |
| 六车 320 px 绘制 | 60.008 FPS，帧间隔 P95 18.4 ms |
| 正常连接，8 次原生输入到真实像素 | P95 78.11 ms，8／8 可见 |
| RTT 151 ms 慢连接，8 次原生输入到真实像素 | P95 49.40 ms，8／8 可见 |
| 正常／慢连接 ACK | P95 27／173 ms，与本地像素响应分列 |
| 正常两人应用 payload | 上行 611 B/s，下行 28.17 KB/s |
| 慢连接应用 payload | 上行 588 B/s，下行 30.43 KB/s |
| 六车托管 PC／320 px 应用 payload | 下行 58.08／57.95 KB/s；该观察窗无手动上行输入 |
| 六车状态到达间隔 | P95 101 ms |
| 健康玩家与慢连接并存 | 60.002 FPS，帧间隔 P95 18.1 ms，状态到达 P95 102 ms |

KB 在此按 1000 bytes 计算。流量直接统计 CDP 原 `payloadData` 的应用字节，不含 WebSocket 帧头／mask、TCP、TLS 或重传开销；实际 101 握手没有协商扩展。初始资源与握手另行保留，不混入稳定场景窗口。两人上行数字对应八次交替转向及回正等待的实际操作模式，不是通用上行容量上限。完整消息计数、总字节、时间窗和类型分组见公开机器摘要与 [NETWORK-AUDIT.md](NETWORK-AUDIT.md)。

慢连接期间健康玩家的权威时钟持续推进；真实 socket 中断后，epoch、原比赛期限保持，旧油门／喇叭释放且按键 repeat 不会重新启动旧输入。所有原生输入、像素 PNG 与 ACK 记录分别保留。

历史红记录没有删除：旧 27 构建一次正常输入 P95 83.64 ms 超出预算；中间 87 构建的慢连接一次输入前已有 179 个变化像素，按原噪声公式测得 142.16 ms。后续先等待真实渲染车轮回零至少 250 ms、6 次绘制，再发下一次输入；有界 5 秒、完整保留八次已发输入，像素公式与 80 ms 门槛均不变。最终数据按这一明确因果前提测得，不把等待回正当成输入响应时间，也不以重定义时钟或丢弃失败试次达标。

## 口径与保留限制

原始细节保留在 `.local`；仓库内公开机器摘要为 [driving-feedback-browser-validation.json](driving-feedback-browser-validation.json)，仅包含结果、源哈希、证据文件哈希、数值和限制，不包含服务地址、Cookie 或完整房间快照。

主要复跑入口为 `scripts/racing/check-nfs.py` 的 `gauge-layout`、`feedback-perf`、`latency`、`cockpit`、`feedback`、`safety` 和 `circuits --track <track> --circuit-plan ...`；每次指定现有仓库内 `--dist`、对应 `--build-identity` 与全新 `--out`。长圈使用每圈小摘要及完整增量 journal，避免约 5 MB 深对象经 CDP 一次返回卡住 target；完整工件可用 `recheck-circuits.py` 离线重验。中断与校验器问题的先行清单见 [BROWSER-HARNESS-INTERRUPTIONS.md](BROWSER-HARNESS-INTERRUPTIONS.md)。

320 px 为 Chromium 触屏仿真，不能替代实体手机 GPU 或 iOS 验收。Gamepad 仅模拟硬件边界；实际控制器未实测。音频为实际混合流数字采样加实测 master 包络，未声称扬声器录音或模型已经试听。Windows 原版手感和实体设备表现不由这些浏览器结果推断。
