<script>
  // "Moje projekty" — wdrażanie własnych aplikacji.
  // Formularz wysyła multipart POST, a odpowiedź NIE jest zwykłym
  // JSON-em, tylko strumieniem tekstu: czytamy go kawałek po
  // kawałku (ReadableStream) i dopisujemy do loga na ekranie —
  // użytkownik ogląda budowanie na żywo.
  import { getJSON } from "./api.js";

  let projects = $state([]);
  let building = $state(false);
  let logText = $state("");
  let logBox; // <pre> do autoprzewijania
  let error = $state("");

  // pola formularza
  let name = $state("");
  let gitUrl = $state("");
  let zipFile = $state(null);
  let containerPort = $state("");
  let hostPort = $state("");

  async function refresh() {
    try {
      projects = await getJSON("/api/v1/projects");
    } catch {
      // API chwilowo niedostępne
    }
  }

  $effect(() => {
    refresh();
    const t = setInterval(refresh, 7000);
    return () => clearInterval(t);
  });

  async function deploy(e) {
    e.preventDefault();
    error = "";
    logText = "";
    building = true;

    const fd = new FormData();
    fd.set("name", name.trim());
    fd.set("containerPort", containerPort);
    if (hostPort) fd.set("hostPort", hostPort);
    if (gitUrl.trim()) fd.set("gitUrl", gitUrl.trim());
    if (zipFile?.[0]) fd.set("zip", zipFile[0]);

    try {
      const res = await fetch("/api/v1/projects", { method: "POST", body: fd });
      if (!res.ok && res.headers.get("Content-Type")?.includes("json")) {
        error = (await res.json()).error;
        return;
      }
      // Czytamy strumień logu budowania na bieżąco.
      const reader = res.body.getReader();
      const dec = new TextDecoder();
      for (;;) {
        const { done, value } = await reader.read();
        if (done) break;
        logText += dec.decode(value, { stream: true });
        requestAnimationFrame(() => {
          if (logBox) logBox.scrollTop = logBox.scrollHeight;
        });
      }
      await refresh();
    } catch {
      error = "brak połączenia z serwerem";
    } finally {
      building = false;
    }
  }

  async function remove(p) {
    if (!confirm(`Usunąć projekt ${p.name} (kontener i obraz)?`)) return;
    await fetch(`/api/v1/projects/${p.name}`, { method: "DELETE" });
    await refresh();
  }

  function urlFor(p) {
    const host = window.location.hostname;
    if (host.endsWith("malleus.local")) return `http://${p.name}.malleus.local`;
    return `http://${host}:${p.hostPort}`;
  }
</script>

<div class="card">
  <div class="head">
    <h2>Moje projekty</h2>
    <span class="hint">repozytorium git albo ZIP z plikiem Dockerfile — resztą zajmie się serwer</span>
  </div>

  {#if projects.length > 0}
    <table>
      <thead>
        <tr><th>Nazwa</th><th>Stan</th><th>Adres</th><th class="right">Akcje</th></tr>
      </thead>
      <tbody>
        {#each projects as p (p.name)}
          <tr>
            <td class="name">{p.name}</td>
            <td>
              <span class="pill" class:run={p.state === "running"}>
                {p.state === "running" ? "działa" : p.state === "" ? "brak kontenera" : "zatrzymany"}
              </span>
            </td>
            <td class="mono addr">{p.name}.malleus.local</td>
            <td>
              <div class="btns">
                <a class="open" href={urlFor(p)} target="_blank" rel="noopener">Otwórz</a>
                <button class="del" onclick={() => remove(p)}>usuń</button>
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}

  <form onsubmit={deploy}>
    <div class="fields">
      <label>
        Nazwa projektu
        <input bind:value={name} required pattern="[a-z0-9][a-z0-9-]+"
               placeholder="mojastrona" title="małe litery, cyfry, myślniki" />
      </label>
      <label>
        Adres repozytorium git
        <input bind:value={gitUrl} placeholder="https://github.com/ty/projekt.git" />
      </label>
      <label>
        …albo plik ZIP
        <input type="file" accept=".zip" bind:files={zipFile} />
      </label>
      <label>
        Port aplikacji w kontenerze
        <input bind:value={containerPort} required type="number" min="1" max="65535"
               placeholder="3000" />
      </label>
      <label>
        Port na serwerze <small>(puste = ten sam)</small>
        <input bind:value={hostPort} type="number" min="1" max="65535" />
      </label>
    </div>

    {#if error}<p class="error" role="alert">{error}</p>{/if}

    <button class="go" type="submit" disabled={building}>
      {building ? "buduję…" : "Zbuduj i uruchom"}
    </button>
  </form>

  {#if logText}
    <pre bind:this={logBox}>{logText}</pre>
  {/if}
</div>

<style>
  .head { display: flex; align-items: baseline; flex-wrap: wrap; gap: 0.5rem; }
  .hint { margin-left: auto; font-size: 0.78rem; color: var(--dim); }

  table { width: 100%; border-collapse: collapse; font-size: 0.87rem; margin: 0.4rem 0 1.2rem; }
  th {
    text-align: left; font-size: 0.68rem; font-weight: 600;
    letter-spacing: 0.1em; text-transform: uppercase; color: var(--dim);
    padding: 0.35rem 0.4rem 0.6rem;
  }
  th.right { text-align: right; }
  td { padding: 0.6rem 0.4rem; border-top: 1px solid var(--edge); }
  .name { font-weight: 600; }
  .addr { font-size: 0.78rem; color: var(--cyan); }

  .pill {
    display: inline-block; font-size: 0.72rem; font-weight: 600;
    padding: 0.16rem 0.66rem; border-radius: 999px;
    color: var(--dim); background: rgba(157, 149, 184, 0.12);
  }
  .pill.run { color: var(--cyan); background: rgba(167, 139, 250, 0.12); }

  .btns { display: flex; gap: 0.35rem; justify-content: flex-end; }
  .open {
    font-size: 0.78rem; font-weight: 600; color: var(--cyan);
    text-decoration: none;
    border: 1px solid rgba(167, 139, 250, 0.35);
    border-radius: 999px; padding: 0.3rem 0.9rem;
  }
  .del {
    font: inherit; font-size: 0.74rem; color: var(--dim);
    background: none; border: 1px solid var(--edge);
    border-radius: 999px; padding: 0.3rem 0.8rem; cursor: pointer;
  }
  .del:hover { color: var(--red); border-color: var(--red); }

  .fields {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 0.9rem;
    margin-top: 0.6rem;
  }
  label {
    display: flex; flex-direction: column; gap: 0.3rem;
    font-size: 0.78rem; color: var(--dim);
  }
  label small { color: var(--dim); opacity: 0.7; }
  input {
    font: inherit; color: var(--text);
    background: rgba(13, 11, 26, 0.6);
    border: 1px solid var(--edge); border-radius: 8px;
    padding: 0.5rem 0.7rem;
  }
  input[type="file"] { padding: 0.4rem; font-size: 0.78rem; }
  input:focus-visible { outline: 2px solid var(--cyan); outline-offset: 2px; }

  .error { color: var(--red); font-size: 0.85rem; margin-top: 0.7rem; }

  .go {
    margin-top: 1rem;
    font: inherit; font-size: 0.85rem; font-weight: 650;
    color: #1d1533; background: var(--cyan);
    border: none; border-radius: 999px;
    padding: 0.55rem 1.6rem; cursor: pointer;
  }
  .go:disabled { opacity: 0.6; cursor: wait; }

  pre {
    margin-top: 1rem;
    max-height: 300px; overflow: auto;
    background: rgba(10, 8, 20, 0.7);
    border-radius: 10px; padding: 0.9rem 1rem;
    font-family: var(--mono); font-size: 0.76rem; line-height: 1.7;
    color: var(--text); white-space: pre-wrap;
  }
</style>
