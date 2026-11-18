# 背包与路线显示对照

对照器从原版 v1.7.3 提取面板 DOM 绘制与输入处理，用固定状态和资源生成参考画面。原版处理函数只在本地测试夹具内运行，不加载 Main 或战斗规则。产品侧使用实际 React 组件、资源校验与独立 Go 规则。

```sh
. scripts/env.sh
node games/pokepath/tests/inventory-render-oracle.mjs
node games/pokepath/tests/routes-render-oracle.mjs
go test -race ./games/pokepath/engine -run '^TestInventory' -count=1
```

背包对照覆盖 7 种排序、前缀搜索及 `mega`、分类、伙伴筛选、5 档尺寸、鼠标位移阈值、换位、合法装备、非法目标和管理道具的动画相位。320 px 验收另用浏览器触控输入，核对实际可见性、稳定装备操作和只读保护。桌面缩放后，拖动图像维持 44 px 基础尺寸并随指针移动。搜索框保留平台键盘焦点指示。

换位请求传递两个稳定道具 ID。服务端拒绝同一、未知、未拥有或错误席位的选择；接受后只交换道具条目顺序，保留原归属、装备、生命与金币。顺序随共享存档保存。浏览器预测与服务端使用同一份 Go 规则。

路线对照使用正常、波次进行中及秘密隐藏三组状态。原版世界图的扩展路线、占位、锁定缩略图、记录颜色、编辑器入口和秘密地图使用同一份事实数据。列表按原版地区顺序每四条路线分组，缎带总数直接使用服务器投影。路线缩略图仅用于打开的路线面板；列表在可见范围附近加载，开局不预加载全部路线。

逐步收据、源码摘要、截图和未遮罩像素数存于 `.local/pokepath/verification/inventory-render/result.json` 与 `.local/pokepath/verification/routes-render/result.json`。仅 `passed=true` 表示该矩阵所有像素一致；`behaviorPassed` 与 `exactPixelCases` 分别记录行为及画面范围，不能作为整款游戏通过的证据。
