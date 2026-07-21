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

  // Aplikacja z polami "pytaj" (np. klucz playit.gg) najpierw
  // rozwija mini-formularz; formId mówi, który kafelek jest otwarty.
  let formId = $state(null);
  let formValues = $state({});

  function askable(app) {
    return (app.env ?? []).filter((e) => e.pytaj);
  }

  function startInstall(app) {
    if (askable(app).length > 0 && formId !== app.id) {
      formId = app.id;
      formValues = Object.fromEntries(askable(app).map((e) => [e.name, e.value]));
      return;
    }
    install(app);
  }

  async function install(app) {
    busy = app.id;
    errors = { ...errors, [app.id]: null };
    try {
      const res = await fetch(`/api/v1/catalog/${app.id}/install`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ env: formValues }),
      });
      const body = await res.json();
      if (!res.ok) {
        errors = { ...errors, [app.id]: body.error };
      } else {
        formId = null;
        formValues = {};
      }
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

  // Adres do wklejenia w grze: szablon "HOST:25565" z podmienionym
  // HOST na adres serwera (ten, pod którym otwarto panel).
  function connectAddr(app) {
    return app.connect?.replace("HOST", window.location.hostname) ?? "";
  }

  // Podział na sekcje: zwykłe aplikacje i serwery gier.
  let zwykle = $derived(apps.filter((a) => a.kategoria !== "gry"));
  let gry = $derived(apps.filter((a) => a.kategoria === "gry"));
</script>

{#snippet appRow(app)}
  <div class="app" class:open-form={formId === app.id}>
    <div class="row">
      <div class="ico" class:game={app.kategoria === "gry"} aria-hidden="true">{app.name[0]}</div>
      <div class="info">
        <b>{app.name}</b>
        <span>{app.tagline}</span>
        {#if app.installed && app.subdomain && app.webPort > 0}
          <span class="addr mono">{app.subdomain}.malleus.local</span>
        {/if}
        {#if app.installed && app.connect}
          <span class="addr mono" title="Ten adres znajomi wpisują w grze">
            adres w grze: {connectAddr(app)}
          </span>
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
          <button class="install" onclick={() => startInstall(app)}>Zainstaluj</button>
        {/if}
      </div>
    </div>

    {#if formId === app.id && busy !== app.id}
      <div class="ask">
        {#each askable(app) as e (e.name)}
          <label>
            {e.opis}
            <input bind:value={formValues[e.name]} placeholder={e.name} />
          </label>
        {/each}
        <div class="ask-actions">
          <button class="install" onclick={() => install(app)}>Instaluj</button>
          <button class="cancel" onclick={() => (formId = null)}>anuluj</button>
        </div>
      </div>
    {/if}
  </div>
{/snippet}

<div class="card">
  <div class="head">
    <h2>Katalog aplikacji</h2>
    <span class="hint">jedno kliknięcie — MalleusOS sam pobierze i skonfiguruje</span>
  </div>

  {#if !available}
    <p class="empty">Docker jest niedostępny — instalacja aplikacji poczeka, aż wróci.</p>
  {/if}

  <div class="apps">
    {#each zwykle as app (app.id)}
      {@render appRow(app)}
    {/each}
  </div>

  {#if gry.length > 0}
    <div class="games-head">
      <h3>Serwery gier</h3>
      <span class="hint">postaw serwer i podeślij znajomym adres — dołączą z gry</span>
    </div>
    <div class="apps">
      {#each gry as app (app.id)}
        {@render appRow(app)}
      {/each}
    </div>
  {/if}
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
    background: rgba(13, 11, 26, 0.45);
    border: 1px solid var(--edge);
    border-radius: 14px;
    padding: 0.9rem 1rem;
  }
  .app.open-form { border-color: rgba(167, 139, 250, 0.35); }

  .row {
    display: flex;
    align-items: center;
    gap: 0.9rem;
  }

  /* mini-formularz pól "pytaj" rozwijany pod wierszem */
  .ask {
    margin-top: 0.9rem;
    padding-top: 0.9rem;
    border-top: 1px solid var(--edge);
    display: flex;
    flex-direction: column;
    gap: 0.7rem;
  }
  .ask label {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    font-size: 0.76rem;
    color: var(--dim);
  }
  .ask input {
    font: inherit;
    color: var(--text);
    background: rgba(13, 11, 26, 0.7);
    border: 1px solid var(--edge);
    border-radius: 8px;
    padding: 0.5rem 0.7rem;
  }
  .ask input:focus-visible { outline: 2px solid var(--cyan); outline-offset: 2px; }
  .ask-actions { display: flex; gap: 0.5rem; }
  .cancel {
    font: inherit;
    font-size: 0.78rem;
    color: var(--dim);
    background: none;
    border: none;
    cursor: pointer;
  }
  .cancel:hover { color: var(--text); }

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

  .games-head {
    display: flex;
    align-items: baseline;
    gap: 0.5rem;
    flex-wrap: wrap;
    margin-top: 1.4rem;
    padding-top: 1.1rem;
    border-top: 1px solid var(--edge);
  }
  .games-head h3 {
    font-size: 0.72rem;
    font-weight: 600;
    letter-spacing: 0.1em;
    text-transform: uppercase;
    color: var(--dim);
  }
  .games-head .hint { margin-left: auto; }

  /* ikonki gier na bursztynowo — od razu widać inną kategorię */
  .ico.game {
    background: rgba(251, 191, 36, 0.1);
    color: var(--amber);
    border-color: rgba(251, 191, 36, 0.25);
  }

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
