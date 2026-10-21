# 经典地图战场地形接入

`games/bubbleparty/web/classic-terrain.ts` 独立处理原图渲染，不修改规则、人物、道具或音效。当前 `canvas.ts` 的 `CELL` 是 24，源贴图格宽是 40；所有 API 的 `cellSize` 可传入当前 `CELL`，避免未来调整尺寸后不一致。

## 加载和清理

```ts
const [classicImages, setClassicImages] = useState<ClassicImages>({});
useEffect(() => {
  const controller = new AbortController();
  setClassicImages({});
  void loadClassicSprites(data.mapId, assets, controller.signal).then(result => {
    if (!controller.signal.aborted) setClassicImages(result.images);
  });
  return () => controller.abort();
}, [data.mapId, assets]);
```

只加载当前地图实际引用的 Default 资源，不一次载入 366 张。键为 `/bubbleparty/v4/classic/sprites/<file>`，首先使用公共 `assets` 前缀，失败后转同源。每次调用返回独立字典；没有全局当前地图、动画计时器或音频播放器。切图和卸载中止旧请求并清空其绘制数据。

## 战场绘制

在 `drawBoard` 的现有 translate 后调用：

```ts
if (!drawClassicGround(ctx, view, view.mapId, classicImages, now, {
  cellSize: CELL, reduced,
})) ground(ctx, view);
```

保留道具和水柱图层。在现有 `objects` 排序队列收集阶段，经典地图改为收集原生对象，旧地图仍保留原障碍逻辑：

```ts
if (classicTerrain(view.mapId)) {
  const players = view.players.filter(p => p.alive).map(p =>
    display.get(p.seat) ?? { x: p.px ?? p.x, y: p.py ?? p.y });
  for (const object of classicObjectEntries(view, view.mapId)) {
    objects.push({
      y: object.depth,
      draw: () => drawClassicObject(
        ctx, object.id, object.x, object.y, classicImages, now,
        { cellSize: CELL, reduced,
          opacity: classicObjectOpacity(object.id, object.x, object.y, players) },
      ),
    });
  }
} else {
  // existing tileCode/obstacle collection
}
```

后续水泡和人物仍加入同一队列并一次排序，不另画到最上层。普通障碍深度为 `y + 0.4`，可穿过地物为 `y + 0.85`，与现有角色 `y + 0.6` 的脚底排序一致；可穿过地物与角色重叠时透明度为 0.5，避免草丛、帐篷完全盖住人物。每次绘制 save/restore，不影响后续图层。

`nativeObjects?: number[]` 是服务端 15 × 13 投影。只要该字段存在，就完全覆盖静态对象矩阵；`-1` 或缺失下标均为空，炸毁和推移不得重新读取初始位置。只有字段整体不存在时才使用资源矩阵，适用于静态预览。`Village00` 自动居中到 `(1,1)`，外圈以不可走边界显示；其他地图不偏移。

## 原始几何与限制

源帧矩形为 `[left, top, right, bottom)`。原生左上角为 `(gridX × 40 − frame[4], gridY × 40 + 39 − frame[5])`，之后统一按 `cellSize / 40` 缩放；不镜像、不按一格强行拉伸高树、大房子或多格物件。

动画保留 Default 的原始帧序和相对时长。第二列以每单位 30ms 驱动属于重构参数，依据已核验逻辑步长，但尚无原作渲染器时序实测证明；减少动态效果固定首帧。未知或失效贴图显示与箱子、植被、固定物体对应的可辨识回退，不能让碰撞地物变成看不见的障碍。

## 针对性验证

```sh
node --experimental-strip-types --test games/bubbleparty/tests/classic-terrain.test.mjs
```

失败场景在实现前写入测试。覆盖全部 112 地图尺寸和资源范围、原生裁切和锚点、动态对象权威性、草丛遮挡与透明度恢复、媒体回退及切图中止。实际玩家移动、推箱和战场布局仍由主任务真实对局 E2E 验收。
