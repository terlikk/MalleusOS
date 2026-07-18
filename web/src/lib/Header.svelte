<script>
  // Nagłówek: nazwa hosta, dioda stanu połączenia i "chipy"
  // z danymi systemu. $props() to sposób Svelte 5 na propsy.
  import { uptime } from "./format.js";

  let { system = null, online = false } = $props();
</script>

<header>
  <span class="led" class:off={!online} title={online ? "połączono" : "łączenie…"}></span>
  <h1>
    {system?.hostname ?? "…"}
    <small>· MalleusOS</small>
  </h1>
  <div class="chips">
    {#if system?.os}<span class="chip">{system.os}</span>{/if}
    {#if system?.kernel}<span class="chip mono">{system.kernel} · {system.arch}</span>{/if}
    <span class="chip">uptime <b>{uptime(system?.uptimeSeconds)}</b></span>
  </div>
</header>

<style>
  header {
    display: flex;
    align-items: center;
    gap: 0.85rem;
    margin-bottom: 1.3rem;
    flex-wrap: wrap;
  }

  .led {
    width: 9px;
    height: 9px;
    border-radius: 50%;
    background: var(--cyan);
    box-shadow: 0 0 10px 2px rgba(83, 214, 232, 0.55);
    animation: pulse 2.2s ease-in-out infinite;
  }
  /* Brak połączenia: dioda gaśnie na bursztynowo i nie pulsuje */
  .led.off {
    background: var(--amber);
    box-shadow: 0 0 8px 2px rgba(255, 180, 84, 0.4);
    animation: none;
  }
  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.45; }
  }

  h1 {
    font-size: 1.12rem;
    font-weight: 650;
    letter-spacing: -0.01em;
  }
  h1 small { color: var(--dim); font-weight: 500; }

  .chips { margin-left: auto; display: flex; gap: 0.5rem; flex-wrap: wrap; }
  .chip {
    font-size: 0.74rem;
    color: var(--dim);
    background: var(--card);
    border: 1px solid var(--edge);
    border-radius: 999px;
    padding: 0.3rem 0.8rem;
  }
  .chip b { color: var(--text); font-weight: 600; }

  @media (prefers-reduced-motion: reduce) {
    .led { animation: none; }
  }
</style>
