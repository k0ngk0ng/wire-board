// Original Wire Board illustration: knight and crowned king on a perspective
// chessboard. Chessnut is used only for the separately licensed playing pieces.
const svg = (body: string) =>
  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 480 300">${body}</svg>`;
export const coverSVG = svg(
  `<rect width="480" height="300" rx="24" fill="#e8e3d5"/><circle cx="375" cy="76" r="53" fill="#d4bfa0"/><path d="M0 225Q140 150 255 207T480 190V300H0Z" fill="#c6d1bf"/><g stroke="#738174" stroke-width="2" stroke-linejoin="round"><path d="M59 232 177 166 426 218 305 286Z" fill="#f8f0db"/>${Array.from(
    { length: 4 },
    (_, r) =>
      Array.from({ length: 4 }, (_, c) => {
        if ((r + c) % 2) return "";
        const x = 59 + c * 62 + r * 29.5,
          y = 232 + c * 13 - r * 16.5;
        return `<path d="M${x} ${y}l62 13 29.5-16.5-62-13Z" fill="#879c8b"/>`;
      }).join(""),
  ).join(
    "",
  )}</g><ellipse cx="194" cy="230" rx="64" ry="17" fill="#394e4333"/><ellipse cx="319" cy="217" rx="46" ry="13" fill="#394e4326"/><g stroke="#43564d" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"><path d="M266 220h92l-5-16h-82Z" fill="#eee6cf"/><path d="M280 202q15-47 10-67h44q-6 25 12 67Z" fill="#f8f1de"/><path d="M282 131h61l-10-31-20 12-18-12Z" fill="#ece0c2"/><path d="M313 106V66m-12 13h24" fill="none"/><path d="M130 241h128l-7-19H139Z" fill="#405e53"/><path d="M149 219q-5-29 22-57l-31-5-14-21 36-41 15-27 20 17 22-7 9 26q32 47 13 115Z" fill="#577667"/><path d="m147 141 25-18 16 4m-3-27 8 4m-9 65q28-9 31-34m-52-41 22 3m-37 113h75" fill="none"/><circle cx="181" cy="113" r="4" fill="#f8f1de"/></g><g stroke="#b28d60" stroke-width="3" fill="none" stroke-linecap="round"><path d="M71 98h24m-12-12v24m318 47h15m-7-7v14"/></g>`,
);
export const boardSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512"><rect width="512" height="512" fill="#eee5cc"/>${Array.from({ length: 64 }, (_, i) => (((i % 8) + Math.floor(i / 8)) % 2 ? `<rect x="${(i % 8) * 64}" y="${Math.floor(i / 8) * 64}" width="64" height="64" fill="#8b9e85"/>` : "")).join("")}</svg>`;
