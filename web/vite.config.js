// Konfiguracja Vite — narzędzia, które podczas developmentu
// serwuje panel z podmianą na żywo, a przy budowaniu skleja
// wszystko do katalogu dist/ (ten trafia potem do binarki Go).
import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

export default defineConfig({
  plugins: [svelte()],
  server: {
    // W trybie dev zapytania /api lecą do działającej binarki
    // malleus — panel z Vite, dane z prawdziwego agenta.
    proxy: {
      "/api": "http://localhost:8443",
    },
  },
});
