/** SI motion shared by the original-game reference driving model. */
export type ClassicMotionState = {
  x: number;
  z: number;
  vx: number;
  vz: number;
  angle: number;
  yawRate: number;
  steering: number;
  longAccel: number;
  lateralAccel: number;
  rpm: number;
  gear: number;
  shiftTimer: number;
  throttle: number;
  brake: number;
  handbrake: number;
  reverseTimer: number;
};
export type ClassicMotionInput = {
  steer?: number;
  throttle?: number;
  brakeAmount?: number;
  handbrake?: boolean;
  allowReverse?: boolean;
};
export type ClassicRoadSurface = {
  resistance?: number;
  grip?: number;
  grade?: number;
  normalY?: number;
};
export type ClassicVehicleParams = {
  gearCount: number;
  reverseVelocityToRPM: number;
  reverseEfficiency: number;
  id: string;
  mass: number;
  wheelbase: number;
  tireRadius: number;
  maxSpeed: number;
  idleRPM: number;
  redlineRPM: number;
  finalDrive: number;
  maxBraking: number;
  frontGripBias: number;
  frontBrakeBias: number;
  frontDriveRatio: number;
  turnInRate: number;
  turnOutRate: number;
  turningRadius: number;
  lateralGrip: number;
  gasOffFactor: number;
  shiftDelay: number;
  gearRatios: number[];
  gearEfficiency: number[];
  velocityToRPM: number[];
  torqueCurve: number[];
};

/** Base CARP values shipped in the demo; physical interpretation is independent. */
const CLASSIC_VEHICLES: Record<string, ClassicVehicleParams> = {
  p911: {
    id: "p911",
    mass: 1500.0,
    wheelbase: 2.272,
    tireRadius: 0.3141,
    maxSpeed: 80.555557,
    idleRPM: 975,
    redlineRPM: 6500,
    finalDrive: 3.444444,
    maxBraking: 11.8,
    frontGripBias: 0.45,
    frontBrakeBias: 0.6345,
    frontDriveRatio: 0.0,
    turnInRate: 4.0,
    turnOutRate: 8.0,
    turningRadius: 11.0,
    lateralGrip: 3.7,
    gasOffFactor: 0.3424,
    shiftDelay: 0.1875,
    gearCount: 6,
    reverseVelocityToRPM: 299.19516,
    reverseEfficiency: 0.8,
    gearRatios: [3.818182, 2.15, 1.45, 1.11, 0.92, 0.7],
    gearEfficiency: [0.68, 0.7, 0.82, 0.91, 0.95, 1.28],
    velocityToRPM: [
      399.833557, 225.144363, 151.841553, 116.237328, 96.340843, 73.302818,
    ],
    torqueCurve: [
      0.0, 205.0, 240.0, 275.0, 345.0, 455.0, 500.0, 525.0, 535.0, 540.0, 515.0,
      500.0, 455.0, 410.0, 360.0, 310.0, 0.0, 0.0, 0.0, 0.0, 0.0,
    ],
  },
  fm50: {
    id: "fm50",
    mass: 1690.0,
    wheelbase: 2.5,
    tireRadius: 0.33185,
    maxSpeed: 88.962402,
    idleRPM: 1155,
    redlineRPM: 7700,
    finalDrive: 3.86,
    maxBraking: 10.2,
    frontGripBias: 0.481,
    frontBrakeBias: 0.5312,
    frontDriveRatio: 0.0,
    turnInRate: 3.0,
    turnOutRate: 6.0,
    turningRadius: 5.75,
    lateralGrip: 3.43,
    gasOffFactor: 0.374991,
    shiftDelay: 0.25,
    gearCount: 6,
    reverseVelocityToRPM: 360.994202,
    reverseEfficiency: 0.8,
    gearRatios: [3.153846, 2.17647, 1.565217, 1.185185, 0.935483, 0.757575],
    gearEfficiency: [0.67, 0.67, 0.67, 0.7, 0.91, 1.28],
    velocityToRPM: [
      350.313904, 241.75177, 173.856735, 131.644608, 103.908997, 84.147835,
    ],
    torqueCurve: [
      0.0, 350.0, 390.0, 425.0, 430.0, 430.0, 450.0, 475.0, 530.0, 550.0, 568.5,
      550.0, 525.0, 510.0, 490.0, 450.0, 400.0, 350.0, 0.0, 0.0, 0.0,
    ],
  },
  ldia: {
    id: "ldia",
    mass: 1625.0,
    wheelbase: 2.65,
    tireRadius: 0.34585,
    maxSpeed: 93.055557,
    idleRPM: 1030,
    redlineRPM: 7300,
    finalDrive: 3.8319,
    maxBraking: 10.25,
    frontGripBias: 0.47,
    frontBrakeBias: 0.55624,
    frontDriveRatio: 0.5,
    turnInRate: 4.5,
    turnOutRate: 8.0,
    turningRadius: 6.27,
    lateralGrip: 3.64,
    gasOffFactor: 0.285,
    shiftDelay: 0.1875,
    gearCount: 5,
    reverseVelocityToRPM: 224.831268,
    reverseEfficiency: 0.8,
    gearRatios: [2.31, 1.52, 1.12, 0.83, 0.68, 0.0],
    gearEfficiency: [0.86, 0.87, 0.87, 0.89, 1.055, 0.0],
    velocityToRPM: [244.4048, 160.82048, 118.499298, 87.816444, 71.946007, 0.0],
    torqueCurve: [
      0.0, 300.0, 350.0, 401.0, 425.0, 450.0, 450.0, 460.0, 515.0, 550.0, 575.0,
      600.0, 585.0, 550.0, 500.0, 450.0, 0.0, 0.0, 0.0, 0.0, 0.0,
    ],
  },
  mcf1: {
    id: "mcf1",
    mass: 915.0,
    wheelbase: 2.718,
    tireRadius: 0.4736,
    maxSpeed: 88.068306,
    idleRPM: 1095,
    redlineRPM: 7300,
    finalDrive: 3.45,
    maxBraking: 11.8,
    frontGripBias: 0.49,
    frontBrakeBias: 0.5328,
    frontDriveRatio: 0.0,
    turnInRate: 4.0,
    turnOutRate: 8.0,
    turningRadius: 6.1,
    lateralGrip: 3.9,
    gasOffFactor: 0.425,
    shiftDelay: 0.125,
    gearCount: 6,
    reverseVelocityToRPM: 197.10582,
    reverseEfficiency: 0.799988,
    gearRatios: [2.857143, 2.176471, 1.789474, 1.473684, 1.272727, 1.15],
    gearEfficiency: [0.75, 0.95, 0.95, 0.899994, 0.95, 1.06],
    velocityToRPM: [
      193.707443, 147.489532, 121.661873, 99.912262, 86.318756, 74.764282,
    ],
    torqueCurve: [
      0.0, 474.529999, 528.762024, 576.215027, 582.994019, 582.994019,
      610.109985, 644.005005, 677.900024, 714.506592, 711.794983, 691.458008,
      664.34198, 610.109985, 542.320007, 474.25473, 0.0, 0.0, 0.0, 0.0, 0.0,
    ],
  },
  ccor: {
    id: "ccor",
    mass: 1472.0,
    wheelbase: 2.6543,
    tireRadius: 0.3386,
    maxSpeed: 76.445084,
    idleRPM: 900,
    redlineRPM: 6000,
    finalDrive: 3.15,
    maxBraking: 10.5,
    frontGripBias: 0.49,
    frontBrakeBias: 0.5328,
    frontDriveRatio: 0.0,
    turnInRate: 4.0,
    turnOutRate: 8.0,
    turningRadius: 6.1,
    lateralGrip: 3.23,
    gasOffFactor: 0.405,
    shiftDelay: 0.1875,
    gearCount: 4,
    reverseVelocityToRPM: 179.70903,
    reverseEfficiency: 0.799988,
    gearRatios: [3.73, 2.12, 1.43, 1.04, 0.0, 0.0],
    gearEfficiency: [0.83, 0.82, 0.85, 1.18, 0.0, 0.0],
    velocityToRPM: [292.713837, 166.368179, 112.220047, 75.232437, 0.0, 0.0],
    torqueCurve: [
      0.0, 302.367798, 380.979797, 393.182007, 412.163208, 420.298004,
      440.63501, 454.192993, 474.529999, 474.529999, 440.63501, 406.73999,
      372.845001, 338.950012, 305.054993, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0,
    ],
  },
};
export function classicVehicle(id: string): ClassicVehicleParams {
  const p = Object.hasOwn(CLASSIC_VEHICLES, id)
    ? CLASSIC_VEHICLES[id]
    : CLASSIC_VEHICLES.p911;
  return {
    ...p,
    gearRatios: [...p.gearRatios],
    gearEfficiency: [...p.gearEfficiency],
    velocityToRPM: [...p.velocityToRPM],
    torqueCurve: [...p.torqueCurve],
  };
}

const finite = (n: number | undefined, fallback = 0) =>
  Number.isFinite(n) ? (n as number) : fallback;
const clamp = (n: number, lo: number, hi: number) =>
  Math.max(lo, Math.min(hi, n));
const toward = (n: number, target: number, step: number) =>
  n < target ? Math.min(target, n + step) : Math.max(target, n - step);
const sign = (n: number) => (n > 0 ? 1 : n < 0 ? -1 : 0);
function torque(p: ClassicVehicleParams, rpm: number) {
  const x = clamp(rpm / 500, 0, 20),
    i = Math.min(19, Math.floor(x));
  return p.torqueCurve[i] + (p.torqueCurve[i + 1] - p.torqueCurve[i]) * (x - i);
}
function validVehicle(p: ClassicVehicleParams) {
  if (!Number.isInteger(p.gearCount) || p.gearCount < 1 || p.gearCount > 6)
    return false;
  const positive = [
    p.mass,
    p.wheelbase,
    p.tireRadius,
    p.maxSpeed,
    p.idleRPM,
    p.redlineRPM,
    p.finalDrive,
    p.maxBraking,
    p.turnInRate,
    p.turnOutRate,
    p.turningRadius,
    p.lateralGrip,
    p.shiftDelay,
    p.reverseVelocityToRPM,
    p.reverseEfficiency,
  ];
  if (positive.some((n) => !Number.isFinite(n) || n <= 0)) return false;
  if (
    [p.frontGripBias, p.frontBrakeBias, p.frontDriveRatio, p.gasOffFactor].some(
      (n) => !Number.isFinite(n),
    )
  )
    return false;
  if (
    !p.gearRatios ||
    !p.gearEfficiency ||
    !p.velocityToRPM ||
    !p.torqueCurve ||
    p.torqueCurve.length !== 21
  )
    return false;
  for (let i = 0; i < p.gearCount; i++)
    if (
      [p.gearRatios[i], p.gearEfficiency[i], p.velocityToRPM[i]].some(
        (n) => !Number.isFinite(n) || n <= 0,
      )
    )
      return false;
  if (p.torqueCurve.some((n) => !Number.isFinite(n) || n < 0)) return false;
  return true;
}
function cleanState(
  input: ClassicMotionState,
  p: ClassicVehicleParams,
): ClassicMotionState {
  return {
    x: finite(input.x),
    z: finite(input.z),
    vx: clamp(finite(input.vx), -200, 200),
    vz: clamp(finite(input.vz), -200, 200),
    angle: finite(input.angle),
    yawRate: clamp(finite(input.yawRate), -3, 3),
    steering: clamp(finite(input.steering), -1, 1),
    longAccel: clamp(finite(input.longAccel), -30, 30),
    lateralAccel: clamp(finite(input.lateralAccel), -40, 40),
    rpm: clamp(finite(input.rpm, p.idleRPM), p.idleRPM, p.redlineRPM * 1.05),
    gear:
      Number.isInteger(input.gear) &&
      input.gear >= -1 &&
      input.gear <= p.gearCount &&
      input.gear !== 0
        ? input.gear
        : 1,
    shiftTimer: clamp(finite(input.shiftTimer), 0, 2),
    throttle: clamp(finite(input.throttle), 0, 1),
    brake: clamp(finite(input.brake), 0, 1),
    handbrake: clamp(finite(input.handbrake), 0, 1),
    reverseTimer: clamp(finite(input.reverseTimer), 0, 1),
  };
}

/** Fixed 20 ms maximum steps. Stiffness, CG height, low-speed stability and drag
 * fitting are design choices, not claims about recovered original formulas. */
export function stepClassicMotion(
  state: ClassicMotionState,
  input: ClassicMotionInput,
  vehicle: ClassicVehicleParams,
  surface: ClassicRoadSurface,
  dt: number,
): ClassicMotionState {
  let p = vehicle;
  if (!validVehicle(p)) p = classicVehicle(p.id);
  let s = cleanState(state, p);
  const controls = {
    steer: clamp(finite(input.steer), -1, 1),
    throttle: clamp(finite(input.throttle), 0, 1),
    brakeAmount: clamp(finite(input.brakeAmount), 0, 1),
    handbrake: !!input.handbrake,
    allowReverse: !!input.allowReverse,
  };
  let grip = finite(surface.grip, 1),
    normalY = finite(surface.normalY, 1);
  if (grip <= 0) grip = 1;
  if (normalY <= 0) normalY = 1;
  const road = {
    grip: clamp(grip, 0.1, 1.5),
    normalY: clamp(normalY, 0.25, 1),
    grade: clamp(finite(surface.grade), -0.65, 0.65),
    resistance: clamp(finite(surface.resistance), 0, 20),
  };
  dt = clamp(finite(dt), 0, 0.1);
  for (let left = dt; left > 1e-9; left -= 0.02)
    s = motionStep(s, controls, p, road, Math.min(0.02, left));
  return s;
}

function motionStep(
  s: ClassicMotionState,
  input: Required<ClassicMotionInput>,
  p: ClassicVehicleParams,
  road: Required<ClassicRoadSurface>,
  dt: number,
): ClassicMotionState {
  let cs = Math.cos(s.angle),
    sn = Math.sin(s.angle);
  const u = s.vx * cs + s.vz * sn,
    v = -s.vx * sn + s.vz * cs;
  let steerRate = p.turnInRate;
  if (
    input.steer * s.steering < 0 ||
    Math.abs(input.steer) < Math.abs(s.steering)
  )
    steerRate = p.turnOutRate;
  const steerTarget = input.steer * s.steering < 0 ? 0 : input.steer;
  s.steering = toward(s.steering, steerTarget, steerRate * dt);
  s.handbrake = toward(s.handbrake, input.handbrake ? 1 : 0, 10 * dt);
  if (
    input.allowReverse &&
    Math.abs(u) < 0.4 &&
    ((s.gear > 0 && input.brakeAmount > 0.5 && input.throttle < 0.1) ||
      (s.gear < 0 && input.throttle > 0.5 && input.brakeAmount < 0.1))
  ) {
    s.reverseTimer += dt;
    if (s.reverseTimer >= 0.3) {
      s.gear = s.gear < 0 ? 1 : -1;
      s.shiftTimer = p.shiftDelay;
      s.throttle = 0;
      s.brake = 0;
      s.reverseTimer = 0;
    }
  } else s.reverseTimer = 0;
  let throttleTarget = input.throttle,
    brakeTarget = input.brakeAmount;
  if (s.gear < 0) {
    if (input.allowReverse) {
      throttleTarget = input.brakeAmount;
      brakeTarget = input.throttle;
    } else {
      throttleTarget = 0;
      brakeTarget = Math.max(input.throttle, input.brakeAmount);
    }
  }
  const throttleRate =
    throttleTarget < s.throttle ? (32 / 255) * 32 : (16 / 255) * 32;
  s.throttle = toward(s.throttle, throttleTarget, throttleRate * dt);
  const brakeCurve = [64, 32, 16, 8, 4, 2, 1, 1],
    idx = Math.min(7, Math.round(s.brake * 255) >> 5);
  const brakeRate =
    brakeTarget < s.brake ? (32 / 255) * 32 : (brakeCurve[idx] / 255) * 32;
  s.brake = toward(s.brake, brakeTarget, brakeRate * dt);
  s.shiftTimer = Math.max(0, s.shiftTimer - dt);
  let gearIndex = Math.max(0, s.gear - 1);
  let velocityRatio =
    s.gear < 0 ? p.reverseVelocityToRPM : p.velocityToRPM[gearIndex];
  let wheelRPM = Math.abs(u) * velocityRatio;
  if (s.shiftTimer === 0 && s.gear > 0) {
    if (s.gear < p.gearCount && wheelRPM > p.redlineRPM * 0.94) {
      s.gear++;
      s.shiftTimer = p.shiftDelay;
    } else if (
      s.gear > 1 &&
      wheelRPM < p.redlineRPM * 0.36 &&
      Math.abs(u) * p.velocityToRPM[s.gear - 2] < p.redlineRPM * 0.78
    ) {
      s.gear--;
      s.shiftTimer = p.shiftDelay;
    }
    gearIndex = s.gear - 1;
    velocityRatio = p.velocityToRPM[gearIndex];
    wheelRPM = Math.abs(u) * velocityRatio;
  }
  let rpmTarget = Math.max(p.idleRPM, wheelRPM);
  if (Math.abs(u) < 5 && s.throttle > 0)
    rpmTarget = Math.max(rpmTarget, p.idleRPM + s.throttle * 1200);
  rpmTarget = clamp(rpmTarget, p.idleRPM, p.redlineRPM * 1.025);
  s.rpm += (rpmTarget - s.rpm) * (1 - Math.exp(-12 * dt));
  const efficiency =
    s.gear < 0 ? p.reverseEfficiency : p.gearEfficiency[gearIndex];
  let engineForce =
    torque(p, s.rpm) *
    velocityRatio *
    ((2 * Math.PI) / 60) *
    efficiency *
    s.throttle;
  if (s.shiftTimer > 0) engineForce = 0;
  if (s.gear < 0) {
    engineForce = -engineForce;
    if (u < -12) engineForce = 0;
  }
  if (wheelRPM > p.redlineRPM * 1.025) engineForce = 0;
  const brakeForce = p.mass * p.maxBraking * s.brake * sign(u);
  const frontFraction = clamp(p.frontGripBias, 0.3, 0.65),
    a = p.wheelbase * (1 - frontFraction),
    b = p.wheelbase * frontFraction;
  const weight = p.mass * 9.81 * road.normalY;
  const frontLoad = clamp(
      weight * frontFraction - (p.mass * s.longAccel * 0.48) / p.wheelbase,
      weight * 0.15,
      weight * 0.85,
    ),
    rearLoad = weight - frontLoad;
  const mu = p.lateralGrip * 0.5 * road.grip,
    frontLimit = mu * frontLoad,
    rearLimit = mu * rearLoad;
  const frontX = clamp(
    engineForce * p.frontDriveRatio - brakeForce * p.frontBrakeBias,
    -frontLimit,
    frontLimit,
  );
  const rearX = clamp(
    engineForce * (1 - p.frontDriveRatio) -
      brakeForce * (1 - p.frontBrakeBias) -
      sign(u) * p.mass * 5 * s.handbrake,
    -rearLimit,
    rearLimit,
  );
  const frontYLimit = Math.sqrt(
    Math.max(0, frontLimit * frontLimit - frontX * frontX),
  );
  const rearYLimit =
    Math.sqrt(Math.max(0, rearLimit * rearLimit - rearX * rearX)) *
    (1 - 0.82 * s.handbrake);
  const maxSteer = Math.atan(p.wheelbase / Math.max(p.turningRadius, 1));
  const delta = (s.steering * maxSteer) / (1 + 0.000055 * u * u),
    denom = Math.max(Math.abs(u), 1);
  const alphaFront = Math.atan2(v + a * s.yawRate, denom) - delta * sign(u),
    alphaRear = Math.atan2(v - b * s.yawRate, denom);
  const frontY =
    frontYLimit > 1e-6
      ? -frontYLimit * Math.tanh((frontLoad * 10 * alphaFront) / frontYLimit)
      : 0;
  const rearY =
    rearYLimit > 1e-6
      ? -rearYLimit * Math.tanh((rearLoad * 10 * alphaRear) / rearYLimit)
      : 0;
  const topIndex = p.gearCount - 1;
  const topRPM = p.maxSpeed * p.velocityToRPM[topIndex];
  const topAcceleration =
    (torque(p, topRPM) *
      p.velocityToRPM[topIndex] *
      ((2 * Math.PI) / 60) *
      p.gearEfficiency[topIndex]) /
    p.mass;
  const dragCoefficient = Math.max(
    0.00001,
    (topAcceleration - 0.13) / (p.maxSpeed * p.maxSpeed),
  );
  const rolling = 0.13 * (1 + Math.max(0, 1 - road.grip) * 3);
  let dragAcceleration =
    (rolling +
      road.resistance * clamp(Math.abs(u) / 6, 0, 1) +
      dragCoefficient * u * u) *
    sign(u);
  if (s.throttle < 0.05)
    dragAcceleration +=
      p.gasOffFactor * (1 + p.gearRatios[gearIndex] * 0.35) * sign(u);
  const longAccel =
    (frontX + rearX - frontY * Math.sin(delta)) / p.mass -
    dragAcceleration -
    9.81 * Math.sin(road.grade);
  const latAccel = (frontY * Math.cos(delta) + rearY) / p.mass;
  let nextU = u + (longAccel + v * s.yawRate) * dt;
  let nextV = v + (latAccel - u * s.yawRate) * dt;
  const inertia = p.mass * p.wheelbase * p.wheelbase * 0.35;
  let nextYaw =
    s.yawRate + ((a * frontY * Math.cos(delta) - b * rearY) / inertia) * dt;
  nextYaw *= Math.exp(-(0.18 + 0.18 * (1 - s.handbrake)) * dt);
  const beta = Math.atan((b / p.wheelbase) * Math.tan(delta));
  const kinYaw = (nextU * Math.tan(delta)) / p.wheelbase,
    kinV = nextU * Math.tan(beta);
  const blend = clamp((Math.abs(u) - 2) / 5, 0, 1);
  nextV = kinV * (1 - blend) + nextV * blend;
  nextYaw = kinYaw * (1 - blend) + nextYaw * blend;
  const maxYaw = Math.min(
    2.4,
    Math.max(0.35, (mu * 9.81) / (Math.abs(u) + 4)) * 1.3,
  );
  nextYaw = clamp(nextYaw, -maxYaw, maxYaw);
  if (
    s.brake > 0.01 &&
    throttleTarget < 0.01 &&
    (u * nextU < 0 || Math.abs(u) < p.maxBraking * s.brake * dt + 0.02)
  )
    nextU = 0;
  if (Math.abs(nextU) < 0.015 && s.throttle < 0.01) {
    nextU = 0;
    nextV = 0;
    nextYaw = 0;
  }
  const oldYaw = s.yawRate;
  s.yawRate = nextYaw;
  s.angle = Math.atan2(
    Math.sin(s.angle + (oldYaw + nextYaw) * 0.5 * dt),
    Math.cos(s.angle + (oldYaw + nextYaw) * 0.5 * dt),
  );
  cs = Math.cos(s.angle);
  sn = Math.sin(s.angle);
  s.vx = nextU * cs - nextV * sn;
  s.vz = nextU * sn + nextV * cs;
  const speed = Math.hypot(s.vx, s.vz);
  if (speed > p.maxSpeed * 1.4) {
    s.vx *= (p.maxSpeed * 1.4) / speed;
    s.vz *= (p.maxSpeed * 1.4) / speed;
  }
  s.x += s.vx * dt;
  s.z += s.vz * dt;
  s.longAccel = longAccel;
  s.lateralAccel = latAccel;
  return s;
}
