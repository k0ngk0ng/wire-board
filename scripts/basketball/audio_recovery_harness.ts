import { GameAudio } from "../../web/src/audio";
import {
  BasketballAudio,
  type BasketballMusicState,
} from "../../games/basketball/web/audio";
import delivery from "../../games/basketball/assets/audio-playback.json";

const audio = new GameAudio();
audio.mount();
const adapter = new BasketballAudio(audio.channel("recovery"), {
  localBase: "/audio",
});
const data = await (await fetch("/host-music-view.json")).json();
const h = {
  audio,
  adapter,
  data,
  delivery,
  sequence: data.soundSequence,
  started: false,
  played: [] as number[],
  result: undefined as unknown,
};
(window as any).audioRecovery = h;
document.querySelector("button")!.onclick = async () => {
  adapter.reset(h.sequence);
  h.result = await adapter.restoreMusic(data.music, data.music.at);
  h.started = true;
};
(window as any).restoreRecovery = async (state: BasketballMusicState) => {
  adapter.reset(++h.sequence);
  return adapter.restoreMusic(
    { ...state, controlSequence: h.sequence },
    state.at,
  );
};
(window as any).recoveryCommand = async (ids: number[]) =>
  adapter.consume(ids, ++h.sequence);
