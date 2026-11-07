import previews from "../assets/arena-previews.png";
import { arenas } from "./labels";
import { ImagePicker } from "./CharacterPicker";
import "./network.css";

function ArenaImage({ value }: { value: number }) {
  return <span className="yuyu-arena-image" aria-hidden="true">
    <img src={previews} alt="" draggable={false} style={{ left: `${-(value / 2) * 100}%` }} />
  </span>;
}

export function ArenaPicker({ value, disabled, onChange }: {
  value: number;
  disabled: boolean;
  onChange: (value: number) => void;
}) {
  return <div className="yuyu-arena-field" data-arena-field>
    <span>场地</span>
    <ImagePicker value={value} label="场地" kind="arena" disabled={disabled} onChange={onChange}
      choices={arenas.map(arena => ({ value: arena.id, name: arena.name }))}
      renderImage={arena => <ArenaImage value={arena} />} />
  </div>;
}
