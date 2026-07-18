// Rozmowa z API binarki malleus.

// getJSON pobiera i parsuje odpowiedź; błąd HTTP zamienia na wyjątek.
export async function getJSON(path) {
  const res = await fetch(path);
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

// streamMetrics podpina się pod strumień SSE.
// EventSource to wbudowana klasa przeglądarki: sama utrzymuje
// połączenie i wznawia je po zerwaniu. Zwracamy funkcję
// zamykającą — wywołanie jej kończy nasłuch.
export function streamMetrics(onSample, onState) {
  const es = new EventSource("/api/v1/stream");
  es.addEventListener("metrics", (e) => onSample(JSON.parse(e.data)));
  es.onopen = () => onState?.(true);
  es.onerror = () => onState?.(false);
  return () => es.close();
}
