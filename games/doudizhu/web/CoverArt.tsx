import { useId } from "react";

// A landlord faces two farmers: the silhouette identifies the game at card size.
export function CoverArt() {
  const grain = useId();
  return (
    <svg
      viewBox="0 0 520 260"
      preserveAspectRatio="xMidYMid meet"
      style={{ background: "#eadbc0" }}
      aria-hidden="true"
    >
      <defs>
        <pattern id={grain} width="8" height="8" patternUnits="userSpaceOnUse">
          <circle cx="2" cy="3" r=".65" fill="#805e3f" opacity=".12" />
        </pattern>
      </defs>
      <rect width="520" height="260" fill="#eadbc0" />
      <circle cx="408" cy="66" r="37" fill="#f4e9cb" />
      <path d="M0 146Q83 109 181 141T358 141T520 128V260H0Z" fill="#c7c3a0" />
      <path d="M0 188Q134 140 262 191T520 159V260H0Z" fill="#b5b590" />
      <g fill="none" stroke="#ece1be" strokeWidth="3" opacity=".7">
        <path d="M8 235Q102 178 171 175M61 260Q146 201 204 195M440 245l69-48M471 258l48-37" />
      </g>
      <g stroke="#9b8c6a" strokeWidth="2" strokeLinejoin="round">
        <path d="M33 134v-28h56v28" fill="#e8d7ac" />
        <path d="M24 108l38-25 38 25Z" fill="#9a8f72" />
        <path d="M55 134v-18h13v18" fill="#b7a685" />
        <path d="M440 142v-29h45v29" fill="#ded3b2" />
        <path d="M433 115l29-21 31 21Z" fill="#aaa087" />
      </g>
      <ellipse
        cx="260"
        cy="236"
        rx="159"
        ry="15"
        fill="#686448"
        opacity=".18"
      />
      {/* The two farmer partners lean toward the middle. */}
      <g
        stroke="#5e6655"
        strokeWidth="2.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M67 225l7-51q13-20 41-18 28 0 45 25l9 45" fill="#69857d" />
        <path
          d="M92 176l23 18 23-20M114 194l-3 30"
          fill="none"
          stroke="#d4debf"
        />
        <path d="M91 150l4 18q18 15 32-1l5-22" fill="#dfb68a" />
        <ellipse cx="110" cy="127" rx="29" ry="34" fill="#f0cda2" />
        <path
          d="M81 124q-4-35 30-35 32 3 28 37l-10-15-22 3-23 13"
          fill="#555d4e"
        />
        <path
          d="M63 117l46-43 47 39q-39 21-93 4Z"
          fill="#d8b469"
          stroke="#907b46"
        />
        <path
          d="M109 82l-6 29m12-29 19 28m-56 3 68 1"
          fill="none"
          stroke="#ac914e"
          strokeWidth="1.8"
        />
        <path d="M104 130h1m18-3h1m-16 15q7 5 13-1" fill="none" />
        <path d="M142 187q13-10 26 2l-5 16-21-5" fill="#f0cda2" />
        <path d="M354 225l10-44q12-23 39-24 29-1 42 18l13 51" fill="#a77856" />
        <path
          d="M384 173l20 19 23-17m-23 18 5 31"
          fill="none"
          stroke="#efdbb6"
        />
        <path d="M382 149l3 20q21 14 34-3v-20" fill="#ddb286" />
        <ellipse cx="402" cy="127" rx="28" ry="34" fill="#edc89c" />
        <path d="M374 128q-4-40 29-38 34 0 29 35l-14-13-35 6" fill="#59604e" />
        <path d="M369 110l11-25q20-15 45 1l11 25q-26 9-67-1Z" fill="#829081" />
        <path d="M368 111q37-14 74-1l-6 9-64 2Z" fill="#a4ae8d" />
        <path d="M390 129h1m17 1h1m-19 15q8 4 14-2" fill="none" />
        <path d="M375 189q-12-12-26-1l4 17 23-5" fill="#edc89c" />
      </g>
      {/* The landlord's round hat, robe and moustache form one strong focal shape. */}
      <g
        stroke="#694c3b"
        strokeWidth="2.8"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M182 228l9-70q8-25 69-25 57 0 69 26l14 69Z" fill="#b86446" />
        <path
          d="M251 153l15 26 21-28m-21 28 1 51"
          fill="none"
          stroke="#ecd19b"
          strokeWidth="3"
        />
        <path d="M237 132v24q26 17 44-2v-25" fill="#dbab79" />
        <ellipse cx="260" cy="101" rx="42" ry="45" fill="#efd0a0" />
        <path
          d="M218 95l-1-24q4-34 45-34 40 1 41 33v25"
          fill="#3e5047"
          stroke="#37443b"
        />
        <path
          d="M210 85q49-20 102 0l-3 13q-45-14-96 0Z"
          fill="#516356"
          stroke="#37443b"
        />
        <path
          d="M256 60l8-6 8 7-8 9Z"
          fill="#d4b968"
          stroke="#ad914d"
          strokeWidth="1.6"
        />
        <path d="M236 106q7-5 13-1m25 0q6-5 12 0" fill="none" />
        <path
          d="M260 107l-4 12 9 1"
          fill="none"
          stroke="#b18460"
          strokeWidth="2"
        />
        <path
          d="M260 123q-10-7-21 3 8 9 21 2 13 7 22-2-12-10-22-3Z"
          fill="#5a5040"
          stroke="none"
        />
        <path d="M251 136q11 5 20-1" fill="none" strokeWidth="2" />
        <path d="M199 196q-2-15 14-19l24 12-5 19-26-1Z" fill="#efd0a0" />
        <path d="M315 195q2-15-14-19l-23 13 5 20 26-2Z" fill="#efd0a0" />
      </g>
      <g stroke="#81714f" strokeWidth="2" strokeLinejoin="round">
        <g transform="translate(201 161) rotate(-18)">
          <rect width="40" height="62" rx="4" fill="#fcf4da" />
          <path
            d="M20 15C-2 35 12 43 20 33c8 10 22 2 0-18ZM14 46h12l-6-14Z"
            fill="#3e574a"
            stroke="none"
          />
        </g>
        <g transform="translate(242 151) rotate(3)">
          <rect width="40" height="62" rx="4" fill="#fff8e6" />
          <path
            d="M20 24c-19-23-31 10 0 23 31-13 19-46 0-23Z"
            fill="#b95d49"
            stroke="none"
          />
        </g>
        <g transform="translate(280 157) rotate(19)">
          <rect width="40" height="62" rx="4" fill="#fcf4da" />
          <path d="M20 15l12 17-12 17-12-17Z" fill="#b95d49" stroke="none" />
        </g>
        <g transform="translate(133 169) rotate(-26)">
          <rect width="30" height="46" rx="3" fill="#fff5dc" />
          <path d="M15 10l9 12-9 12-9-12Z" fill="#b86446" stroke="none" />
        </g>
        <g transform="translate(367 164) rotate(27)">
          <rect width="30" height="46" rx="3" fill="#fff5dc" />
          <path
            d="M15 11c-15-17-23 6 0 17 23-11 15-34 0-17Z"
            fill="#b86446"
            stroke="none"
          />
        </g>
      </g>
      <g fill="#e3bd6d" stroke="#967440" strokeWidth="1.8">
        <path d="M321 223v8c0 11 48 11 48 0v-8" />
        <ellipse cx="345" cy="223" rx="24" ry="9" fill="#f2d58e" />
        <path d="M332 222h24m-12-5v10" fill="none" />
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
