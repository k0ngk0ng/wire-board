import { useId, type SVGProps } from "react";

// A small, outlined gold crown: velvet lining, rounded finials and a jewelled
// band remain distinct at map-marker size. Each SVG owns its gradient IDs.
export function Crown(props: SVGProps<SVGSVGElement>) {
  const gold = `cq-crown-${useId().replaceAll(":", "")}`;
  return (
    <svg viewBox="0 0 30 26" fill="none" aria-hidden="true" {...props}>
      <defs>
        <linearGradient
          id={gold}
          x1="3"
          y1="3"
          x2="26"
          y2="24"
          gradientUnits="userSpaceOnUse"
        >
          <stop stopColor="#fff1ad" />
          <stop offset=".38" stopColor="#eac55d" />
          <stop offset=".7" stopColor="#c38a2d" />
          <stop offset="1" stopColor="#f7dc85" />
        </linearGradient>
      </defs>
      <path
        d="M6 18C5 10 9 6 15 6s10 4 9 12Z"
        fill="#a6383e"
        stroke="#57391c"
        strokeWidth="1.2"
      />
      <path
        d="m3 8 6 5 1-7 5 6 5-6 1 7 6-5-3 12H6Z"
        fill={`url(#${gold})`}
        stroke="#513c1b"
        strokeWidth="1.3"
        strokeLinejoin="round"
      />
      <path d="M15 3v12M10 7l2 9m8-9-2 9" stroke="#fff0a4" strokeWidth="1.4" />
      <g fill={`url(#${gold})`} stroke="#513c1b" strokeWidth="1">
        <circle cx="3" cy="7" r="1.7" />
        <circle cx="10" cy="5.5" r="1.6" />
        <circle cx="15" cy="3.2" r="2" />
        <circle cx="20" cy="5.5" r="1.6" />
        <circle cx="27" cy="7" r="1.7" />
        <path d="M6 18q9-2 18 0v5q-9 2-18 0Z" />
      </g>
      <path
        d="M7 19q8-1.6 16 0M7 22.4q8 1.6 16 0"
        stroke="#fff1af"
        strokeWidth=".9"
      />
      <path
        d="m15 18.2 2 2.2-2 2.2-2-2.2Z"
        fill="#a52c3b"
        stroke="#653a23"
        strokeWidth=".7"
      />
      <ellipse
        cx="9"
        cy="20.3"
        rx="1.1"
        ry="1.3"
        fill="#286665"
        stroke="#674821"
        strokeWidth=".6"
      />
      <ellipse
        cx="21"
        cy="20.3"
        rx="1.1"
        ry="1.3"
        fill="#286665"
        stroke="#674821"
        strokeWidth=".6"
      />
    </svg>
  );
}
