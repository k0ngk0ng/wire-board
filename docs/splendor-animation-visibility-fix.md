# 宝石动画可见性与手机创建焦点修复

2026-10-17。

## 复现与修改

普通桌面和手机视口、头像可见时，拿牌和筹码飞行仍正常。将页面滚动至操作区域，或横向滚动玩家栏，使目标头像不可见后，原逻辑会将整段飞行隐藏；修复前 390px 复现中事件已产生，但可见动画帧为 0。

宝石动画现在优先使用真实头像；头像不可见时，在动画层显示带席位、颜色和姓名的临时接收标记。取币、付款和归还沿正确方向连接到同一玩家，不把屏幕边缘当成另一玩家头像。动画层不挤动棋盘，结束即移除，初次加载或刷新不回放旧事件。

同时修复两处相关反馈：平台全局的减少动态效果规则盖过了宝石自己的静态淡出动画，导致筹码停留在来源位置；现在只保留终点处的淡出和静态收牌提示，不恢复空间飞行。购买浮层中的预留牌时，始终从刚确认的牌面发出，避免跳回仍可见的原预留栏。

创建房间在手机宽度或粗指针设备上将焦点放在当前步骤容器，不主动聚焦搜索或房间名称；电脑仍自动聚焦输入框。返回选择游戏同样不弹键盘，手动点击输入仍正常。

## 证据与复跑

测试探针在修改前补齐滚动、实际 CSS 动画及焦点检查，分别保存失败结果。没有用 DOM 元素存在代替实际运动与可见性检查。

| 检查 | 结果／产物 |
| --- | --- |
| 390px 滚动后预留、取币、归还 | `.local/splendor-animation-390-077ab59c/result.json` 通过 |
| 320px 滚动、正确玩家、刷新不重播 | `.local/splendor-animation-320-34d25281/result.json` 通过 |
| 减少动态效果、静态淡出及刷新 | `.local/splendor-animation-390-fb30acdf/result.json` 通过 |
| 桌面预留牌购买、贵族到访及自然结算 | `.local/splendor-animation-1440-34509bb9/result.json` 通过 |
| 390／320 手机与 1440 桌面的两步创建焦点、返回、手动编辑 | `.local/create-focus-c9b83730/result.json` 通过；修复前 `.local/create-focus-afc7ea33/result.json` 显示手机各步骤均聚焦输入框 |
| 范围内前端检查 | 宝石 15 项、公共前端 65 项通过；生产构建通过 |

使用独立本地数据目录，启用宝石后可执行：

```sh
source scripts/env.sh
python3 -B scripts/accept_splendor_animation.py --width 320 --scroll-board
python3 -B scripts/accept_splendor_animation.py --width 390 --scroll-board --reduced-motion
python3 -B scripts/accept_splendor_animation.py --width 1440 --acquisition
python3 -B scripts/accept_create_focus.py
```

焦点脚本要求本地测试邀请码 `page-accept-code`；宝石脚本也可从环境变量读取邀请码。结果包含逐帧几何、实际动画名称／变换、玩家身份及截图。未修改游戏规则，未运行其他游戏的规则或完整对局。
