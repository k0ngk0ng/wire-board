# 极品飞车重制验收

本次将赛车改为车后追逐视角，提供 5 条长赛道、6 款经典车型外观、自动挡、B 回看、R 低速重置和空格手刹。模型、环境和封面由项目代码绘制；参考图片只用于研究。泡泡堂同步改为四向走格摇杆，等待房间增加原子「保存并准备」。

## 验证范围

先于实现列出的失败场景见 [PLAN.md](PLAN.md)。复杂功能使用真实 HTTP、SQLite、服务端时钟和浏览器原生输入验收；浏览器脚本不注入游戏结果。低风险纯计算逻辑使用针对性检查。

| 范围 | 结果产物（仓库本地） |
| --- | --- |
| 赛车 5 赛道 × 2～6 人 × 1／3／5 圈，75 组自然完整比赛 | `.local/racing-remake/racing-complete-brake-final.txt` |
| 赛车规则、HTTP、恢复、托管和输入安全竞态检查 | `.local/racing-remake/racing-race-final.txt` |
| 公共前端、赛车、泡泡堂共 149 项前端检查 | `.local/racing-remake/front-final.txt` |
| 真实创建、五赛道、六车型前后截图、键盘、手机、重连、观众、托管和自然结算 | `.local/racing-remake/browser-release/report.json` |
| 禁用 WebGL 后透视兼容画面及 W／B／空格／R、重连与手机操作 | `.local/racing-remake/browser-compatible/report.json` |
| 泡泡堂完整浏览器验收，含四向摇杆、多指放泡、取消、困泡、接管和全部已有地图 | `.local/racing-remake/bubble-joystick-final/result.json` |
| 非默认地图保存并准备、电脑保持准备、实际点击开局 | `.local/racing-remake/browser-final-six/report.json` |
| 准备事务权限、失败回滚、未改规则、SQLite 恢复与开局 | `.local/racing-remake/ready-final.txt` |
| 公共平台 HTTP、持久化、并发与恢复 | `.local/ci/shared/result-0.json` 及同目录日志 |
| 模块边界、Go 格式与静态检查、生产前端构建 | `.local/racing-remake/boundaries-final.txt`、`gofmt-final.txt`、`vet-final.txt`、`build-final.txt` |
| v1 音效与图集保持不变，v2 来源说明及 MIT 许可的本地／OSS 摘要 | `.local/racing-remake/assets/oss-readback.json` |

桌面和 320px 浏览器验收记录最终计算颜色，对正文要求至少 4.5:1。六席位手机首屏检查实际可见的 HUD、画面、方向盘和操作按钮，不通过测试滚动掩盖布局问题。鼠标检查滚轮锚点缩放、拖动、小地图连续定位；触屏检查双指缩放。地图关闭回到驾驶焦点，比赛自然结束时地图同步关闭，公共结算保持焦点，关闭结算后快照不重新打开。

## 重跑

赛车操作入口见 [游戏 README](../../../games/racing/README.md)。`scripts/racing/check-browser.py` 转入当前追车验收；`check-fallback.py` 转入禁用 WebGL 的兼容验收，两者均要求指定新的仓库内 `--out` 目录。

```sh
. scripts/env.sh
node --experimental-strip-types --test web/tests/*.test.mjs games/racing/tests/*.test.mjs games/bubbleparty/tests/*.test.mjs
python3 -B scripts/ci/scope.py run --game shared
python3 -B scripts/racing/verify-assets.py --remake --dist --oss
```

## 限制与来源

Chrome 的 320px 触屏仿真不代表实体手机 GPU 性能测试。画面为风格化街机赛车，物理仍为地面二维计算；显示挡位按速度派生，车型之间性能和碰撞尺寸相同。旧存档保留已保存赛道几何和期限。图形兼容验收采用禁用 WebGL，未把它冒充真实设备的显卡故障测试。

Three.js 0.180.0 的 MIT 全文和原创代码摘要见 `games/racing/assets/public/racing/v2/`。引擎、轮胎、碰撞和提示音沿用 v1 署名，全部经平台音频接口播放。Need for Speed Heat 官方商店参考图片的来源与摘要保存在 `.local/racing-remake/reference/sources.json`，未用于产品素材。

清理仅覆盖本轮过期浏览器运行、临时 profile、socket、失败及中间日志；最终结果、截图、参考证据和已发布 OSS 对象保留。具体路径记录在 `.local/racing-remake/cleanup.json`。
