import { coverSVG } from "../assets/art";

export function Cover() {
  return (
    <span
      className="spf2t-cover"
      aria-hidden="true"
      dangerouslySetInnerHTML={{ __html: coverSVG }}
    />
  );
}
