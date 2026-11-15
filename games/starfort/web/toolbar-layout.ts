export function toolbarLayout(width: number, hud: number, items: string[]) {
  const slots = Math.max(1, Math.floor((width - hud) / 31));
  if (items.length <= slots) return { inline: [...items], overflow: [] as string[] };
  const priority = (key: string) => ({ ready: 0, resume: 0, build: 1, tasks: 2, info: 3 }[key] ?? 4);
  const inline = [...items].sort((a, b) => priority(a) - priority(b)).slice(0, slots - 1);
  return { inline, overflow: items.filter((item) => !inline.includes(item)) };
}
