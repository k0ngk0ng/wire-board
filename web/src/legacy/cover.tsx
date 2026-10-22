import { SanguoshaCover } from "../../../games/sanguosha/web/Cover";
import { gemColors } from "../../../games/splendor/web/colors";
export function LegacyCover({
  kind,
  mini = false,
}: {
  kind: string;
  mini?: boolean;
}) {
  return (
    <div className={`cover ${kind} ${mini ? "mini" : ""}`} aria-hidden="true">
      {kind === "sanguosha" ? (
        <SanguoshaCover />
      ) : kind === "carcassonne" ? (
        <svg preserveAspectRatio="xMidYMid slice" viewBox="0 0 520 260">
          <rect width="520" height="260" fill="#bfd0b7" />
          <circle cx="422" cy="59" r="34" fill="#f3d992" />
          <path d="M0 156Q100 96 218 160T520 142V260H0Z" fill="#8fa77f" />
          <path d="M0 204Q135 154 273 202T520 180V260H0Z" fill="#5d8060" />
          <g transform="translate(160 35) rotate(8 100 100)">
            <rect
              x="5"
              y="8"
              width="204"
              height="204"
              rx="7"
              fill="#294a3c"
              opacity=".2"
            />
            {[
              [0, 0],
              [102, 0],
              [0, 102],
              [102, 102],
            ].map(([x, y], i) => (
              <rect
                key={i}
                x={x}
                y={y}
                width="99"
                height="99"
                rx="4"
                fill={i === 2 ? "#93ad69" : "#abc17d"}
                stroke="#eddbac"
                strokeWidth="3"
              />
            ))}
            <path
              d="M1 48Q52 42 51 99M51 103Q48 151 0 157M103 151Q155 151 154 204"
              fill="none"
              stroke="#f3e4b9"
              strokeWidth="11"
            />
            <path
              d="M104 0H200V59L184 69H118L104 59Z"
              fill="#be9971"
              stroke="#ecdbb2"
              strokeWidth="4"
            />
            <path
              d="M116 61V24H128V33H138V24H150V33H160V24H172V33H184V24H194V61"
              fill="#ead5a3"
              stroke="#866c49"
              strokeWidth="2"
            />
            <path d="M147 61V49Q155 38 163 49V61" fill="#856847" />
            <path
              d="M26 178l9-19 9 19M70 136l9-19 9 19M164 120l9-19 9 19"
              fill="#4f8052"
              stroke="#f0dfad"
              strokeWidth="1.5"
            />
            <g transform="translate(64 67) scale(1.25)">
              <path
                d="M-5-13Q0-19 5-13L5-7L14 0L10 5L5 2L10 14H2L0 7L-2 14H-10L-5 2L-10 5L-14 0L-5-7Z"
                fill="#b95546"
                stroke="#f5dfae"
                strokeWidth="2"
              />
            </g>
            <g transform="translate(158 157) scale(1.25)">
              <path
                d="M-5-13Q0-19 5-13L5-7L14 0L10 5L5 2L10 14H2L0 7L-2 14H-10L-5 2L-10 5L-14 0L-5-7Z"
                fill="#376c89"
                stroke="#f5dfae"
                strokeWidth="2"
              />
            </g>
          </g>
          <path d="M34 27h75M34 33h42" stroke="#55726c" opacity=".6" />
        </svg>
      ) : kind === "catan" ? (
        <svg
          preserveAspectRatio="xMidYMid slice"
          viewBox="0 0 520 260"
          aria-hidden="true"
        >
          <rect width="520" height="260" fill="#527f8a" />
          <circle cx="423" cy="58" r="34" fill="#e5c986" opacity=".8" />
          <path
            d="M0 205Q95 174 180 202T350 203T520 193V260H0Z"
            fill="#386671"
          />
          <path
            d="M43 187h43M391 211h51M84 224h55"
            stroke="#acc8bf"
            strokeWidth="3"
            strokeLinecap="round"
            opacity=".5"
          />
          {[
            [220, 75, 0],
            [295, 75, 3],
            [183, 140, 1],
            [257, 140, 2],
            [331, 140, 4],
            [220, 205, 3],
            [295, 205, 0],
          ].map(([x, y, c], i) => (
            <g key={i}>
              <path
                d={`M${x} ${y - 44}l38 22v44l-38 22-38-22v-44Z`}
                fill={
                  ["#34703d", "#b8643d", "#91ac54", "#ddad44", "#78868d"][c]
                }
                stroke="#f2d899"
                strokeWidth="5"
              />
              <circle cx={x} cy={y} r="14" fill="#f5e8b9" />
              <text
                x={x}
                y={y + 5}
                textAnchor="middle"
                fill="#563c24"
                fontSize="16"
              >
                {[6, 4, 5, 8, 10, 3, 9][i]}
              </text>
            </g>
          ))}
          <g fill="#b95845" stroke="#f0d5a0" strokeWidth="2">
            <path d="M217 121v-21l15-13 15 13v21Z" />
            <path d="M285 186v-21l15-13 15 13v21Z" />
          </g>
          <path d="M34 27h75M34 33h42" stroke="#d7c18a" opacity=".6" />
        </svg>
      ) : kind === "splendor" ? (
        <svg preserveAspectRatio="xMidYMid slice" viewBox="0 0 520 260">
          <defs>
            <linearGradient id="gemsky" x2="0" y2="1">
              <stop stopColor="#204e58" />
              <stop offset="1" stopColor="#0c302e" />
            </linearGradient>
          </defs>
          <rect width="520" height="260" fill="url(#gemsky)" />
          <circle cx="400" cy="78" r="54" fill="#d3b469" opacity=".22" />
          <path
            d="M0 180L70 140l45 30 55-50 65 68 76-55 69 46 60-42 80 65v58H0"
            fill="#41655a"
          />
          <path
            d="M0 218l85-20 91 12 110-20 93 22 141-25v73H0"
            fill="#173d39"
          />
          {[0, 1, 2].map((x) => (
            <g
              key={x}
              transform={`translate(${145 + x * 92},${78 + Math.abs(x - 1) * 16}) rotate(${(x - 1) * 14})`}
            >
              <rect
                width="82"
                height="124"
                rx="7"
                fill="#d2b681"
                stroke="#eed5a5"
                strokeWidth="2"
              />
              <rect
                x="6"
                y="7"
                width="70"
                height="81"
                rx="3"
                fill={["#3b674d", "#2c5778", "#763d37"][x]}
              />
              <path
                d="M16 42h50L41 76Z M16 42l12-17h26l12 17"
                fill={["#4fa574", "#559bd0", "#d87162"][x]}
                stroke="#e7d7ad"
              />
              <path
                d="M28 25l13 51 13-51M16 42h50"
                fill="none"
                stroke="#ffffff77"
              />
              <circle cx="22" cy="105" r="9" fill="#f6ebcf" />
              <circle cx="44" cy="105" r="9" fill="#353f41" />
              <circle cx="65" cy="105" r="9" fill="#ac3937" />
            </g>
          ))}
          {[0, 1, 2, 3, 4].map((x) => (
            <g key={x} transform={`translate(${119 + x * 68},224)`}>
              <ellipse cy="4" rx="23" ry="9" fill="#091f1c" />
              <ellipse
                rx="22"
                ry="9"
                fill={gemColors[x]}
                stroke="#f7e8c8"
                strokeWidth="2"
              />
            </g>
          ))}
          <path d="M34 27h75M34 33h42" stroke="#d7c18a" opacity=".6" />
        </svg>
      ) : (
        <svg preserveAspectRatio="xMidYMid slice" viewBox="0 0 520 260">
          <rect width="520" height="260" fill="#c4d6d3" />
          <circle cx="422" cy="60" r="33" fill="#f8df9d" />
          <path
            d="m0 130 83-86 66 74 64-55 103 94 69-58 135 65v96H0"
            fill="#8dafab"
          />
          <path
            d="m0 163 115-54 73 62 74-37 113 52 78-44 67 49v69H0"
            fill="#5d8c80"
          />
          <path d="M0 233Q150 134 520 221v39H0" fill="#d9bd87" />
          <path
            d="M-30 260Q155 140 550 219M-20 278Q150 156 550 237"
            fill="none"
            stroke="#564d44"
            strokeWidth="5"
          />
          <path
            d="M-30 271Q155 147 550 228"
            fill="none"
            stroke="#564d44"
            strokeWidth="23"
            strokeDasharray="4 14"
          />
          <g transform="translate(190 154) rotate(-4)">
            <path
              d="M-39-17q-23-8-13-24t-12-30"
              fill="none"
              stroke="#f8f3df"
              strokeWidth="13"
              strokeLinecap="round"
            />
            <path d="M-62 29h159l-9-14H-59Z" fill="#263f43" />
            <rect x="-45" y="-9" width="76" height="32" rx="8" fill="#c65342" />
            <path d="M-35-12v-27h16v27M16 19v-58h42V19" fill="#284e53" />
            <rect x="24" y="-32" width="24" height="20" fill="#e8c479" />
            <path d="M10-40h55" stroke="#e8c479" strokeWidth="5" />
            {[-27, 5, 42, 76].map((x) => (
              <circle
                key={x}
                cx={x}
                cy="28"
                r="12"
                fill="#263b3d"
                stroke="#d6b97b"
                strokeWidth="4"
              />
            ))}
            <path d="m76 13 23 19H66" fill="#c65342" />
          </g>
          <path d="M34 27h75M34 33h42" stroke="#55726c" opacity=".6" />
        </svg>
      )}
    </div>
  );
}
