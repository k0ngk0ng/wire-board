# 选车彩色显示验收

实现前定义的失败场景：

- 14 辆车中部分使用像素已烘焙为蓝色的旧大厅头像；正常可选状态看起来像未解锁。
- 已选／未选卡片、移动端或加载回退重新使用暗蓝图片。
- 图集取错帧、额外镜像或重着色，与战场坦克不一致。
- CDN 或 v3 图集失败导致空白；所有图片失败后失去可辨识的入口。
- 320px 宽度卡片溢出、后排无法滚动并点选、编号角标或英文标题重复出现；原生透明留白导致个别坦克预览过小。

复现命令（先完成前端构建）：

```sh
. scripts/env.sh
python3 -B scripts/fortressblue/check-refactor.py --only portraits --out .local/fortressblue/picker-color-acceptance
```

使用生产二进制、真实登录、准备、开赛和选车请求。覆盖桌面、320px、CDN 转同源、v3 失败转 v2 彩色 idle、全部媒体失败。记录 14 张卡片的状态、尺寸和原生像素颜色；当前画布颜色须来自相应原始帧调色集合，不能只检查 URL 或 DOM 数量；每个预览的不透明区域宽、高均须至少 30px。每个场景保存首排和选中第 14 辆车的截图。

媒体保持不可变。选择画面直接绘制 `animations.json` 的 idle 右向首帧（原始坡度中间帧），不镜像。该静态预览独立于战场动作播放，不增加动画或定时器。

本次通过记录：`.local/fortressblue/picker-color-final/report.json`，包含四类场景的 8 张桌面／手机截图、14 辆坦克的原始调色集合核对、真实点击选中和可见尺寸。`picker-verification-summary.json` 保存回归复现摘要及清理范围；预览尺寸与颜色的失败中间目录已删除。原有前端 23 项针对性测试通过，`prepare-picker-frames.py --check` 通过。没有改动服务端规则或已发布媒体。
