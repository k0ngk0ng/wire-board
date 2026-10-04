export function catanBaseLayoutName(layout: string) {
  return { fixed: "固定新手布局", variable: "随机布局" }[layout] || "";
}
