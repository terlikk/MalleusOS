<script>
  // Tryb kiosku — pełnoekranowy widok pod mały panoramiczny
  // ekranik LCD w obudowie serwera. Wielkie liczby, zero klikania.
  // Otwiera się pod adresem /kiosk.
  import { bps, stateColor } from "./format.js";

  let { sample = null, system = null, online = false } = $props();

  // Zegar odświeżany co sekundę.
  let now = $state(new Date());
  $effect(() => {
    const t = setInterval(() => (now = new Date()), 1000);
    return () => clearInterval(t);
  });

  let clock = $derived(
    now.toLocaleTimeString("pl-PL", { hour: "2-digit", minute: "2-digit" }),
  );

  let maxTemp = $derived(
    sample?.temps?.length
      ? Math.max(...sample.temps.map((t) => t.celsius))
      : null,
  );

  // Jedna pozycja kiosku: etykieta + wartość + ułamek do koloru.
  let stats = $derived([
    { label: "CPU", value: sample ? `${Math.round(sample.cpu.usagePercent)}%` : "—", frac: sample ? sample.cpu.usagePercent / 100 : null },
    { label: "RAM", value: sample ? `${Math.round(sample.mem.usedPercent)}%` : "—", frac: sample ? sample.mem.usedPercent / 100 : null },
    { label: "TEMP", value: maxTemp == null ? "—" : `${Math.round(maxTemp)}°`, frac: maxTemp == null ? null : maxTemp / 90 },
    { label: "↓", value: sample ? bps(sample.net.rxBps) : "—", frac: 0, small: true },
    { label: "↑", value: sample ? bps(sample.net.txBps) : "—", frac: 0, small: true },
  ]);
</script>

<div class="kiosk">
  <div class="left">
    <div class="host">
      <span class="led" class:off={!online}></span>
      {system?.hostname ?? "…"}
    </div>
    <div class="clock mono">{clock}</div>
  </div>

  {#each stats as s (s.label)}
    <div class="stat">
      <div class="label">{s.label}</div>
      <div class="value mono" class:small={s.small} style="color:{stateColor(s.frac)}">{s.value}</div>
    </div>
  {/each}
</div>

<style>
  .kiosk {
    height: 100vh;
    display: flex;
    align-items: center;
    justify-content: space-evenly;
    gap: 2vw;
    padding: 0 2vw;
    overflow: hidden;
  }

  .left { text-align: center; }

  .host {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.6rem;
    font-size: clamp(0.9rem, 2.2vw, 1.6rem);
    font-weight: 650;
    color: var(--dim);
  }
  .led {
    width: 0.55em; height: 0.55em; border-radius: 50%;
    background: var(--cyan);
    box-shadow: 0 0 0.5em 0.15em rgba(167, 139, 250, 0.55);
    animation: pulse 2.2s ease-in-out infinite;
  }
  .led.off { background: var(--amber); animation: none; }
  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.4; }
  }

  .clock {
    font-size: clamp(2.4rem, 7vw, 6rem);
    font-weight: 700;
    letter-spacing: -0.02em;
  }

  .stat { text-align: center; }
  .label {
    font-size: clamp(0.7rem, 1.6vw, 1.1rem);
    letter-spacing: 0.2em;
    color: var(--dim);
    margin-bottom: 0.2em;
  }
  .value {
    font-size: clamp(1.8rem, 6vw, 5rem);
    font-weight: 700;
    letter-spacing: -0.02em;
    white-space: nowrap; /* "78,2 KB/s" ma zostać w jednej linii */
  }
  /* tempo sieci bywa długie — mniejszy stopień pisma */
  .value.small { font-size: clamp(1.2rem, 3.2vw, 2.6rem); }

  @media (prefers-reduced-motion: reduce) {
    .led { animation: none; }
  }
</style>
