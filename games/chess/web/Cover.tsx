import { coverSVG } from "../assets/art";
export const Cover = () => (
  <span
    className="chess-cover"
    aria-hidden="true"
    dangerouslySetInnerHTML={{ __html: coverSVG }}
  />
);
