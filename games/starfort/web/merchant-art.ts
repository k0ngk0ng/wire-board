// Suit, dark hair and a phone at the ear, following the wish attachment.
// The feet share the same world anchor as convoy sprites and objective hits.
export function drawMerchant(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  size: number,
) {
  ctx.save();
  ctx.translate(x, y);
  ctx.scale(size / 64, size / 64);
  ctx.lineWidth = 2;
  ctx.lineJoin = "round";
  ctx.strokeStyle = "#393244";
  const shape = (points: number[][], color: string) => {
    ctx.fillStyle = color;
    ctx.beginPath();
    points.forEach(([x, y], i) => (i ? ctx.lineTo(x, y) : ctx.moveTo(x, y)));
    ctx.closePath();
    ctx.fill();
    ctx.stroke();
  };
  ctx.fillStyle = "#263d2930";
  ctx.beginPath();
  ctx.ellipse(0, -1, 20, 5, 0, 0, Math.PI * 2);
  ctx.fill();
  shape(
    [
      [-13, -24],
      [1, -24],
      [0, -3],
      [-14, -3],
    ],
    "#686277",
  );
  shape(
    [
      [1, -24],
      [13, -24],
      [15, -3],
      [3, -3],
    ],
    "#686277",
  );
  shape(
    [
      [-15, -6],
      [0, -6],
      [1, -1],
      [-17, -1],
    ],
    "#35313d",
  );
  shape(
    [
      [3, -6],
      [15, -6],
      [19, -1],
      [3, -1],
    ],
    "#35313d",
  );
  shape(
    [
      [-13, -44],
      [12, -44],
      [15, -20],
      [-15, -20],
    ],
    "#91849f",
  );
  shape(
    [
      [-6, -44],
      [6, -44],
      [0, -27],
    ],
    "#fff9ed",
  );
  shape(
    [
      [-2, -42],
      [2, -42],
      [4, -31],
      [0, -27],
      [-3, -31],
    ],
    "#494050",
  );
  shape(
    [
      [-13, -44],
      [-4, -43],
      [-8, -36],
      [-3, -32],
      [-8, -24],
    ],
    "#b0a1b8",
  );
  shape(
    [
      [12, -44],
      [4, -43],
      [8, -36],
      [3, -32],
      [8, -24],
    ],
    "#b0a1b8",
  );
  // Left hand in the pocket; right elbow bent toward the handset.
  shape(
    [
      [-13, -43],
      [-20, -38],
      [-18, -24],
      [-10, -25],
      [-10, -30],
      [-13, -31],
    ],
    "#91849f",
  );
  shape(
    [
      [12, -43],
      [19, -35],
      [23, -39],
      [22, -51],
      [17, -52],
      [16, -43],
    ],
    "#91849f",
  );
  shape(
    [
      [17, -52],
      [16, -58],
      [21, -59],
      [24, -53],
      [22, -47],
    ],
    "#e8c1a6",
  );
  ctx.fillStyle = "#edc7ac";
  ctx.beginPath();
  ctx.ellipse(0, -53, 12, 14, 0, 0, Math.PI * 2);
  ctx.fill();
  ctx.stroke();
  shape(
    [
      [-12, -54],
      [-14, -62],
      [-9, -69],
      [3, -70],
      [12, -65],
      [13, -53],
      [8, -59],
      [4, -62],
      [-5, -60],
      [-10, -60],
    ],
    "#2c2731",
  );
  ctx.fillStyle = "#393244";
  ctx.fillRect(-6, -54, 2, 2);
  ctx.fillRect(4, -54, 2, 2);
  ctx.beginPath();
  ctx.moveTo(-3, -47);
  ctx.quadraticCurveTo(0, -45, 4, -48);
  ctx.stroke();
  shape(
    [
      [13, -62],
      [19, -62],
      [21, -47],
      [15, -47],
    ],
    "#272735",
  );
  ctx.strokeStyle = "#c7c5d3";
  ctx.beginPath();
  ctx.moveTo(15, -59);
  ctx.lineTo(17, -50);
  ctx.stroke();
  ctx.restore();
}
