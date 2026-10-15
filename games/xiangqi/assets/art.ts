// Original Wire Board xiangqi artwork; Chinese letterforms use Noto Serif SC.
const wrap = (w: number, h: number, body: string) =>
  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${w} ${h}">${body}</svg>`;
const glyphs = ["", "帅", "仕", "相", "马", "车", "炮", "兵"];
const black = ["", "将", "士", "象", "马", "车", "炮", "卒"];
const token = (x: number, y: number, r: number, text: string, red: boolean) =>
  `<g transform="translate(${x} ${y})"><ellipse cy="${r * 0.12}" rx="${r}" ry="${r * 0.98}" fill="#51433033"/><circle r="${r * 0.96}" fill="#f5dfb4" stroke="#795a36" stroke-width="${r * 0.065}"/><circle r="${r * 0.81}" fill="#fae8c5" stroke="${red ? "#9e302c" : "#263c34"}" stroke-width="${r * 0.045}"/><text y="${r * 0.37}" text-anchor="middle" fill="${red ? "#9e302c" : "#263c34"}" font-family="Noto Serif SC,serif" font-size="${r * 1.2}" font-weight="700">${text}</text></g>`;
export const pieceSVG = (code: number) =>
  wrap(
    160,
    160,
    token(80, 77, 74, (code > 0 ? glyphs : black)[Math.abs(code)], code > 0),
  );
export const boardSVG = wrap(
  450,
  500,
  `<rect width="450" height="500" rx="12" fill="#e6cca0"/><g stroke="#b89154" stroke-width="1" opacity=".18" fill="none">${Array.from({ length: 15 }, (_, i) => `<path d="M${i * 34} 0q-20 120 5 250t-4 250"/>`).join("")}</g><path d="M0 237q95-20 210 5t240-4v24q-105 15-240-4T0 262Z" fill="#8caaa13b"/>`,
);
export const coverSVG = wrap(
  480,
  300,
  `<rect width="480" height="300" rx="24" fill="#eae0c7"/><circle cx="386" cy="64" r="40" fill="#d4ae87"/><path d="M0 137q70-43 143-12t177-5 160 7v173H0Z" fill="#c6cdb8"/><path d="M0 218q88-76 194-51t286-27v49q-174 9-262 18T0 260Z" fill="#8ba89e"/><g stroke="#8a7758" stroke-width="1.5" fill="none" opacity=".8"><path d="M28 251 223 139 442 195 247 291Z" fill="#e8d5ab"/>${[0, 1, 2, 3, 4].map((i) => `<path d="M${28 + i * 48.75} ${251 - i * 28}l219 56M${28 + i * 54.75} ${251 + i * 14}l195-112"/>`).join("")}<path d="M130 224l108-16-12 56Z"/></g>${token(128, 172, 64, "帅", true)}${token(326, 129, 51, "将", false)}${token(293, 245, 34, "马", true)}<g stroke="#718a70" stroke-width="3" fill="none" stroke-linecap="round"><path d="M42 120q5-30 17-42m-12 25-14-6m18-4 14-6M420 270q4-24 20-45m-13 28-14-7m17-7 14-5"/></g>`,
);
