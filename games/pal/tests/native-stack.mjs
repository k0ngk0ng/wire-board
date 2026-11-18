// The native save contract includes the suspended C stack pointer. Observation
// exports use an independent C service stack and restore the suspended pointer.
const services = new Set([
  "wb_input",
  "wb_budget",
  "wb_release",
  "wb_view",
  "wb_scene_ops",
  "wb_scene_state",
  "wb_field_info",
  "wb_probe_field_config",
  "wb_probe_field_move",
  "wb_palette_state",
  "wb_palette_red_step",
  "wb_game_over_info",
  "wb_ui_ops",
  "wb_dialog_info",
  "wb_menu_info",
  "wb_battle_info",
  "wb_coop_info",
  "wb_raster_graph",
  "wb_raster_seed_info",
  "wb_platform_players",
]);
export function protectedExports(raw) {
  const exports = { ...raw };
  for (const name of services) {
    if (typeof raw[name] !== "function") continue;
    exports[name] = (...args) => {
      const previous = raw.__stack_pointer.value;
      raw.__stack_pointer.value = raw.wb_service_stack.value + 262144;
      try {
        return raw[name](...args);
      } finally {
        raw.__stack_pointer.value = previous;
      }
    };
  }
  return exports;
}
