import { useId } from "react";

// The same illustrated cover is used by the lobby, game picker and waiting room.
export function Cover() {
  const id = useId();
  return (
    <svg
      viewBox="0 0 520 260"
      preserveAspectRatio="xMidYMid slice"
      className="mono-cover-art"
      aria-hidden="true"
    >
      <defs>
        <pattern id={id} width="8" height="8" patternUnits="userSpaceOnUse">
          <circle cx="2" cy="2" r=".5" fill="#35584a" opacity=".08" />
        </pattern>
      </defs>
      <rect width="520" height="260" fill="#d9e2cd" />
      <rect width="520" height="260" fill={`url(#${id})`} />
      <circle cx="445" cy="46" r="32" fill="#edd79b" />
      <path d="M0 197Q106 166 252 210T520 203V260H0Z" fill="#b6c6aa" />
      <g
        transform="matrix(.83,-.18,.50,.46,88,102)"
        stroke="#3d6252"
        strokeWidth="2.5"
        strokeLinejoin="round"
      >
        <rect
          x="4"
          y="12"
          width="280"
          height="280"
          rx="4"
          fill="#5c7b62"
          opacity=".24"
        />
        <rect width="280" height="280" rx="3" fill="#f8edcf" />
        <rect x="32" y="32" width="216" height="216" fill="#c4d6b1" />
        {Array.from({ length: 9 }, (_, i) => {
          const x = 32 + i * 24;
          const c = [
            "#ae6d4b",
            "#c7ad78",
            "#9dbfc5",
            "#9dbfc5",
            "#be7691",
            "#c7ad78",
            "#bf794e",
            "#ccad58",
            "#749674",
          ][i];
          return (
            <g key={i}>
              <path
                d={`M${x} 0V32M${x} 248V280M0 ${x}H32M248 ${x}H280`}
                fill="none"
              />
              <path
                d={`M${x + 1} 1h22v9h-22ZM${x + 1} 249h22v9h-22Z`}
                fill={c}
                stroke="none"
              />
              <path
                d={`M1 ${x + 1}h9v22H1ZM249 ${x + 1}h9v22h-9Z`}
                fill={c}
                stroke="none"
              />
            </g>
          );
        })}
        <path d="M55 208H220M70 72H213" stroke="#759c78" strokeWidth="2" />
        <rect
          x="95"
          y="92"
          width="88"
          height="43"
          rx="4"
          fill="#f5e7b6"
          transform="rotate(-12 139 114)"
        />
        <path d="M116 112h47m-35-9h32m-31 18h17" stroke="#aa8853" />
        <circle
          cx="18"
          cy="262"
          r="11"
          fill="#b7604d"
          stroke="#fff3d0"
          strokeWidth="3"
        />
        <circle
          cx="265"
          cy="111"
          r="11"
          fill="#497e93"
          stroke="#fff3d0"
          strokeWidth="3"
        />
      </g>
      <g stroke="#f5ecd2" strokeWidth="2" strokeLinejoin="round">
        <path d="M229 133v-31l23-16 23 16v31Z" fill="#729566" />
        <path d="M275 133v-31l15-8v31Z" fill="#466c52" />
        <path d="M229 102l23-22 38 14-15 8-23-16Z" fill="#315b48" />
        <path d="M246 133v-18h11v18" fill="#e7d8a9" />
        <path d="M301 158v-43l25-19 26 19v43Z" fill="#c78060" />
        <path d="M352 158v-43l17-9v43Z" fill="#9a5147" />
        <path d="M301 115l25-24 43 15-17 9-26-19Z" fill="#a75849" />
        <path
          d="M322 158v-18h10v18M311 119h8v9h-8ZM335 119h8v9h-8Z"
          fill="#f4deb0"
        />
      </g>
      <g
        transform="translate(71 151) rotate(-14)"
        stroke="#846b3f"
        strokeWidth="2"
      >
        <path d="M0 17v12c0 13 56 13 56 0V17" fill="#c8a557" />
        <ellipse cx="28" cy="17" rx="28" ry="12" fill="#ecd18a" />
        <ellipse cx="28" cy="17" rx="18" ry="7" fill="none" />
        <path d="M-11 0v11c0 13 56 13 56 0V0" fill="#d5b66c" />
        <ellipse cx="17" cy="0" rx="28" ry="12" fill="#f0d994" />
        <path d="M8 0h18M17-6v12" stroke="#9d7c42" />
      </g>
      <g
        transform="translate(395 64) rotate(13)"
        stroke="#778778"
        strokeWidth="2"
      >
        <path d="M0 0h43v43H0Z" fill="#fff9e6" />
        <path d="M43 0l10-7v42l-10 8Z" fill="#c8d1ba" />
        <path d="M0 0l10-7h43L43 0Z" fill="#eef2da" />
        <g fill="#365c4c" stroke="none">
          <circle cx="11" cy="11" r="4" />
          <circle cx="32" cy="11" r="4" />
          <circle cx="21.5" cy="21.5" r="4" />
          <circle cx="11" cy="32" r="4" />
          <circle cx="32" cy="32" r="4" />
        </g>
      </g>
      <g
        transform="translate(136 202) rotate(-12)"
        stroke="#788576"
        strokeWidth="2"
      >
        <rect width="35" height="35" rx="5" fill="#fff9e8" />
        <g fill="#476752" stroke="none">
          <circle cx="10" cy="10" r="3.5" />
          <circle cx="25" cy="25" r="3.5" />
          <circle cx="17.5" cy="17.5" r="3.5" />
        </g>
      </g>
      <path
        d="M35 32h77M35 39h43"
        stroke="#718d6e"
        strokeWidth="2"
        opacity=".5"
      />
    </svg>
  );
}
