# 音乐与 primary 分派初始化的先行失败场景

- dispatcher 分类、调用者成本或返回层级错误；音乐应完成初始化后结束当前 driver 拍，primary 应继续推进本拍声部。
- 只验证空 score，遗漏替换现有音乐、primary 覆盖音乐，以及切换音乐前停止活动 primary 的路径。
- stop-primary 的 priority clear、FM key-off、forced-rest 恢复、patch／pan 写入或 PSG noise 恢复顺序错误；将 stop-primary 与普通 track-end 的不同分支成本混用。
- music reset 的 priority 保存／恢复、header clear、音乐声部 clear 和 selected reset 错序；清除 primary 声部或丢失实际 Root／World owner。
- 根据猜测的音轨数量、固定歌曲或重复资源状态计时；header work 必须来自实际不可变 catalog，按原顺序初始化 DAC／FM／PSG 及 primary 声部。
- primary claim、PSG3 的双 mute、音乐末尾 key-off 扫描或 secondary claim 被遗漏、重复或映射到错误 slot。
- 参数 header、清零循环、固定 DBF、数组寻址和尾部 return 未逐个保留 refresh 边界；主时钟与预算在子阶段重新开始。
- canonical 规则和计时层分别修改同一 voice／Root 字段；effect lease 遗漏、重复、过期或异常仍提交。
- 初始化后的 score／Root／queue 与已验证规则接口不同，或实际 mixed IO 与原版的值、顺序、时刻和最终时钟不同。
- 回调错误或全局预算耗尽后提交 score；复用返回的诊断记录再次播放。
- 当前 scope 扩大为 01／05／04、活动 fade／pause、world IRQ、请求生产时刻或平台播放已完成；这些边界继续显式待办。

失败计划先于输入构造、原版采样、E2E 与生产实现。本轮扩展同一个 request／running 执行器，旧闭包接口继续遵守已冻结的支持范围；不新增原 CPU、ROM 执行器或持久化状态副本。
