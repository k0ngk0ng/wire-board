// Original editable Wire Board artwork. No third-party image source.
const svg = (size: string, body: string) =>
  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${size}">${body}</svg>`;
const lines = Array.from(
  { length: 8 },
  (_, i) => `<path d="M30 ${30 + i * 60}H450M${30 + i * 60} 30V450"/>`,
).join("");
export const boardSVG = svg(
  "0 0 480 480",
  `<defs><linearGradient id="wood" x2="1" y2="1"><stop stop-color="#e8d4a4"/><stop offset="1" stop-color="#d5b77c"/></linearGradient></defs><rect width="480" height="480" rx="14" fill="url(#wood)"/><g stroke="#b28f53" opacity=".18" fill="none">${Array.from({ length: 18 }, (_, i) => `<path d="M${i * 29} 0q-20 130 5 240t-4 240"/>`).join("")}</g>`,
);
export const blackStoneSVG = svg(
  "0 0 80 80",
  `<defs><radialGradient id="stone" cx=".3" cy=".25"><stop stop-color="#626c70"/><stop offset=".55" stop-color="#293439"/><stop offset="1" stop-color="#11191e"/></radialGradient></defs><circle cx="40" cy="41" r="37" fill="#3d372c" opacity=".2"/><circle cx="40" cy="38" r="35" fill="url(#stone)" stroke="#11191e" stroke-width="2"/><ellipse cx="29" cy="20" rx="10" ry="5" fill="#fff" opacity=".12"/>`,
);
export const whiteStoneSVG = svg(
  "0 0 80 80",
  `<defs><radialGradient id="stone" cx=".3" cy=".25"><stop stop-color="#fffef5"/><stop offset=".65" stop-color="#f2edda"/><stop offset="1" stop-color="#c3c3b7"/></radialGradient></defs><circle cx="40" cy="41" r="37" fill="#3d372c" opacity=".2"/><circle cx="40" cy="38" r="35" fill="url(#stone)" stroke="#727d73" stroke-width="2"/>`,
);
export const coverSVG = svg(
  "0 0 480 280",
  `<rect width="480" height="280" fill="#a7bba7"/><path d="M0 226Q135 173 260 233T480 212V280H0" fill="#6e947e"/><g transform="translate(78 -12) rotate(12 170 170) scale(.75)"><rect x="-10" y="-10" width="500" height="500" rx="20" fill="#527562"/><rect width="480" height="480" rx="14" fill="#ead7a8"/><g stroke="#9e8758" stroke-width="2" fill="none">${lines}</g>${[90, 150, 210, 270, 330].map((x, i) => `<circle cx="${x}" cy="${90 + i * 60}" r="23" fill="#253c36"/><circle cx="${x - 7}" cy="${83 + i * 60}" r="7" fill="#567269"/>`).join("")}${[
    [90, 150],
    [150, 210],
    [270, 90],
    [330, 270],
  ]
    .map(
      ([x, y]) =>
        `<circle cx="${x}" cy="${y}" r="23" fill="#faf3db" stroke="#a49978" stroke-width="3"/>`,
    )
    .join(
      "",
    )}<path d="M90 90L330 330" stroke="#d39b4d" stroke-width="5" stroke-linecap="round" stroke-dasharray="4 11"/></g><circle cx="39" cy="58" r="15" fill="#f0d27f"/><path d="M415 225q26-55 47-12q-7 32-47 12" fill="#416a55"/>`,
);
