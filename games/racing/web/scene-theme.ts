export type SceneTheme = {
  sky: string;
  horizon: string;
  fog: string;
  ground: string;
  road: string;
  edge: string;
  accent: string;
  night: boolean;
  sun: number;
};
export const sceneThemes: Record<string, SceneTheme> = {
  metro: {
    sky: "#080d22",
    horizon: "#384267",
    fog: "#1d2843",
    ground: "#18212e",
    road: "#26313d",
    edge: "#25dcdf",
    accent: "#f35ac4",
    night: true,
    sun: 1.4,
  },
  harbor: {
    sky: "#101c32",
    horizon: "#8c6974",
    fog: "#39455a",
    ground: "#343f48",
    road: "#333d47",
    edge: "#f3b85e",
    accent: "#3ebdd2",
    night: true,
    sun: 1.7,
  },
  coast: {
    sky: "#799ebf",
    horizon: "#ffd4a2",
    fog: "#bca6a1",
    ground: "#b6a183",
    road: "#454956",
    edge: "#f3d69e",
    accent: "#62c6ca",
    night: false,
    sun: 3.1,
  },
  woodland: {
    sky: "#7f9ca8",
    horizon: "#dce2bb",
    fog: "#8fa7a4",
    ground: "#455a42",
    road: "#3a4448",
    edge: "#e3d9a8",
    accent: "#87bcb5",
    night: false,
    sun: 3,
  },
  canyon: {
    sky: "#7095b5",
    horizon: "#f4cf9c",
    fog: "#d4a57d",
    ground: "#b88559",
    road: "#514a46",
    edge: "#eed59a",
    accent: "#df8a54",
    night: false,
    sun: 3.4,
  },
};
export const themeFor = (id: string) => sceneThemes[id] ?? sceneThemes.metro;
