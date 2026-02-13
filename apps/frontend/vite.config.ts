import { reactRouter } from "@react-router/dev/vite";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";
import tsconfigPaths from "vite-tsconfig-paths";

export default defineConfig({
  plugins: [tailwindcss(), reactRouter(), tsconfigPaths()],
  clearScreen: false,
  server: {
    port: parseInt(process.env.FRONTEND_PORT || "5390", 10),
    strictPort: true,
    // Configure HMR to use Encore's port when accessed through proxy
    hmr: process.env.ENCORE_PORT
      ? {
          clientPort: parseInt(process.env.ENCORE_PORT, 10),
          host: "localhost",
        }
      : true,
  },
});
