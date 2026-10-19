import { coverSVG } from "../assets/art";

export function Cover() {
  return (
    <div
      className="fortressblue-cover"
      dangerouslySetInnerHTML={{ __html: coverSVG }}
    />
  );
}
