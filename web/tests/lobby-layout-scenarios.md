# 大厅视觉改版验收

先验证：搜索、分类、游戏与状态筛选仍能组合；空结果可清除；创建弹窗完整展示上架游戏、封面保持正确归属、单次点击只创建一个房间；桌面六列封面卡片与 320 px 双列布局；等待房间封面比例正确、设置、席位、电脑、准备和开局操作完整；顶部工具仍可操作；页面和弹窗不横向溢出，普通文字对比度至少 4.5:1，触屏目标至少 44 px。

大厅参考中的 32 张封面与本次已发布图片逐字节一致，直接复用现有 OSS 对象。封面逻辑目录独立于游戏客户端运行包，尤其不得混入魂斗罗的精确运行包清单。

实际 HTTP、SQLite、WebSocket、桌面与 320 px 浏览器入口：`ROOM_FLOW_REPORT=lobby-redesign node --test web/tests/room-flow-browser.test.mjs`。结果保存在 `.local/room-flow/verification/lobby-redesign/`，包括截图、检查记录和实际组件摘要。
