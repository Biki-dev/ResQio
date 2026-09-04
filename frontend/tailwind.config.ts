import type { Config } from "tailwindcss";

// ResQio — Government of India style design tokens.
// ink      - official navy: used as chrome background (header/hero/footer)
//            AND as heading/body text color on light surfaces.
// paper    - off-white document surface.
// verified - official green, used for "verified" ticks and provider actions.
// signal   - official gold, reserved for primary calls-to-action.
// alert    - notice red, reserved strictly for warnings/emergency banners.
// flag     - the national tricolor, used ONLY as a thin signature stripe.
const config: Config = {
  content: [
    "./app/**/*.{ts,tsx}",
    "./components/**/*.{ts,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        ink: {
          DEFAULT: "#0B3558",
          light: "#123F66",
          deep: "#062338",
          border: "#2A567F",
        },
        paper: {
          DEFAULT: "#F7F6F2",
          dim: "#EDEBE3",
        },
        verified: {
          DEFAULT: "#0F7B3E",
          light: "#2E9E5B",
        },
        signal: {
          DEFAULT: "#C9861D",
          dark: "#A66C12",
        },
        alert: {
          DEFAULT: "#B91C1C",
        },
        slate: {
          DEFAULT: "#55606B",
          light: "#8B95A1",
        },
        flag: {
          saffron: "#FF9933",
          white: "#FFFFFF",
          green: "#128807",
        },
      },
      fontFamily: {
        display: ["var(--font-display)", "serif"],
        body: ["var(--font-body)", "sans-serif"],
      },
      borderRadius: {
        none: "0px",
        sm: "1px",
        DEFAULT: "2px",
        md: "3px",
      },
      keyframes: {
        marquee: {
          "0%": { transform: "translateX(0%)" },
          "100%": { transform: "translateX(-50%)" },
        },
      },
      animation: {
        marquee: "marquee 28s linear infinite",
      },
    },
  },
  plugins: [],
};

export default config;
