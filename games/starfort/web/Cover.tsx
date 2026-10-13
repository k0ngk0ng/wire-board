import { useId } from "react";

// Original vector illustration, shared by the lobby, picker, and waiting room.
export function Cover() {
  const id = useId();
  const paint = (name: string) => `url(#${id}-${name})`;
  return (
    <svg
      viewBox="0 0 520 260"
      preserveAspectRatio="xMidYMid slice"
      className="sf-cover-art"
      style={{ display: "block", width: "100%", height: "100%" }}
      aria-hidden="true"
    >
      <defs>
        <linearGradient id={`${id}-sky`} x2="0" y2="1">
          <stop stopColor="#d9e2d5" />
          <stop offset="1" stopColor="#f4e9c5" />
        </linearGradient>
        <linearGradient id={`${id}-stone`} x2=".25" y2="1">
          <stop stopColor="#f3e4b9" />
          <stop offset="1" stopColor="#b3ad90" />
        </linearGradient>
        <linearGradient id={`${id}-leaf`} x2=".3" y2="1">
          <stop stopColor="#b4ca8b" />
          <stop offset="1" stopColor="#6c9674" />
        </linearGradient>
        <linearGradient id={`${id}-ice`} x2=".7" y2="1">
          <stop stopColor="#e7ffff" />
          <stop offset="1" stopColor="#78acbe" />
        </linearGradient>
        <radialGradient id={`${id}-glow`}>
          <stop stopColor="#fff3ad" stopOpacity=".9" />
          <stop offset="1" stopColor="#fff3ad" stopOpacity="0" />
        </radialGradient>
        <pattern id={`${id}-grain`} width="7" height="7" patternUnits="userSpaceOnUse">
          <circle cx="2" cy="3" r=".55" fill="#4c6a58" opacity=".065" />
        </pattern>
      </defs>
      <rect width="520" height="260" fill={paint("sky")} />
      <circle cx="381" cy="56" r="32" fill="#f8ecc3" />
      <path d="M0 137Q60 73 144 116Q228 49 303 109Q398 49 520 121V260H0Z" fill="#b8c8ad" />
      <path d="M0 159Q98 105 196 152Q284 99 370 139Q456 117 520 147V260H0Z" fill="#9fb79a" />
      <path d="M0 199Q152 117 280 169T520 170V260H0Z" fill="#c1cea1" />
      <path d="M-18 253Q60 180 154 206T252 192Q281 174 320 184T421 204Q451 204 540 251" stroke="#9aab83" strokeWidth="29" fill="none" />
      <path d="M-18 250Q60 177 154 203T252 189Q281 171 320 181T421 201Q451 201 540 248" stroke="#e4d2a1" strokeWidth="25" fill="none" />
      <g fill="none" stroke="#b9a57b" strokeWidth="1.5" opacity=".6">
        <path d="M30 221l11 1m61-27 12 5m65 4 12 4m35-4 11-4m88-15 8 4m46 12 9 4m36 9 12 7" />
      </g>
      <g stroke="#526e58" strokeWidth="2.2" strokeLinejoin="round" strokeLinecap="round">
        {[
          [34, 134, 1.05], [78, 110, .76], [475, 126, 1], [503, 167, .8],
        ].map(([x, y, scale], index) => (
          <g key={index} transform={`translate(${x} ${y}) scale(${scale})`}>
            <ellipse cx="0" cy="34" rx="27" ry="7" fill="#739373" opacity=".2" stroke="none" />
            <path d="M-5-2L-7 30Q0 37 8 30L6-2Z" fill="#a18962" />
            <path d="M-28 1Q-36-12-24-23Q-25-39-9-39Q5-52 16-35Q34-32 28-18Q42-3 28 9Q14 24-3 14Q-23 24-28 1Z" fill={paint("leaf")} />
            <path d="M-20-19Q-20-31-9-31M-8-37Q3-44 10-33M9-22Q19-23 21-12" fill="none" stroke="#c7d8a2" strokeWidth="3" />
          </g>
        ))}
      </g>
      <ellipse cx="281" cy="185" rx="76" ry="14" fill="#5f795e" opacity=".18" />
      <circle cx="285" cy="91" r="67" fill={paint("glow")} />
      <g stroke="#536b65" strokeWidth="2.3" strokeLinejoin="round" strokeLinecap="round">
        <path d="M239 97L324 97L332 178Q284 196 235 178Z" fill={paint("stone")} />
        <path d="M313 102L324 97L332 178L310 184Z" fill="#a6a38c" stroke="none" />
        <path d="M237 142Q282 155 328 141M237 161Q284 174 330 160M264 148V168M304 148V168" fill="none" stroke="#a29b7e" strokeWidth="1.6" />
        <path d="M262 183V159Q283 130 306 159V184" fill="#61797a" />
        <path d="M272 184V161Q284 145 296 161V185" fill="#3d5e68" />
        <path d="M225 173V105L236 99L249 105V182M316 183V106L331 98L343 105V174" fill="#c6c1a2" />
        <path d="M220 107V92H228V98H235V87H243V98H253V108Q236 118 220 107M311 108V92H321V98H328V88H337V98H348V109Q330 118 311 108Z" fill="#e7ddba" />
        <path d="M228 94L282 47L339 94L327 108Q281 119 238 108Z" fill="#64877f" />
        <path d="M228 94L282 47L274 76L249 111L238 108Z" fill="#9ab49b" stroke="none" />
        <path d="M238 98Q282 113 329 98M251 82Q284 96 314 81M267 68Q282 73 298 65" fill="none" stroke="#456d65" strokeWidth="1.5" />
        <path d="M283 48V25L315 34L285 42" fill="#d6b971" />
        <circle cx="283" cy="112" r="22" fill="#527471" />
        <circle cx="283" cy="112" r="17" fill="#efd28a" stroke="#b59555" />
        <path d="M283 98L287 107L298 108L290 116L292 126L283 121L274 126L276 116L268 108L279 107Z" fill="#fff0af" stroke="#bba16a" strokeWidth="1.4" />
        <path d="M230 142v-11q5-8 10 0v11M325 142v-11q5-8 10 0v11" fill="#5e7571" />
      </g>
      <g transform="translate(110 94)" stroke="#596851" strokeWidth="2.2" strokeLinejoin="round" strokeLinecap="round">
        <ellipse cx="40" cy="104" rx="39" ry="8" fill="#577852" opacity=".2" stroke="none" />
        <path d="M11 91L17 44L65 44L71 92Q40 107 11 91Z" fill="#b58b61" />
        <path d="M17 53L65 87M64 52L16 88M15 73H68" stroke="#806747" strokeWidth="3" />
        <path d="M9 44L36 13L73 43L66 53Q39 60 9 50Z" fill="#789667" />
        <path d="M9 44L36 13L42 21L24 53Z" fill="#b1c28a" stroke="none" />
        <path d="M23 45L37 24M39 49L44 28M56 47L53 35" stroke="#5b7c56" strokeWidth="1.5" />
        <path d="M28 85V64Q39 57 51 65V85" fill="#485d47" />
        <path d="M21 72Q40 54 58 73Q40 89 21 72M39 55V91" fill="none" stroke="#e9c995" />
        <path d="M7 93Q40 108 75 93L78 101Q42 116 5 102Z" fill="#c4c0a0" />
        <path d="M68 44V20L89 28L70 34" fill="#d7b475" />
      </g>
      <g transform="translate(371 128)" stroke="#577780" strokeWidth="2.2" strokeLinejoin="round" strokeLinecap="round">
        <ellipse cx="30" cy="63" rx="35" ry="7" fill="#617f6b" opacity=".2" stroke="none" />
        <path d="M6 29H55L59 61Q31 73 1 62Z" fill="#c0d3c6" />
        <ellipse cx="30" cy="31" rx="25" ry="8" fill="#dae5ce" />
        <path d="M13 29L1-1L12-19L24 0L35-38L50-12L44 31Z" fill={paint("ice")} />
        <path d="M35-38L35 29L44 31L50-12Z" fill="#73a6b8" stroke="none" />
        <path d="M1-1L24 10L12-19M24 0L35-38L35 29M50-12L35-2" fill="none" stroke="#e8ffff" strokeWidth="1.7" />
        <path d="M7 50Q30 60 54 50M19 54v11M43 54v11" fill="none" stroke="#98b4aa" strokeWidth="1.6" />
      </g>
      <g transform="translate(58 222)" stroke="#5e785b" strokeWidth="1.8" strokeLinecap="round">
        <ellipse cx="20" cy="18" rx="25" ry="5" fill="#648758" opacity=".18" stroke="none" />
        <path d="M0 12Q1-7 14-14Q28-22 40-8Q51 7 42 16Q29 22 20 18Q6 23 0 12Z" fill={paint("leaf")} />
        <path d="M20-15Q8-31 5-21Q5-12 20-15M20-15Q23-33 32-27Q39-20 20-15" fill="#96b674" />
        <path d="M9-1Q10-9 16-10" fill="none" stroke="#dbe7af" strokeWidth="3.5" />
        <g fill="#3d574b" stroke="none"><ellipse cx="17" cy="5" rx="2" ry="3" /><ellipse cx="31" cy="4" rx="2" ry="3" /></g>
        <path d="M21 11q3 3 6 0" fill="none" strokeWidth="1.3" />
      </g>
      <g fill="#f5e5ad" stroke="#b8a476" strokeWidth="1.2" strokeLinejoin="round">
        <path d="M216 45l3 7 8 1-6 5 2 8-7-4-7 4 2-8-6-5 8-1Z" />
        <path d="M354 29l2 5 6 1-4 4 1 5-5-3-5 3 1-5-4-4 6-1Z" />
      </g>
      <g stroke="#fff2bd" strokeWidth="2.5" strokeLinecap="round"><path d="M348 121v10m-5-5h10M191 112v8m-4-4h8M414 90v8m-4-4h8" /></g>
      <g fill="#9ab37e"><path d="M5 260l9-28 6 16 10-20 4 20 12-11 4 23ZM455 260l7-22 8 10 9-17 5 17 12-7 4 19Z" /></g>
      <rect width="520" height="260" fill={paint("grain")} />
    </svg>
  );
}
