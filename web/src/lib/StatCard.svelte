<script>
  // Karta metryki z pierścieniem: procent w kole, kolor wg progów
  // 70/90%, obok dwie linijki szczegółów.
  import { stateColor } from "./format.js";

  let {
    label,
    value = null, // liczba do pokazania w środku pierścienia
    unit = "%",
    max = 100,    // pełna skala pierścienia (dla temperatur np. 90)
    line1 = "",
    line2 = "",
  } = $props();

  const R = 34; // promień łuku
  const CIRC = 2 * Math.PI * R;

  let frac = $derived(value == null ? null : Math.min(value / max, 1));
  let color = $derived(stateColor(frac));
  let dash = $derived(frac == null ? 0 : CIRC * frac);
</script>

<div class="card">
  <h2>{label}</h2>
  <div class="stat">
    <div class="ring">
      <svg width="82" height="82" viewBox="0 0 82 82">
        <circle cx="41" cy="41" r={R} fill="none" stroke="var(--track)" stroke-width="7" />
        {#if frac != null}
          <circle
            cx="41" cy="41" r={R} fill="none"
            stroke={color} stroke-width="7" stroke-linecap="round"
            stroke-dasharray="{dash} {CIRC}"
          />
        {/if}
      </svg>
      <div class="val" style="color:{color}">
        {value == null ? "—" : Math.round(value)}<small>{value == null ? "" : unit}</small>
      </div>
    </div>
    <div class="info">
      <b>{line1}</b>
      <span>{line2}</span>
    </div>
  </div>
</div>

<style>
  .stat { display: flex; align-items: center; gap: 1rem; }

  .ring { position: relative; width: 82px; height: 82px; flex: none; }
  /* Łuk rośnie od godziny 12, nie od 3 */
  .ring svg { transform: rotate(-90deg); }
  .ring circle { transition: stroke-dasharray 0.6s ease, stroke 0.3s ease; }

  .val {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.05rem;
    font-weight: 700;
    letter-spacing: -0.02em;
  }
  .val small {
    font-size: 0.62rem;
    font-weight: 500;
    color: var(--dim);
    margin-left: 1px;
  }

  .info { line-height: 1.5; min-width: 0; }
  .info b { display: block; font-size: 0.9rem; font-weight: 600; }
  .info span { font-size: 0.78rem; color: var(--dim); }
</style>
