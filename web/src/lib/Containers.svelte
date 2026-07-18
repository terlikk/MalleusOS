<script>
  // Karta kontenerów: lista ze stanami, przyciski akcji
  // i podgląd logów na żywo (SSE) rozwijany pod wierszem.
  import { getJSON } from "./api.js";

  let available = $state(true);
  let containers = $state([]);
  let busyId = $state(null); // kontener, na którym trwa akcja
  let logsFor = $state(null); // id kontenera z otwartymi logami
  let logLines = $state([]);
  let logBox; // element <pre> do autoprzewijania
  let es = null;

  async function refresh() {
    try {
      const res = await getJSON("/api/v1/containers");
      available = res.available;
      containers = res.containers;
    } catch {
      // np. wylogowanie w międzyczasie — App i tak to obsłuży
    }
  }

  // Pierwsze pobranie + odświeżanie listy co 5 s.
  $effect(() => {
    refresh();
    const t = setInterval(refresh, 5000);
    return () => {
      clearInterval(t);
      es?.close();
    };
  });

  async function action(c, what) {
    busyId = c.id;
    try {
      await fetch(`/api/v1/containers/${c.id}/${what}`, { method: "POST" });
      await refresh();
    } finally {
      busyId = null;
    }
  }

  function toggleLogs(c) {
    es?.close();
    es = null;
    if (logsFor === c.id) {
      logsFor = null;
      return;
    }
    logsFor = c.id;
    logLines = [];
    es = new EventSource(`/api/v1/containers/${c.id}/logs`);
    es.addEventListener("log", (e) => {
      const entry = JSON.parse(e.data);
      // Trzymamy maks. 500 linii, żeby karta nie puchła w nieskończoność.
      logLines = [...logLines.slice(-499), entry];
      // Po dorysowaniu przewiń na dół (requestAnimationFrame czeka
      // aż przeglądarka zaktualizuje widok).
      requestAnimationFrame(() => {
        if (logBox) logBox.scrollTop = logBox.scrollHeight;
      });
    });
  }
</script>

<div class="card">
  <h2>Kontenery</h2>

  {#if !available}
    <p class="empty">
      Docker jest niedostępny — nie znaleziono socketu albo brak
      uprawnień. Panel metryk działa normalnie; ta sekcja obudzi
      się sama, gdy Docker wróci.
    </p>
  {:else if containers.length === 0}
    <p class="empty">Brak kontenerów. Zainstaluj coś z katalogu (wkrótce!).</p>
  {:else}
    <table>
      <thead>
        <tr><th>Nazwa</th><th>Stan</th><th>Obraz</th><th class="right">Akcje</th></tr>
      </thead>
      <tbody>
        {#each containers as c (c.id)}
          <tr>
            <td class="name">{c.name}</td>
            <td>
              <span class="pill" class:run={c.state === "running"} title={c.status}>
                {c.state === "running" ? "działa" : "zatrzymany"}
              </span>
            </td>
            <td class="image mono">{c.image}</td>
            <td>
              <div class="btns">
                {#if c.state === "running"}
                  <button onclick={() => action(c, "stop")} disabled={busyId === c.id}>stop</button>
                  <button onclick={() => action(c, "restart")} disabled={busyId === c.id}>restart</button>
                {:else}
                  <button class="play" onclick={() => action(c, "start")} disabled={busyId === c.id}>start</button>
                {/if}
                <button class:on={logsFor === c.id} onclick={() => toggleLogs(c)}>logi</button>
              </div>
            </td>
          </tr>
          {#if logsFor === c.id}
            <tr class="logrow">
              <td colspan="4">
                <pre bind:this={logBox}>{#each logLines as l}<span
                  class:err={l.stream === "stderr"}>{l.line}
</span>{/each}</pre>
              </td>
            </tr>
          {/if}
        {/each}
      </tbody>
    </table>
  {/if}
</div>

<style>
  table { width: 100%; border-collapse: collapse; font-size: 0.87rem; }
  th {
    text-align: left;
    font-size: 0.68rem;
    font-weight: 600;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--dim);
    padding: 0.35rem 0.4rem 0.6rem;
  }
  th.right { text-align: right; }
  td { padding: 0.6rem 0.4rem; border-top: 1px solid var(--edge); }

  .name { font-weight: 600; }
  .image { font-size: 0.78rem; color: var(--dim); }

  .pill {
    display: inline-block;
    font-size: 0.72rem;
    font-weight: 600;
    padding: 0.16rem 0.66rem;
    border-radius: 999px;
    color: var(--dim);
    background: rgba(143, 160, 182, 0.12);
  }
  .pill.run { color: var(--cyan); background: rgba(167, 139, 250, 0.12); }

  .btns { display: flex; gap: 0.35rem; justify-content: flex-end; }
  .btns button {
    font: inherit;
    font-size: 0.74rem;
    color: var(--dim);
    background: transparent;
    border: 1px solid var(--edge);
    border-radius: 8px;
    padding: 0.26rem 0.7rem;
    cursor: pointer;
  }
  .btns button:hover { color: var(--text); }
  .btns button:disabled { opacity: 0.5; cursor: wait; }
  .btns .play { color: var(--cyan); border-color: rgba(167, 139, 250, 0.35); }
  .btns button.on { color: var(--cyan); border-color: rgba(167, 139, 250, 0.35); }

  .logrow td { padding: 0; border-top: none; }
  pre {
    max-height: 260px;
    overflow: auto;
    background: rgba(10, 8, 20, 0.7);
    border-radius: 10px;
    margin: 0 0 0.6rem;
    padding: 0.8rem 1rem;
    font-family: var(--mono);
    font-size: 0.76rem;
    line-height: 1.7;
    color: var(--text);
  }
  pre .err { color: var(--amber); }
</style>
