// Original Wire Board military-game art; no commercial game screenshots.
export const names: Record<string, string> = {
  commander: "司令",
  general: "军长",
  division: "师长",
  brigade: "旅长",
  regiment: "团长",
  battalion: "营长",
  company: "连长",
  platoon: "排长",
  engineer: "工兵",
  bomb: "炸弹",
  mine: "地雷",
  flag: "军旗",
};
export const armyNames = ["红方", "蓝方", "黄方", "绿方"];
export const colors = ["#9b4338", "#376687", "#856521", "#3d6951"];
const svg = (w: number, h: number, body: string) =>
  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${w} ${h}">${body}</svg>`;
const star = (x: number, y: number, r: number) =>
  Array.from({ length: 10 }, (_, i) => {
    const a = (i * Math.PI) / 5 - Math.PI / 2,
      d = i % 2 ? r * 0.42 : r;
    return (
      (i ? "L" : "M") + (x + Math.cos(a) * d) + " " + (y + Math.sin(a) * d)
    );
  }).join(" ") + "Z";
export const pieceSVG = (seat: number, kind: string) =>
  svg(
    160,
    136,
    `<rect x="8" y="14" width="144" height="116" rx="15" fill="#4b493a44"/><rect x="7" y="6" width="146" height="116" rx="15" fill="#efe1bb" stroke="${colors[seat]}" stroke-width="7"/><rect x="16" y="15" width="128" height="98" rx="9" fill="${kind === "back" ? colors[seat] : "#fff2d2"}" stroke="${colors[seat]}" stroke-width="2"/>${kind === "back" ? `<path d="${star(80, 62, 34)}" fill="#fff2d2"/>` : `<text x="80" y="79" text-anchor="middle" font-family="Noto Serif SC,serif" font-size="52" font-weight="700" fill="${colors[seat]}">${names[kind]}</text>`}`,
  );
export const boardSVG = (teams: boolean) =>
  svg(
    teams ? 720 : 360,
    teams ? 720 : 840,
    `<rect width="100%" height="100%" rx="16" fill="#e5e1c7"/><g stroke="#9fa68c" fill="none" stroke-width="1" opacity=".32">${Array.from({ length: 15 }, (_, i) => `<path d="M0 ${i * 60}q130-40 360 0t360 0"/>`).join("")}</g>`,
  );
export const coverSVG = svg(
  480,
  300,
  `
<rect width="480" height="300" rx="24" fill="#e7dfc9"/>
<circle cx="64" cy="66" r="38" fill="#d4b393"/>
<path d="M0 168q95-35 214-9t266-27v168H0Z" fill="#bdcab1"/>
<path d="M29 221 186 143 464 214 326 293Z" fill="#e6d2a9" stroke="#899074" stroke-width="3"/>
<g stroke="#869178" stroke-width="2" fill="none"><path d="M72 224 203 163 426 220 312 273ZM129 244l128-65m-60 83 120-67M115 204l234 54"/><path d="M73 229 207 171 418 223" stroke-dasharray="5 4"/></g>
<path d="m355 119 34-21 50 20v35h-84Z" fill="#94a286" stroke="#738368" stroke-width="2"/><path d="M389 100v52m0-30 12 30h-24Z" fill="#6e8264"/>
<g stroke="#3e5540" stroke-linejoin="round" stroke-linecap="round">
 <rect x="56" y="191" width="267" height="61" rx="29" fill="#3e5140" stroke-width="4"/>
 <rect x="69" y="201" width="241" height="40" rx="20" fill="#6c7e5c" stroke-width="3"/>
 ${[91, 132, 173, 214, 255, 291].map((x, i) => `<circle cx="${x}" cy="221" r="${i === 5 ? 13 : 16}" fill="#96a27b" stroke-width="4"/><circle cx="${x}" cy="221" r="5" fill="#526d4c" stroke-width="2"/>`).join("")}
 <path d="m69 154 198-23 64 27-14 37H57Z" fill="#6d835c" stroke-width="4"/>
 <path d="m267 131 64 27 14 20-28 17-7-28Z" fill="#526d4b" stroke-width="3"/>
 <path d="M59 190h253M76 155l23 24 213-13" fill="none" stroke-width="3"/>
 <ellipse cx="180" cy="150" rx="68" ry="13" fill="#536e4d" stroke-width="3"/>
 <path d="m124 111 53-21 55 16 21 35-40 18-96-8Z" fill="#7b8d63" stroke-width="4"/>
 <path d="m232 106 21 35-40 18-8-36Z" fill="#5d7851" stroke-width="3"/>
 <ellipse cx="175" cy="102" rx="26" ry="10" fill="#a0aa7d" stroke-width="3"/>
 <path d="M160 94h30" stroke-width="4"/>
 <g transform="translate(226 118) rotate(-12)"><rect x="0" y="-8" width="175" height="16" rx="3" fill="#667e55" stroke-width="3"/><rect x="162" y="-11" width="29" height="22" rx="3" fill="#536d4b" stroke-width="3"/><path d="M68-9v18m89-18v18" stroke-width="3"/></g>
 <path d="M140 108 129 52" stroke-width="2"/>
 <path d="${star(170, 128, 13)}" fill="#e7d5a3" stroke="none"/>
 <rect x="77" y="164" width="37" height="12" rx="5" fill="#a6ac83" stroke-width="2"/>
 <circle cx="311" cy="176" r="5" fill="#e4cd97" stroke-width="2"/>
 <path d="M79 247h212" stroke="#b5b38a" stroke-width="2" stroke-dasharray="6 6"/>
</g>
<g transform="translate(344 169) rotate(6)">${pieceSVG(0, "flag").replace('viewBox="0 0 160 136"', 'width="106" height="90" viewBox="0 0 160 136"')}</g>
<g transform="translate(280 225) rotate(-6)">${pieceSVG(3, "commander").replace('viewBox="0 0 160 136"', 'width="92" height="78" viewBox="0 0 160 136"')}</g>
`,
);
