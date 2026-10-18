import { coverSVG } from "../assets/art";

export function Cover() {
  return (
    <span
      className="tetris-cover"
      aria-hidden="true"
      dangerouslySetInnerHTML={{ __html: coverSVG }}
    />
  );
}
