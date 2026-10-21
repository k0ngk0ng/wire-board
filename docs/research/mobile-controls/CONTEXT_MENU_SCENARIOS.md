# 控件长按菜单：实现前失败场景

- 按钮文字不可选择，但长按仍弹出「复制」菜单；按钮内的文字、图标、SVG 未继承抑制规则。
- 自定义 role=button、方向盘、摇杆、十字方向键和蓄力操作区域漏掉处理。
- 只阻止冒泡阶段的事件，菜单处理或按住操作已被中断。
- 为禁止菜单而拦截所有页面右键、pointerdown/up、触摸取消，导致聊天不能复制或按键不能松开。
- 控制区包含文本输入、下拉框或 contenteditable，不能正常编辑和复制。
- 组件重挂载重复安装监听器，或清理遗漏影响其他界面。
- PC 正常点击、键盘操作、触屏拖动、多指与取消被新逻辑破坏。

验收使用 scripts/mobile-controls/check-browser.py --games bubbleparty，仅启动这款游戏的真实浏览器交互，不跑其他游戏规则矩阵。原生 Chromium 触摸保持 1.2 秒并移动、松手、取消、多指；在保持触摸期间显式派发可取消的 contextmenu，检查 defaultPrevented 及动作持续。另对普通按钮、图标、角色按钮、控制区、输入框、可编辑区和普通文字检查事件边界，保存 JSON、实际样式与截图。

Chrome 的触屏仿真不能证明实体 iOS 系统菜单完全消失。验收分别记录原生触屏结果、事件抑制边界，以及源样式中的 -webkit-touch-callout；最终不把这些结果声称为实体 iPhone 实测。

## 实体手机反馈后的补充

仅拦截 `contextmenu` 不能覆盖没有发出此事件的原生选字／长按行为。新增 `data-game-hold-control` 标记仅用于已通过 Pointer Events 驱动的按住型控件；泡泡堂摇杆与放泡按钮明确接入。对这些控件使用非 passive 的原生 `touchstart`／`touchmove` 防止浏览器默认手势，并按 touch identifier 跟踪拖出控件的手指；松手、取消、失焦、隐藏和卸载清理。普通点击按钮不取消 touchstart，防止 WebKit 的合成 click 被吞掉。

先补验收：控件 touchstart 需 prevented、同一手指移至普通文字需 prevented、保持期间 selectstart 不得选字、松手后恢复普通文字选择、输入框 touchstart 保持默认行为。修复前这些原生触摸默认行为均未拦截；修复后 DOM 合同和真实泡泡堂 320/390px 长按、多指、输入 ACK、聊天选择均通过。结果分别在 `.local/version-update/touch-accepted/report.json` 与 `.local/version-update/bubble-touch/report.json`。仍不把 Chromium 触控仿真声称为实体 iOS 验收。

原生触摸开始时还会清除控件以外遗留的普通文字选区，避免旧复制菜单残留；当前焦点在输入框时保留编辑选区。此场景同样先获得失败记录，再验证修复。用户反馈的主要环境是 iPhone Safari，其他浏览器也有复现。
