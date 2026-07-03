import type { Config } from "tailwindcss";

const config: Config = {
  content: ["./app/**/*.{ts,tsx}", "./components/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        eidolon: {
          bg: "#0B0E11",
          surface: "#12161B",
          surface2: "#171C22",
          border: "#232932",
          text: "#E6EAEE",
          muted: "#7C8792",
          faint: "#4A5460",
          accent: "#4FD1C5",
          accentDim: "#2A5F5A",
          danger: "#E0555A",
          warn: "#E8B75A",
          ok: "#6FCF97",
        },
      },
      fontFamily: {
        sans: ["var(--font-inter)", "system-ui", "sans-serif"],
        mono: ["var(--font-jbmono)", "ui-monospace", "monospace"],
      },
      boxShadow: {
        glow: "0 0 0 1px rgba(79, 209, 197, 0.15), 0 0 24px rgba(79, 209, 197, 0.08)",
      },
    },
  },
  plugins: [],
};

export default config;
