import { useId } from "react";

// A heart-powered bomb and two cooperating hands give Guandan its own motion.
export function CoverArt() {
  const grain = useId();
  return (
    <svg
      viewBox="0 0 520 260"
      preserveAspectRatio="xMidYMid meet"
      style={{ background: "#dce5d5" }}
      aria-hidden="true"
    >
      <defs>
        <pattern id={grain} width="8" height="8" patternUnits="userSpaceOnUse">
          <circle cx="3" cy="2" r=".6" fill="#345b53" opacity=".11" />
        </pattern>
      </defs>
      <rect width="520" height="260" fill="#dce5d5" />
      <path d="M0 205Q98 171 176 208T357 208T520 175V260H0Z" fill="#bacfba" />
      <circle cx="264" cy="131" r="99" fill="#edf0d9" />
      <path
        d="M170 48l33 25-7-39 40 22 20-36 20 39 39-23-4 42 47-10-24 37 43 16-36 21 29 37-44-1 2 43-35-23-20 37-20-37-39 21 7-42-43 3 26-32-37-19 40-14Z"
        fill="#e7cf8e"
      />
      <path
        d="M167 168q-67 11-107-20m109 43q-51 19-101 9M356 94q46-33 96-23m-90 53q55-19 99-2"
        fill="none"
        stroke="#769b87"
        strokeWidth="4"
        strokeLinecap="round"
      />
      {/* Split card pairs travel inward from two opposite partners. */}
      <g stroke="#648376" strokeWidth="2.4" strokeLinejoin="round">
        <g transform="translate(83 82) rotate(-26 33 47)">
          <rect x="-13" y="10" width="60" height="83" rx="5" fill="#72968a" />
          <rect width="60" height="83" rx="5" fill="#fff9e6" />
          <path
            d="M12 10C1 20 7 25 12 20c5 5 11 0 0-10ZM9 28h6l-3-8Z"
            fill="#416759"
            stroke="none"
          />
          <path
            d="M30 25C5 47 20 57 30 45c10 12 25 2 0-20ZM23 62h14l-7-18Z"
            fill="#416759"
            stroke="none"
          />
        </g>
        <g transform="translate(380 118) rotate(24 31 44)">
          <rect x="12" y="8" width="60" height="83" rx="5" fill="#72968a" />
          <rect width="60" height="83" rx="5" fill="#fff9e6" />
          <path
            d="M12 10C1 20 7 25 12 20c5 5 11 0 0-10ZM9 28h6l-3-8Z"
            fill="#416759"
            stroke="none"
          />
          <path
            d="M30 25C5 47 20 57 30 45c10 12 25 2 0-20ZM23 62h14l-7-18Z"
            fill="#416759"
            stroke="none"
          />
        </g>
      </g>
      <g
        fill="#ebc69b"
        stroke="#8a775c"
        strokeWidth="2.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M0 167l41-6 20-18q9-8 14-2l-8 15 23-9q13-4 14 4l-18 12 24-5q12-1 12 7l-34 17 27-1q12 2 10 10l-48 13-24-2-35 16" />
        <path d="M520 186l-32-3-21-20q-8-9-15-3l6 16-20-11q-11-6-15 1l16 15-25-8q-12-4-15 3l34 23-29-3q-12 1-11 9l47 17 25 1 45 23" />
      </g>
      <path
        d="M0 167l29-3 16 43-31 16H0Z"
        fill="#557e75"
        stroke="#45645b"
        strokeWidth="2.5"
      />
      <path
        d="M520 186l-24-2-18 46 42 20Z"
        fill="#b66e60"
        stroke="#93574e"
        strokeWidth="2.5"
      />
      <ellipse cx="263" cy="221" rx="73" ry="12" fill="#607e69" opacity=".2" />
      {/* The large, egg-shaped heart bomb refers to both the name and wild cards. */}
      <g
        transform="rotate(12 263 144)"
        stroke="#754d49"
        strokeWidth="3"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M248 80v-16h28v16" fill="#627769" />
        <path
          d="M261 64q-12-31 15-37 21-3 20-14"
          fill="none"
          stroke="#856e45"
          strokeWidth="5"
        />
        <path
          d="M259 77c-45 0-74 49-66 91 6 34 30 48 66 48s62-17 66-49c5-41-20-90-66-90Z"
          fill="#b96762"
        />
        <path
          d="M217 112q-15 24-14 48"
          fill="none"
          stroke="#dfaaa0"
          strokeWidth="7"
        />
        <path
          d="M321 157q-5 40-39 46"
          fill="none"
          stroke="#97534f"
          strokeWidth="5"
        />
        <path
          d="M261 123c-37-44-72 21 0 52 72-31 37-96 0-52Z"
          fill="#f7e5b8"
          stroke="#ecd39d"
          strokeWidth="2"
        />
      </g>
      <g
        fill="#bd7858"
        stroke="#bd7858"
        strokeWidth="2.5"
        strokeLinecap="round"
      >
        <path d="M307 24l9-12m-1 24 15-2m-37-19-2-12" fill="none" />
        <path d="M314 54l7-10 8 10-8 10Z" stroke="none" />
      </g>
      <g fill="#b16b5d">
        <path d="M170 209c-15-18-28 7 0 20 28-13 15-38 0-20ZM358 48c-12-14-22 6 0 15 22-9 12-29 0-15Z" />
      </g>
      <g fill="#718e75">
        <path d="M51 60l8-10 8 10-8 10ZM455 217l7-9 7 9-7 9Z" />
        <circle cx="422" cy="44" r="4" />
        <circle cx="153" cy="43" r="3" />
      </g>
      <rect
        width="520"
        height="260"
        fill={`url(#${grain})`}
        pointerEvents="none"
      />
    </svg>
  );
}
