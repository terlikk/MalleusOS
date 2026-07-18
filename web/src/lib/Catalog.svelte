<script>
  // Katalog aplikacji: kafelki z przyciskiem "Zainstaluj".
  // Instalacja to jedno żądanie POST — serwer pobiera obraz,
  // tworzy kontener i startuje; w tym czasie kafelek pokazuje
  // "instalowanie…". Po sukcesie pojawia się "Otwórz".
  import { getJSON } from "./api.js";

  let available = $state(true);
  let apps = $state([]);
  let busy = $state(null); // id aplikacji w trakcie instalacji/usuwania
  let errors = $state({}); // błędy per aplikacja

  async function refresh() {
    try {
      const res = await getJSON("/api/v1/catalog");
      available = res.available;
      apps = res.apps;
    } catch {
      // chwilowy brak API — spróbujemy przy następnym odświeżeniu
    }
  }

  $effect(() => {
    refresh();
    const t = setInterval(refresh, 7000);
    return () => clearInterval(t);
  });

  async function install(app) {
    busy = app.id;
    errors = { ...errors, [app.id]: null };
    try {
      const res = await fetch(`/api/v1/catalog/${app.id}/install`, { method: "POST" });
      const body = await res.json();
      if (!res.ok) errors = { ...errors, [app.id]: body.error };
      await refresh();
    } catch {
      errors = { ...errors, [app.id]: "brak połączenia z serwerem" };
    } finally {
      busy = null;
    }
  }

  async function uninstall(app) {
    if (!confirm(`Usunąć ${app.name}? Dane (wolumeny) zostaną zachowane.`)) return;
    busy = app.id;
    try {
      await fetch(`/api/v1/catalog/${app.id}/uninstall`, { method: "POST" });
      await refresh();
    } finally {
      busy = null;
    }
  }

  // Adres aplikacji. Jeśli panel otwarto przez malleus.local,
  // linkujemy do ładnej subdomeny (filmy.malleus.local — reverse
  // proxy w binarce). W innym razie (wejście po IP) — port wprost,
  // bo skoro mDNS nie zadziałał dla panelu, dla apki też może nie.
  function urlFor(app) {
    const host = window.location.hostname;
    if (host.endsWith("malleus.local") && app.subdomain) {
      return `http://${app.subdomain}.malleus.local`;
    }
    return `http://${host}:${app.webPort}`;
  }
</script>

<div class="card">
  <div class="head">
    <h2>Katalog aplikacji</h2>
    <span class="hint">jedno kliknięcie — MalleusOS sam pobierze i skonfiguruje</span>
  </div>

  {#if !available}
    <p class="empty">Docker jest niedostępny — instalacja aplikacji poczeka, aż wróci.</p>
  {/if}

  <div class="apps">
    {#each apps as app (app.id)}
      <div class="app">
        <div class="ico" aria-hidden="true">{app.name[0]}</div>
        <div class="info">
          <b>{app.name}</b>
          <span>{app.tagline}</span>
          {#if app.installed && app.subdomain && app.webPort > 0}
            <span class="addr mono">{app.subdomain}.malleus.local</span>
          {/if}
          {#if errors[app.id]}
            <span class="error">{errors[app.id]}</span>
          {/if}
        </div>
        <div class="act">
          {#if busy === app.id}
            <span class="busy">instalowanie…</span>
          {:else if app.installed}
            {#if app.webPort > 0}
              <a class="open" href={urlFor(app)} target="_blank" rel="noopener">Otwórz</a>
            {/if}
            <button class="del" onclick={() => uninstall(app)} title="Usuń aplikację">usuń</button>
          {:else if available}
            <button class="install" onclick={() => install(app)}>Zainstaluj</button>
          {/if}
        </div>
      </div>
    {/each}
  </div>
</div>

<style>
  .head { display: flex; align-items: baseline; flex-wrap: wrap; gap: 0.5rem; }
  .hint { margin-left: auto; font-size: 0.78rem; color: var(--dim); }

  .apps {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(380px, 1fr));
    gap: 0.9rem;
    margin-top: 0.4rem;
  }

  .app {
    display: flex;
    align-items: center;
    gap: 0.9rem;
    background: rgba(13, 11, 26, 0.45);
    border: 1px solid var(--edge);
    border-radius: 14px;
    padding: 0.9rem 1rem;
  }

  .ico {
    flex: none;
    width: 44px;
    height: 44px;
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.15rem;
    font-weight: 700;
    background: rgba(167, 139, 250, 0.1);
    color: var(--cyan);
    border: 1px solid rgba(167, 139, 250, 0.22);
  }

  .info { min-width: 0; flex: 1; line-height: 1.35; display: flex; flex-direction: column; }
  .info b { font-size: 0.9rem; white-space: nowrap; }
  .info span { font-size: 0.76rem; color: var(--dim); }
  .info .error { color: var(--red); }
  .info .addr { font-size: 0.72rem; color: var(--cyan); opacity: 0.85; }

  .act { margin-left: auto; flex: none; display: flex; gap: 0.4rem; align-items: center; }

  .install {
    font: inherit;
    font-size: 0.8rem;
    font-weight: 650;
    color: #1d1533;
    background: var(--cyan);
    border: none;
    border-radius: 999px;
    padding: 0.42rem 1.05rem;
    cursor: pointer;
  }
  .install:hover { opacity: 0.9; }

  .open {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--cyan);
    text-decoration: none;
    border: 1px solid rgba(167, 139, 250, 0.35);
    border-radius: 999px;
    padding: 0.4rem 1.05rem;
  }

  .del {
    font: inherit;
    font-size: 0.74rem;
    color: var(--dim);
    background: none;
    border: 1px solid var(--edge);
    border-radius: 999px;
    padding: 0.4rem 0.8rem;
    cursor: pointer;
  }
  .del:hover { color: var(--red); border-color: var(--red); }

  .busy { font-size: 0.8rem; color: var(--amber); }
</style>
