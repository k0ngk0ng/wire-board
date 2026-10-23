# 热血篮球故障恢复与房间布局验收

线上机场球场存档停在前半场 0:52、57:3。服务器及客户端都在持篮筐动作 `0x3A` 报错；客户端不断重新获取同一份检查点并重置音乐，造成 `sync` 与 MP3 请求循环。保存的故障状态和原版指令对照用于本次修复，未通过重置对局、修改得分或替换电脑策略绕过错误。

## 修复范围

- 恢复持筐瞄准、投掷、走动、刹车、抛筐及直接衔接的动作，保留原版动画、物理与写入顺序。原版对照的范围、数量和限制见 [空中及地面动作](../games/basketball/HOOP_AIR_THROW_VERIFICATION.md) 与 [共享动作](../games/basketball/HOOP_SHARED_VERIFICATION.md)。
- 确定性的引擎错误停止自动恢复并保留画面；同一权威状态的其他错误最多自动获取一次检查点。权威状态真正推进后可恢复，显式重试仍可用。已停止的运行时须刷新页面，不能反复重建同一失效实例。
- 故障期间暂停已有媒体元素，不反复停止并重新加载音乐。提示区分游戏运行异常与连接中断。
- PC 创建房间的游戏选择弹窗随视窗扩大，第二步表单保持易读宽度；手机保留原布局。
- 手机等待房间和玩家栏约束长名称的宽度，视觉省略仍保留完整文本和提示。电脑的准备状态使用显式样式，不再因新增移除按钮而失效。

## 本地验收结果

| 验收 | 证据 |
| --- | --- |
| 原故障继续整场、确认中场与结算 | 两条物理输入路径均完成，每批恢复重演分别 499／482 次，完整状态一致 |
| 真实 HTTP／WebSocket 与 SQLite 恢复 | 跨过原失败帧、持续接收增量帧、操作获确认；重启保持对局并清除按住的输入 |
| 原事故、300 ms RTT、真实媒体 | 26 项浏览器检查通过，真实 `GameAudio.channel` 与 `HTMLAudioElement` 播放已校验的音乐 060；故障窗口无重复检查点或媒体重置 |
| 客户端与共享组件 | 篮球前端 53 项检查、3 项公共准备／暂停组件检查、TypeScript 与正式构建通过 |
| PC 创建弹窗 | 1280×900：1216px 宽、6 列、24 张完整卡片；1920×1080：1400px 宽、7 列、25 张完整卡片。原布局两种尺寸均仅显示 6 张 |
| 手机与窗口变化 | 13 项创建弹窗检查通过，320px 保持两列；短视窗关闭按钮、末尾卡片、搜索、返回与草稿可用 |
| 手机电脑名称 | 54 项检查通过。320px 长名等待卡片保持 117×155，旧版玩家卡保持 167×54；现金 1500 与 0 实际可见 |

浏览器使用生产 React 组件、原生鼠标／键盘／触摸和真实 Go/Wasm。外层测试页面、账号及提交入口使用夹具；错误处理用例在测试桥注入错误码，原故障状态的规则恢复另由真实引擎与主机验证。媒体来自不可变素材清单的校验字节，通过本地镜像提供给真实播放器。桌面 Chromium 的窄屏与延迟模拟不等同于所有实体手机验证。

旧版玩家栏原有的手机横向滚动布局保留；本次验证长名字不挤掉当前卡片的分数，且可逐席滚动查看，并未把它声明为所有席位同时可见的布局验收。

## 复现

从仓库根目录执行，使用已有 Go、Node、前端依赖和 agent-browser：

```sh
source scripts/env.sh
export AGENT_BROWSER_SOCKET_DIR="$PWD/.local/cs"
go test ./games/basketball/server -run '^TestBasketballHoopIncidentStreamRecovery$' -count=1
python3 -B scripts/basketball/verify_hoop_incident_continuation.py
python3 -B scripts/basketball/verify_hoop_incident_continuation.py --controls
BASKETBALL_INCIDENT_E2E=1 BASKETBALL_MEDIA_E2E=1 BASKETBALL_INPUT_SYNC_E2E=1 BASKETBALL_BROWSER_RTT_MS=300 BASKETBALL_BROWSER_E2E=1 go test ./games/basketball/server -run '^TestBasketballBrowserNetworkBoard$' -count=1
python3 -B scripts/accept_create_size.py
python3 -B scripts/accept_bot_layout.py
```

结果、截图与测量分别保存在 `.local/basketball-incident-20261023/`、`.local/basketball/input-sync-rtt300-incident-media/`、`.local/create-size/verified/` 和 `.local/bot-layout/verified/`。版本发布仍须复用同一提交在 main 验证成功的构建；tag 不重新编译或重复规则验收。
