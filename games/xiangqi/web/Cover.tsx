import { coverSVG } from "../assets/art";
export const Cover = () => (
  <span
    className="xiangqi-cover"
    aria-hidden="true"
    dangerouslySetInnerHTML={{ __html: coverSVG }}
  />
);
