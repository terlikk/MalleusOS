// Formatowanie liczb po polsku (przecinek dziesiętny).

// bytes: 1234567 → "1,2 MB"
export function bytes(n) {
  if (n == null) return "—";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i++;
  }
  const val = n >= 100 || i === 0 ? Math.round(n) : n.toFixed(1).replace(".", ",");
  return `${val} ${units[i]}`;
}

// bps: tempo transferu — bajty na sekundę
export function bps(n) {
  return n == null ? "—" : `${bytes(n)}/s`;
}

// uptime: sekundy → "14 dni 3 h" / "3 h 12 min" / "42 min"
export function uptime(seconds) {
  if (seconds == null) return "—";
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d} dni ${h} h`;
  if (h > 0) return `${h} h ${m} min`;
  return `${m} min`;
}

// Polska odmiana: 1 rdzeń, 2–4 rdzenie, 5+ rdzeni
export function rdzenie(n) {
  if (n == null) return "— rdzeni";
  if (n === 1) return "1 rdzeń";
  if (n >= 2 && n <= 4) return `${n} rdzenie`;
  return `${n} rdzeni`;
}

// Kolor stanu wg progów z projektu: <70% cyjan, 70–90% bursztyn,
// ≥90% czerwień. `frac` to ułamek 0–1.
export function stateColor(frac) {
  if (frac == null) return "var(--dim)";
  if (frac >= 0.9) return "var(--red)";
  if (frac >= 0.7) return "var(--amber)";
  return "var(--cyan)";
}
