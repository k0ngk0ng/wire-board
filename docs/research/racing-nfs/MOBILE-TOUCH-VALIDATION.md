# 手机按键布局验收

本轮按用户反馈移除屏幕方向盘，采用左侧双箭头、右侧大尺寸刹车／油门。手刹为右下边缘的 44×44px 次要按钮；全屏时为平台聊天按钮预留空间。参考依据见 [实现前计划](MOBILE-TOUCH-PLAN.md)。

当前构建通过 86 项赛车前端检查、TypeScript／Vite 构建，以及三个真实认证房间 E2E 阶段。三个阶段记录的赛车源码和前端构建摘要逐项一致，见 [机器记录](mobile-touch-validation.json)。赛车服务端规则及原始模型、贴图、音频没有变化。

- 320px 主控件至少 56×64px，无方向盘；左右转向在左，制动／油门在右，手刹位于常用操作下方的右边缘。
- 原生双指转向加速、跨键滑动换向、刹车优先、键盘和触屏合并、取消／失焦释放均通过。
- 旋转时释放旧触摸，新的触摸可以恢复；按钮聚焦后失焦，按键 repeat 不能恢复旧操作。
- 横屏原生全屏的操作可见、互不重叠，聊天按钮不遮挡踏板；桌面与 320px 公共全屏内可以驾驶、打开地图，退出后无惯性输入、inert 或滚动残留。
- C/B/H、F3/F5/F7/F11/F12、Gamepad API 仿真、公开改键和旧默认禁用、地图缩放、刷新身份／期限及离场清理通过。

| 原生触摸到车辆像素 | 试验 | P95 | 门槛 |
| --- | ---: | ---: | ---: |
| 正常连接 | 8/8 | 52.93ms | ≤80ms |
| 150ms TCP 延迟 | 8/8 | 54.17ms | ≤80ms |

触摸由 CDP 原生事件派发，时间取可信 pointerdown 事件；响应取渲染器投影的真实车辆范围内的 compositor PNG 像素变化，先校准无输入噪声。控件高亮和 ACK 不替代车辆响应。仿真不能视为实体手机的人体工学实测。

产物位于 `.local/racing-mobile-research/thumbs-release/`、`controls-release/` 和 `fullscreen-release/`。实现前分别保存移除方向盘、旋转释放、聊天遮挡及手刹靠边的失败证据。以下命令从仓库根目录重跑，输出目录须不存在：

```sh
. scripts/env.sh
npm --prefix web run build
npm --prefix web test -- racing
python3 -B scripts/racing/check-nfs.py --out .local/racing-mobile-research/new-thumbs --phase thumbs
python3 -B scripts/racing/check-nfs.py --out .local/racing-mobile-research/new-controls --phase controls
python3 -B scripts/racing/check-nfs.py --out .local/racing-mobile-research/new-fullscreen --phase fullscreen
```
