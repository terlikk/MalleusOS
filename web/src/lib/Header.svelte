<script>
  // Nagłówek: nazwa hosta, dioda stanu połączenia i "chipy"
  // z danymi systemu. $props() to sposób Svelte 5 na propsy.
  import { uptime } from "./format.js";
  import { getJSON } from "./api.js";

  let { system = null, online = false } = $props();

  // Samo-aktualizacja: sprawdzamy raz przy starcie, czy na GitHubie
  // jest nowsze wydanie MalleusOS. Chip w nagłówku jest widoczny
  // zawsze — można też sprawdzić ręcznie w każdej chwili.
  let update = $state(null);
  let updating = $state(false);
  let checking = $state(false);
  let checkMsg = $state("");

  $effect(() => {
    getJSON("/api/v1/update").then((u) => (update = u)).catch(() => {});
  });

  async function checkUpdate() {
    checking = true;
    checkMsg = "";
    try {
      update = await getJSON("/api/v1/update");
      if (!update.available) {
        checkMsg = update.current?.includes("dev")
          ? "wersja deweloperska — aktualizacje z gita"
          : "masz najnowszą wersję ✓";
      }
    } catch {
      checkMsg = "nie udało się sprawdzić";
    } finally {
      checking = false;
      // komunikat znika sam, chip wraca do zwykłej postaci
      setTimeout(() => (checkMsg = ""), 5000);
    }
  }

  async function applyUpdate() {
    if (!confirm(`Zaktualizować MalleusOS do ${update.latest}? Panel zrestartuje się.`)) return;
    updating = true;
    try {
      await fetch("/api/v1/update", { method: "POST" });
      // Serwer podmienia binarkę i restartuje się — po chwili
      // przeładowujemy stronę, już w nowej wersji.
      setTimeout(() => window.location.reload(), 4000);
    } catch {
      updating = false;
    }
  }
</script>

<header>
  <span class="led" class:off={!online} title={online ? "połączono" : "łączenie…"}></span>
  <h1>
    {system?.hostname ?? "…"}
    <small>· MalleusOS</small>
  </h1>
  <div class="chips">
    {#if update?.available}
      <button class="chip update" onclick={applyUpdate} disabled={updating}>
        {updating ? "aktualizuję…" : `nowa wersja ${update.latest} — aktualizuj`}
      </button>
    {:else}
      <button class="chip check" onclick={checkUpdate} disabled={checking}
              title="Sprawdź, czy jest nowe wydanie MalleusOS">
        {checking
          ? "sprawdzam…"
          : checkMsg ||
            `MalleusOS${update?.current ? " " + update.current : ""} · sprawdź aktualizacje`}
      </button>
    {/if}
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
    box-shadow: 0 0 10px 2px rgba(167, 139, 250, 0.55);
    animation: pulse 2.2s ease-in-out infinite;
  }
  /* Brak połączenia: dioda gaśnie na bursztynowo i nie pulsuje */
  .led.off {
    background: var(--amber);
    box-shadow: 0 0 8px 2px rgba(251, 191, 36, 0.4);
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

  /* Chip aktualizacji — jedyny klikalny, więc wyróżniony */
  button.chip.update {
    font: inherit;
    font-size: 0.74rem;
    color: #1d1533;
    background: var(--cyan);
    border: none;
    cursor: pointer;
    font-weight: 650;
  }
  button.chip.update:disabled { opacity: 0.6; cursor: wait; }

  /* Chip "sprawdź aktualizacje" — dyskretny, ożywa po najechaniu */
  button.chip.check { font: inherit; font-size: 0.74rem; cursor: pointer; }
  button.chip.check:hover { color: var(--cyan); border-color: rgba(167, 139, 250, 0.4); }
  button.chip.check:disabled { cursor: wait; }

  @media (prefers-reduced-motion: reduce) {
    .led { animation: none; }
  }
</style>
