import { coverSVG } from "../assets/art";
export const Cover = () => (
  <span
    className="junqi-cover"
    aria-hidden="true"
    dangerouslySetInnerHTML={{ __html: coverSVG }}
  />
);
