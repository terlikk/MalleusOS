// Punkt startowy panelu: montujemy główny komponent w #app.
// W Svelte 5 służy do tego funkcja mount().
import { mount } from "svelte";
import App from "./App.svelte";
import "./app.css";

// Fonty lokalne z pakietów npm — Vite wkleja pliki woff2 do dist/,
// więc panel działa w 100% offline (zero CDN-ów).
import "@fontsource/jetbrains-mono/400.css";
import "@fontsource/jetbrains-mono/700.css";

mount(App, { target: document.getElementById("app") });
