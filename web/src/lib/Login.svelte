<script>
  // Ekran logowania i pierwszej konfiguracji w jednym.
  // Tryb wybiera rodzic propsem `setup`:
  //  - setup=true  → ustawianie hasła przy pierwszym starcie
  //  - setup=false → zwykłe logowanie
  let { setup = false, onSuccess } = $props();

  let password = $state("");
  let repeat = $state("");
  let error = $state("");
  let busy = $state(false);

  async function submit(e) {
    e.preventDefault();
    error = "";
    if (setup && password !== repeat) {
      error = "Hasła się różnią.";
      return;
    }
    busy = true;
    try {
      const res = await fetch(setup ? "/api/v1/setup" : "/api/v1/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ password }),
      });
      const body = await res.json();
      if (!res.ok) {
        error = body.error ?? `Błąd ${res.status}`;
        return;
      }
      onSuccess?.();
    } catch {
      error = "Brak połączenia z serwerem.";
    } finally {
      busy = false;
    }
  }
</script>

<div class="gate">
  <form class="card box" onsubmit={submit}>
    <div class="brand"><span class="led"></span> MalleusOS</div>

    {#if setup}
      <h1>Ustaw hasło panelu</h1>
      <p class="hint">
        Pierwsze uruchomienie: wymyśl hasło (min. 8 znaków), którym
        będziesz się logować do panelu.
      </p>
    {:else}
      <h1>Zaloguj się</h1>
    {/if}

    <label>
      Hasło
      <input
        type="password"
        bind:value={password}
        minlength="8"
        required
        autocomplete={setup ? "new-password" : "current-password"}
      />
    </label>

    {#if setup}
      <label>
        Powtórz hasło
        <input type="password" bind:value={repeat} minlength="8" required
               autocomplete="new-password" />
      </label>
    {/if}

    {#if error}<p class="error" role="alert">{error}</p>{/if}

    <button type="submit" disabled={busy}>
      {busy ? "chwila…" : setup ? "Zapisz i wejdź" : "Zaloguj"}
    </button>
  </form>
</div>

<style>
  .gate {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 1.5rem;
  }

  .box {
    width: min(380px, 100%);
    display: flex;
    flex-direction: column;
    gap: 1rem;
    padding: 2rem;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-weight: 650;
    color: var(--dim);
    font-size: 0.9rem;
  }
  .led {
    width: 8px; height: 8px; border-radius: 50%;
    background: var(--cyan);
    box-shadow: 0 0 8px 2px rgba(167, 139, 250, 0.5);
  }

  h1 { font-size: 1.25rem; letter-spacing: -0.01em; }
  .hint { font-size: 0.85rem; color: var(--dim); }

  label {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    font-size: 0.8rem;
    color: var(--dim);
  }
  input {
    font: inherit;
    color: var(--text);
    background: rgba(13, 11, 26, 0.6);
    border: 1px solid var(--edge);
    border-radius: 8px;
    padding: 0.6rem 0.8rem;
  }
  input:focus-visible { outline: 2px solid var(--cyan); outline-offset: 2px; }

  .error { color: var(--red); font-size: 0.85rem; }

  button {
    font: inherit;
    font-weight: 650;
    color: #1d1533;
    background: var(--cyan);
    border: none;
    border-radius: 999px;
    padding: 0.7rem;
    cursor: pointer;
  }
  button:disabled { opacity: 0.6; cursor: wait; }
</style>
