// Failure scenarios before implementation: absent/corrupt/out-of-range volume,
// blocked localStorage, malformed mute value, and a valid zero being lost.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readAudioPreferences } from "../src/audio-preferences.ts";
const storage = (data) => ({ getItem: (key) => data[key] ?? null });
test("new browsers start at half volume and keep the existing mute preference", () => {
  assert.deepEqual(readAudioPreferences(storage({})), {
    enabled: true,
    volume: 0.5,
  });
  assert.deepEqual(
    readAudioPreferences(storage({ wb_sound: "off", wb_volume: ".8" })),
    { enabled: false, volume: 0.8 },
  );
  assert.deepEqual(readAudioPreferences(storage({ wb_volume: "0" })), {
    enabled: true,
    volume: 0,
  });
});
test("unavailable or invalid storage never breaks sound controls", () => {
  for (const value of ["", "bad", "NaN", "Infinity", "-1", "1.1"])
    assert.equal(
      readAudioPreferences(storage({ wb_volume: value })).volume,
      0.5,
    );
  assert.deepEqual(
    readAudioPreferences({
      getItem() {
        throw Error("blocked");
      },
    }),
    { enabled: true, volume: 0.5 },
  );
});
