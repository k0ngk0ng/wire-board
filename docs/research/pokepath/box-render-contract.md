# 仓库与换位

仓库同时展示队伍和库存。伙伴可按队伍、名称、等级、能力、适用地形和战斗属性等排序。地形标签调暗不符合条件的伙伴，搜索才隐藏不匹配的结果。选择以房间内的持有编号为准，排序、搜索、改名和并发更新不会改变选中对象。每个玩家在同一对局里保留自己的排序和图块尺寸。

拖动库存伙伴到队伍栏时，服务器校验拖动开始时对应槽位里的伙伴。空位收下库存伙伴；已占用的位置替换旧伙伴，旧伙伴如已部署则先退场再入库。若另一席已改动目标，旧操作被拒绝，避免把新来的伙伴换走。手机可用仓库按钮触控移入或移出队伍。改名、收藏和队伍变更通过房间同步，观众可浏览仓库但不能写入。

```sh
. scripts/env.sh
npm --prefix web run build
node games/pokepath/tests/box-render-oracle.mjs
node games/pokepath/tests/native-platform-browser.mjs --standard-web --box
go test ./games/pokepath/tests -run '^TestNativePlatformBox' -count=1
```

仓库对照采用 `BoxScene.js` 的原版渲染及筛选片段。49 组场景的标签、顺序、地形效果和图块几何一致；徽记和字形边缘仍有像素差异，严格画面对照未通过。报告保留完整未遮罩的 720×624 截图和操作回执，见 `.local/pokepath/verification/box-render/result.json`。正式双人房间与观战测试见 `.local/pokepath/native-browser/run-ebfda365/`，其中记录了桌面拖放、拒绝过期目标、改名同步和 320 px 触控移入／移出。
