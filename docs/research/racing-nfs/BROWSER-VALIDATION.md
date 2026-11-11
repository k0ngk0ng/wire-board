# 浏览器验收记录

当前构建的全部浏览器门槛已按阶段聚合通过。正式 HTTP、认证、SQLite、计时器和 Three.js 页面参与验收；没有写入比赛快照、加速时钟或伪造完赛。

构建 `web/dist/index.html` SHA-256：`bd452b9be413080ed84b5fdc3866216342c54e281d65a1a712e34a14ac19d7b1`。五份采用的运行报告内，赛车 `web/**`、`server/**` 共 61 个已记录文件逐项摘要一致，并与汇总时的文件一致；归并摘要为 `86507088e55bef7e98ac5751a769cc98eca2e9862dc4469d2725eccc12f69708`。`controls.json` 和 v3 资源清单的当前摘要另列于 [机器记录](browser-validation.json)。本报告只声明赛车产品源码与前端构建一致，不声明所有平台源码均受浏览器摘要覆盖。

## 主要测量

| 六车移动场景 | 实际绘制 FPS | 帧间隔 P95 | 动态 JSON 最大值 |
| --- | ---: | ---: | ---: |
| PC，镜子关 | 60.00 | 18.30 ms | 5202 bytes |
| PC，镜子开 | 59.99 | 18.50 ms | 5224 bytes |
| 320 px 触屏，镜子开 | 60.02 | 18.50 ms | 5211 bytes |
| 320 px 触屏，镜子关 | 60.01 | 18.20 ms | 5213 bytes |

PC 门槛为 ≥50 FPS、P95 ≤35 ms；触屏仿真为 ≥30 FPS、P95 ≤50 ms。每项连续采样约 6 秒，以实际递增的绘制计数统计；镜子包含第二摄像机及合成开销。全部动态消息低于 8192 bytes，采样中没有重复静态道路。

| 原生转向到实际本车像素 | 可见试验 | 像素 P95 | 输入 ACK P95 | 实测连接 RTT |
| --- | ---: | ---: | ---: | ---: |
| 正常连接 | 8/8 | 62.87 ms | 28 ms | 0 ms |
| 单端 150 ms TCP 延迟 | 8/8 | 51.61 ms | 185 ms | 153 ms |

像素门槛为 P95 ≤80 ms。测量使用原生按键时间和 compositor PNG，裁剪框来自渲染器投影；先采样无输入噪声，再定位首次可见变化。ACK 单列，不替代像素延迟。原始 PNG 与摘要清单保留在对应产物目录。

六车一圈在真实发车后 191.72 秒自然结束，六车均完成 1 圈，`lastReset` 全为 0，未触及绝对超时。

## 门槛与来源

| 门槛 | 采用的报告 |
| --- | --- |
| 桌面移除底面板、320 px 画面内透明触控 | [full-final-02](../../../.local/racing-nfs-browser/full-final-02/report.json) |
| C 四视角及 B 恢复、F3/F5/F7/F11/F12 边沿 | [full-final-02](../../../.local/racing-nfs-browser/full-final-02/report.json) |
| H 原生 FLAC 保持、释放和失焦 | [full-final-02](../../../.local/racing-nfs-browser/full-final-02/report.json) |
| 标准 Gamepad API 仿真、公开改键、旧默认禁用 | [full-final-02](../../../.local/racing-nfs-browser/full-final-02/report.json) |
| 键盘、权威 R、真实多指、地图、刷新 | [remaining-final-02](../../../.local/racing-nfs-browser/remaining-final-02/report.json) |
| 四条独立赛道、各昼夜、几何/贴图/字节/按需加载 | [remaining-final-02](../../../.local/racing-nfs-browser/remaining-final-02/report.json) |
| 按钮失焦重复输入、托管/独立观战呈现与发动机 | [experience-final-02](../../../.local/racing-nfs-browser/experience-final-02/report.json) |
| 五车真实驾驶舱与 B 恢复 | [vehicles-final-01](../../../.local/racing-nfs-browser/vehicles-final-01/report.json) |
| P911/FM50 原声 SHA、解码/时钟/速率/升挡、静音/失焦/清理 | [vehicles-final-01](../../../.local/racing-nfs-browser/vehicles-final-01/report.json) |
| 六车 PC/320 px、镜子关/开帧率与 8 KiB 动态消息 | [vehicles-final-01](../../../.local/racing-nfs-browser/vehicles-final-01/report.json) |
| 真人托管/接管及真实时钟六车一圈自然完赛 | [vehicles-final-01](../../../.local/racing-nfs-browser/vehicles-final-01/report.json) |
| 正常/150 ms 真实 TCP 延迟的原生输入到车辆像素 | [vehicles-final-01](../../../.local/racing-nfs-browser/vehicles-final-01/report.json) |
| 真实断线与 H 无心跳 1250 ms 过期 | [vehicles-final-01](../../../.local/racing-nfs-browser/vehicles-final-01/report.json) |
| PC/320 px 公共原生全屏、地图内置、ESC/按钮无残留退出 | [fullscreen-final-01](../../../.local/racing-nfs-browser/fullscreen-final-01/report.json) |

四条赛道各验证昼夜场景、精确几何引用、非零原纹理解码、路面高程、真实字节进度及按需加载。五车均保留真实驾驶舱截图；P911、FM50 另验证原始 FLAC 字节 SHA、原生播放推进、16 秒负载/转速变化、升挡降转对应速率下降，以及静音、失焦和离场清理。喇叭为明确标记的自制 FLAC。

托管和独立观战期间可切视角、后视及显示选项，未发送驾驶输入，发动机持续播放。公共全屏入口在 PC/320 均进入真实游戏区原生全屏，地图浮层留在游戏区内；ESC/按钮退出后无新增 inert/overflow 残留，画布未重挂载，比赛 epoch 和期限不变。

采用报告的整体状态分别保留原值。前三份运行虽随后因工具问题停止，只采用表中已明确 PASS 的检查；[机器记录](browser-validation.json) 列出每个门槛的报告、checkIndex、来源 SHA 及摘要相等校验。最终车辆/网络阶段与全屏阶段各自整体通过。

## 工具修正与限制

- CDP 硬冻结仅放在最后的独立过期门槛；后续不再从隐藏的渲染器测输入或帧率。
- 手机布局、截图与触摸在 CDP coarse 仿真连接存活时采集；等待目标真实可见并核对 `elementFromPoint` 命中。
- R 前按住真实 S，直到权威状态允许复位；真实 2 秒复位结束后测触摸。持油门 900 ms 只用于确认起步，不作为像素延迟。
- 公开改键选择器保留中文，保存按钮先滚入视口；新房间键盘输入前聚焦 Canvas。
- 会话前缀缩短，保持 UNIX socket 路径在限制内。
- 失败运行只复用明确 PASS 的检查；逐项绑定相同构建和已记录的赛车 web/server 源码摘要，保留原运行的 `passed: false`。

- 320 px Chromium 触屏仿真不等同实体手机 GPU 测量。
- Gamepad 只替换 `navigator.getGamepads` 硬件入口；生产映射、采样和 HTTP 真实执行，未实测实体 USB 或蓝牙手柄。
- Chrome 以 `--mute-audio` 启动；验证原生 HTMLAudio 解码、`currentTime`、`rate`、`volume` 和可信事件，未作扬声器听音。
- 使用双向有序 TCP 延迟与真实 socket 中断，未声称测试真实 IP 丢包。
- 浏览器验收不证明与 Windows 原版操控逐项等价；本轮未取得原版运行手感或音频输出对照测量。
- 本轮 PC 及 320 px 全屏均走 native 路径；未强制测试网页全屏回退。
- 多阶段聚合通过；没有将任何失败运行整体改记为通过。

全部采用的 fixture、浏览器及本次 `.s/nfs*` 临时 socket 目录已清理。保留可核验的报告、截图、真实事件、音频及像素证据；未删除其他游戏或原有 `.s/cbb0a62d`。

共享控制器旧用户配置迁移由单列真实 HTTP/SQLite E2E 验证。该迁移不在赛车源码摘要范围内，也未计入本浏览器报告的通过断言。

## 重跑

从仓库根目录运行，输出目录必须为仓库内未存在的专用目录：

```sh
python3 -B scripts/racing/check-nfs.py --out .local/racing-nfs-browser/<new-full-name> --phase full
python3 -B scripts/racing/check-nfs.py --out .local/racing-nfs-browser/<new-fullscreen-name> --phase fullscreen
```

`full` 覆盖驾驶、资源、音频、比赛和网络门槛；平台全屏使用独立短阶段。不得在验收期间重建或修改产品构建。
