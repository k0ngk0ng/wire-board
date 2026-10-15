// Original editable artwork, maintained with this game.
const svg = (box: string, body: string) =>
  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${box}">${body}</svg>`;
export const boardSVG = svg(
  "0 0 600 600",
  `<rect width="600" height="600" rx="16" fill="#dfc493"/><g fill="none" stroke="#ae8c58" opacity=".15">${Array.from({ length: 24 }, (_, i) => `<path d="M0 ${i * 27}Q180 ${i * 27 - 24} 320 ${i * 27 + 2}T600 ${i * 27 - 9}"/>`).join("")}</g><rect x="4" y="4" width="592" height="592" rx="14" fill="none" stroke="#b09669" stroke-width="3"/>`,
);
export const blackStoneSVG = svg(
  "0 0 80 80",
  `<defs><radialGradient id="b" cx=".3" cy=".22"><stop stop-color="#63706c"/><stop offset=".5" stop-color="#293632"/><stop offset="1" stop-color="#16231f"/></radialGradient></defs><ellipse cx="41" cy="44" rx="36" ry="34" fill="#514d37" opacity=".25"/><circle cx="40" cy="38" r="35" fill="url(#b)" stroke="#17251e" stroke-width="2"/><path d="M23 23q9-9 18-7" fill="none" stroke="#a1b0a4" opacity=".5" stroke-width="3" stroke-linecap="round"/>`,
);
export const whiteStoneSVG = svg(
  "0 0 80 80",
  `<defs><radialGradient id="w" cx=".3" cy=".22"><stop stop-color="#fffdf3"/><stop offset=".6" stop-color="#f1ecdc"/><stop offset="1" stop-color="#c4c6b6"/></radialGradient></defs><ellipse cx="41" cy="44" rx="36" ry="34" fill="#514d37" opacity=".25"/><circle cx="40" cy="38" r="35" fill="url(#w)" stroke="#637061" stroke-width="2"/>`,
);
const stone = (x: number, y: number, c: string, r = 13) =>
  `<ellipse cx="${x + 2}" cy="${y + 3}" rx="${r}" ry="${r * 0.87}" fill="#635c47" opacity=".22"/><circle cx="${x}" cy="${y}" r="${r}" fill="${c}" stroke="${c === "#253e35" ? "#20392f" : "#9b9d84"}" stroke-width="1.6"/><path d="M${x - r * 0.5} ${y - r * 0.4}q${r * 0.3} ${-r * 0.3} ${r * 0.7} ${-r * 0.25}" fill="none" stroke="#fffdf1" opacity=".25" stroke-width="2" stroke-linecap="round"/>`;
export const coverSVG = svg(
  "0 0 480 280",
  `<rect width="480" height="280" fill="#b9c3ad"/><path d="M0 194q130-28 259 12t221-9v83H0" fill="#809981"/><path d="M0 236q144-25 293 5t187-10v49H0" fill="#6a8973"/><g transform="translate(207 22) rotate(9 125 105)"><path d="M-8 3H245V231H-8Z" fill="#8b7d56"/><rect x="-8" y="-8" width="253" height="226" rx="8" fill="#e5cfa0" stroke="#a58d62" stroke-width="3"/><g stroke="#a28d62" fill="none">${Array.from({ length: 7 }, (_, i) => `<path d="M13 ${11 + i * 31}H224M${21 + i * 31} 5V205"/>`).join("")}</g>${[
    [83, 42],
    [114, 42],
    [145, 42],
    [176, 73],
    [176, 104],
    [145, 135],
    [114, 135],
    [83, 135],
    [52, 104],
    [52, 73],
  ]
    .map(([x, y]) => stone(x, y, "#253e35"))
    .join("")}${[
    [21, 42],
    [52, 11],
    [83, 11],
    [145, 166],
    [176, 166],
    [207, 135],
    [207, 104],
  ]
    .map(([x, y]) => stone(x, y, "#fff8e1"))
    .join(
      "",
    )}</g><ellipse cx="119" cy="239" rx="98" ry="19" fill="#456650" opacity=".28"/><path d="M25 166q2 78 83 78t83-78" fill="#86664b" stroke="#66563f" stroke-width="3"/><path d="M36 185q72 37 143 0M46 213q61 29 122 0" fill="none" stroke="#ae8860" stroke-width="3"/><ellipse cx="108" cy="165" rx="83" ry="33" fill="#bc9a70" stroke="#66563f" stroke-width="3"/><ellipse cx="108" cy="164" rx="69" ry="24" fill="#5f5842"/>${[
    [65, 160],
    [86, 149],
    [111, 150],
    [139, 153],
    [154, 166],
    [123, 173],
    [92, 174],
    [68, 174],
  ]
    .map(([x, y]) => stone(x, y, "#253e35", 12))
    .join(
      "",
    )}<path d="M31 91q0 44 51 45t53-45" fill="#ae8962" stroke="#725c43" stroke-width="3"/><ellipse cx="83" cy="91" rx="52" ry="20" fill="#d5b78a" stroke="#725c43" stroke-width="3"/><ellipse cx="83" cy="91" rx="42" ry="13" fill="#897554"/>${[
    [58, 91],
    [80, 87],
    [103, 90],
    [91, 100],
  ]
    .map(([x, y]) => stone(x, y, "#fff8e1", 10))
    .join(
      "",
    )}${stone(207, 244, "#fff8e1", 14)}${stone(237, 257, "#253e35", 13)}<path d="M27 47q30-13 66-9" stroke="#edf0d8" stroke-width="3" fill="none" stroke-linecap="round"/>`,
);
