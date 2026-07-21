<script>
  // Katalog aplikacji: kafelki z przyciskiem "Zainstaluj".
  // Instalacja to jedno żądanie POST — serwer pobiera obraz,
  // tworzy kontener i startuje; w tym czasie kafelek pokazuje
  // "instalowanie…". Po sukcesie pojawia się "Otwórz".
  import { getJSON } from "./api.js";

  let available = $state(true);
  let apps = $state([]);
  let busy = $state(null); // id aplikacji w trakcie akcji
  let busyText = $state("instalowanie…");
  let errors = $state({}); // błędy per aplikacja
  let notes = $state({}); // komunikaty sukcesu (np. wynik aktualizacji)
  let gateway = $state(""); // adres routera — do linku przy Pi-hole

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
    getJSON("/api/v1/system")
      .then((s) => (gateway = s.gatewayIp ?? ""))
      .catch(() => {});
  });

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

  // Aktualizacja: pobierz nowy obraz i przelej kontener
  // (dane i ustawienia zostają — patrz backend).
  async function update(app) {
    busy = app.id;
    busyText = "aktualizowanie…";
    errors = { ...errors, [app.id]: null };
    notes = { ...notes, [app.id]: null };
    try {
      const res = await fetch(`/api/v1/catalog/${app.id}/update`, { method: "POST" });
      const body = await res.json();
      if (!res.ok) errors = { ...errors, [app.id]: body.error };
      else notes = { ...notes, [app.id]: body.message };
      await refresh();
    } catch {
      errors = { ...errors, [app.id]: "brak połączenia z serwerem" };
    } finally {
      busy = null;
    }
  }

  // Czyta strumień statusów z serwera linia po linii ("pobieranie
  // obrazu 47%"…) i pokazuje każdą na kafelku. Zwraca true, gdy
  // operacja skończyła się bez błędu.
  async function readStatuses(res, app) {
    const reader = res.body.getReader();
    const dec = new TextDecoder();
    let buf = "";
    let failed = false;
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += dec.decode(value, { stream: true });
      const lines = buf.split("\n");
      buf = lines.pop(); // niedokończona linia czeka na resztę
      for (const ln of lines) {
        if (!ln.trim() || ln === "OK") continue;
        if (ln.startsWith("BŁĄD: ")) {
          errors = { ...errors, [app.id]: ln.slice(6) };
          failed = true;
        } else {
          busyText = ln;
        }
      }
    }
    return !failed;
  }

  async function install(app) {
    busy = app.id;
    busyText = "instalowanie…";
    errors = { ...errors, [app.id]: null };
    try {
      const res = await fetch(`/api/v1/catalog/${app.id}/install`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ env: formValues }),
      });
      if (res.headers.get("content-type")?.includes("json")) {
        // błąd walidacji (np. puste pole "pytaj") — zwykły JSON
        const body = await res.json();
        errors = { ...errors, [app.id]: body.error };
      } else if (await readStatuses(res, app)) {
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

  // Własny szablon: formularz z prostymi polami tekstowymi,
  // serwer robi z nich zwykły szablon YAML w katalogu danych.
  let customOpen = $state(false);
  let customBusy = $state(false);
  let customError = $state("");
  let cf = $state({ name: "", tagline: "", image: "", webPort: "", ports: "", volumes: "", env: "" });

  async function customCreate() {
    customBusy = true;
    customError = "";
    // Pola-listy rozdzielamy przecinkami (zmienne też enterem)
    const list = (s, sep = /[,\n]/) => s.split(sep).map((x) => x.trim()).filter(Boolean);
    try {
      const res = await fetch("/api/v1/catalog/custom", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: cf.name,
          tagline: cf.tagline,
          image: cf.image,
          webPort: Number(cf.webPort) || 0,
          ports: list(cf.ports),
          volumes: list(cf.volumes),
          env: list(cf.env),
        }),
      });
      const body = await res.json();
      if (!res.ok) {
        customError = body.error;
      } else {
        customOpen = false;
        cf = { name: "", tagline: "", image: "", webPort: "", ports: "", volumes: "", env: "" };
        await refresh();
      }
    } catch {
      customError = "brak połączenia z serwerem";
    } finally {
      customBusy = false;
    }
  }

  // "Dodaj treść" (np. paczki .zim Kiwiksa): serwer pobiera plik
  // prosto do wolumenu aplikacji i restartuje ją — zero terminala.
  let filesId = $state(null);
  let fileUrl = $state("");

  async function addFiles(app, url) {
    busy = app.id;
    busyText = "pobieranie…";
    errors = { ...errors, [app.id]: null };
    notes = { ...notes, [app.id]: null };
    try {
      const res = await fetch(`/api/v1/catalog/${app.id}/files`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url }),
      });
      if (res.headers.get("content-type")?.includes("json")) {
        const body = await res.json();
        errors = { ...errors, [app.id]: body.error };
      } else if (await readStatuses(res, app)) {
        filesId = null;
        fileUrl = "";
        notes = { ...notes, [app.id]: "treść dodana — aplikacja działa ✓" };
      }
      await refresh();
    } catch {
      errors = { ...errors, [app.id]: "brak połączenia z serwerem" };
    } finally {
      busy = null;
    }
  }

  async function customDelete(app) {
    if (!confirm(`Usunąć szablon ${app.name} z katalogu?`)) return;
    busy = app.id;
    try {
      const res = await fetch(`/api/v1/catalog/custom/${app.id}`, { method: "DELETE" });
      const body = await res.json();
      if (!res.ok) errors = { ...errors, [app.id]: body.error };
      await refresh();
    } finally {
      busy = null;
    }
  }
</script>

{#snippet appRow(app)}
  <div class="app" class:open-form={formId === app.id}>
    <div class="row">
      <div class="ico" class:game={app.kategoria === "gry"} aria-hidden="true">{app.name[0]}</div>
      <div class="info">
        <b>{app.name}{#if app.custom}<span class="badge">własna</span>{/if}</b>
        <span>{app.tagline}</span>
        {#if app.installed && app.subdomain && app.webPort > 0}
          <span class="addr mono">{app.subdomain}.malleus.local</span>
        {/if}
        {#if app.installed && app.connect}
          <span class="addr mono" title="Ten adres znajomi wpisują w grze">
            adres w grze: {connectAddr(app)}
          </span>
        {/if}
        {#if app.installed && app.hint}
          <span class="hintline">
            {app.hint.replace("HOST", window.location.hostname)}
            {#if gateway && app.hint.includes("routera")}
              <a class="router-link" href={"http://" + gateway} target="_blank" rel="noopener">
                otwórz panel routera ({gateway}) →
              </a>
            {/if}
          </span>
        {/if}
        {#if errors[app.id]}
          <span class="error">{errors[app.id]}</span>
        {/if}
        {#if notes[app.id]}
          <span class="addr">{notes[app.id]}</span>
        {/if}
      </div>
      <div class="act">
        {#if busy === app.id}
          <span class="busy">{busyText}</span>
        {:else if app.installed}
          {#if app.webPort > 0}
            <a class="open" href={urlFor(app)} target="_blank" rel="noopener">Otwórz</a>
          {/if}
          {#if app.pliki}
            <button class="del neutral"
                    onclick={() => (filesId = filesId === app.id ? null : app.id)}
                    title={app.pliki.opis}>dodaj treść</button>
          {/if}
          <button class="del" onclick={() => update(app)}
                  title="Pobierz najnowszą wersję — dane zostają">aktualizuj</button>
          <a class="del" href={`/api/v1/catalog/${app.id}/backup`}
             title="Pobierz kopię zapasową danych (tar.gz)">kopia</a>
          <button class="del" onclick={() => uninstall(app)} title="Usuń aplikację">usuń</button>
        {:else if available}
          <button class="install" onclick={() => startInstall(app)}>Zainstaluj</button>
          {#if app.custom}
            <button class="del" onclick={() => customDelete(app)}
                    title="Usuń szablon z katalogu">usuń szablon</button>
          {/if}
        {/if}
      </div>
    </div>

    {#if filesId === app.id && busy !== app.id}
      <div class="ask">
        <span class="files-opis">{app.pliki.opis}</span>
        {#each app.pliki.propozycje ?? [] as prop (prop.url)}
          <button class="propo" onclick={() => addFiles(app, prop.url)}>
            ⬇ {prop.nazwa}
          </button>
        {/each}
        <label>
          własny link do pliku {app.pliki.ext}
          <input bind:value={fileUrl} placeholder={"https://…" + app.pliki.ext} />
        </label>
        <div class="ask-actions">
          <button class="install" onclick={() => addFiles(app, fileUrl)}>Pobierz</button>
          <button class="cancel" onclick={() => (filesId = null)}>anuluj</button>
        </div>
      </div>
    {/if}

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
    <button class="add-custom" onclick={() => (customOpen = !customOpen)}>
      {customOpen ? "zwiń formularz" : "+ dodaj własną"}
    </button>
  </div>

  {#if !available}
    <p class="empty">Docker jest niedostępny — instalacja aplikacji poczeka, aż wróci.</p>
  {/if}

  {#if customOpen}
    <!-- Formularz własnego szablonu: wystarczą nazwa i obraz,
         reszta opcjonalna. Serwer zapisuje z tego YAML. -->
    <div class="custom-form">
      <p class="cf-intro">
        Dodaj dowolną aplikację z
        <a href="https://hub.docker.com" target="_blank" rel="noopener">Docker Huba</a>
        — wystarczy nazwa i obraz. Po dodaniu instalujesz ją jak każdą inną,
        z aktualizacjami i kopią zapasową w zestawie.
      </p>
      <div class="cf-grid">
        <label>
          nazwa *
          <input bind:value={cf.name} placeholder="np. Moja Strona" />
        </label>
        <label>
          obraz Dockera *
          <input bind:value={cf.image} placeholder="np. nginx:latest" />
        </label>
        <label>
          opis (jedno zdanie)
          <input bind:value={cf.tagline} placeholder="np. strona domowa na nginx" />
        </label>
        <label>
          port WWW <span class="cf-note">(jeśli apka ma panel w przeglądarce)</span>
          <input bind:value={cf.webPort} placeholder="np. 8080" inputmode="numeric" />
        </label>
        <label>
          dodatkowe porty <span class="cf-note">(serwer:kontener, po przecinku)</span>
          <input bind:value={cf.ports} placeholder="np. 8080:80, 25565:25565/udp" />
        </label>
        <label>
          foldery na dane <span class="cf-note">(ścieżki w kontenerze, po przecinku)</span>
          <input bind:value={cf.volumes} placeholder="np. /data, /config" />
        </label>
        <label class="cf-wide">
          zmienne środowiskowe <span class="cf-note">(NAZWA=wartość, po przecinku)</span>
          <input bind:value={cf.env} placeholder="np. TZ=Europe/Warsaw, HASLO=sekret" />
        </label>
      </div>
      {#if customError}
        <span class="error">{customError}</span>
      {/if}
      <div class="ask-actions">
        <button class="install" disabled={customBusy} onclick={customCreate}>
          {customBusy ? "dodawanie…" : "Dodaj do katalogu"}
        </button>
        <button class="cancel" onclick={() => (customOpen = false)}>anuluj</button>
      </div>
    </div>
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
    /* Zainstalowana aplikacja ma aż 4 przyciski — gdy się nie
       mieszczą obok opisu, cały pasek akcji spada do nowej linii
       zamiast zgniatać tekst. */
    flex-wrap: wrap;
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

  /* min-width trzyma opisowi sensowną szerokość — poniżej niej
     przyciski akcji zawijają się pod spód, a tekst zostaje czytelny */
  .info { min-width: 11rem; flex: 1; line-height: 1.35; display: flex; flex-direction: column; }
  .info b { font-size: 0.9rem; white-space: nowrap; }
  .info span { font-size: 0.76rem; color: var(--dim); }
  .info .error { color: var(--red); }
  .info .addr { font-size: 0.72rem; color: var(--cyan); opacity: 0.85; }
  .info .hintline { font-size: 0.74rem; color: var(--amber); margin-top: 0.2rem; }
  .router-link { color: var(--cyan); font-weight: 600; }

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

  .act {
    margin-left: auto;
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-end;
    gap: 0.4rem;
    align-items: center;
  }

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
    text-decoration: none;
    white-space: nowrap;
  }
  .del:hover { color: var(--red); border-color: var(--red); }
  /* aktualizuj/kopia/dodaj treść to nie akcje niszczące — hover fioletowy */
  button.del[title^="Pobierz naj"]:hover,
  button.del.neutral:hover,
  a.del:hover { color: var(--cyan); border-color: rgba(167, 139, 250, 0.4); }

  /* sekcja "dodaj treść": gotowe paczki + pole na własny link */
  .files-opis { font-size: 0.74rem; color: var(--dim); }
  .propo {
    font: inherit;
    font-size: 0.8rem;
    text-align: left;
    color: var(--cyan);
    background: rgba(167, 139, 250, 0.08);
    border: 1px solid rgba(167, 139, 250, 0.3);
    border-radius: 10px;
    padding: 0.55rem 0.85rem;
    cursor: pointer;
  }
  .propo:hover { background: rgba(167, 139, 250, 0.16); }

  .busy { font-size: 0.8rem; color: var(--amber); }

  /* przycisk "+ dodaj własną" w nagłówku karty */
  .add-custom {
    font: inherit;
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--cyan);
    background: none;
    border: 1px solid rgba(167, 139, 250, 0.35);
    border-radius: 999px;
    padding: 0.3rem 0.9rem;
    cursor: pointer;
    white-space: nowrap;
  }
  .add-custom:hover { background: rgba(167, 139, 250, 0.1); }

  /* plakietka odróżniająca szablony użytkownika od wbudowanych */
  .badge {
    margin-left: 0.45rem;
    font-size: 0.62rem;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--cyan);
    border: 1px solid rgba(167, 139, 250, 0.35);
    border-radius: 999px;
    padding: 0.08rem 0.45rem;
    vertical-align: middle;
  }

  .custom-form {
    margin-top: 0.7rem;
    padding: 1rem;
    border: 1px solid rgba(167, 139, 250, 0.3);
    border-radius: 14px;
    background: rgba(13, 11, 26, 0.45);
    display: flex;
    flex-direction: column;
    gap: 0.8rem;
  }
  .cf-intro { font-size: 0.8rem; color: var(--dim); line-height: 1.5; }
  .cf-intro a { color: var(--cyan); }
  .cf-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    gap: 0.7rem;
  }
  .cf-grid label {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    font-size: 0.76rem;
    color: var(--dim);
  }
  .cf-wide { grid-column: 1 / -1; }
  .cf-note { font-size: 0.68rem; opacity: 0.75; }
  .cf-grid input {
    font: inherit;
    color: var(--text);
    background: rgba(13, 11, 26, 0.7);
    border: 1px solid var(--edge);
    border-radius: 8px;
    padding: 0.5rem 0.7rem;
  }
  .cf-grid input:focus-visible { outline: 2px solid var(--cyan); outline-offset: 2px; }
  .custom-form .error { font-size: 0.76rem; color: var(--red); }
  .install:disabled { opacity: 0.6; cursor: default; }
</style>
