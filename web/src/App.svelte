<script>
  // Główny komponent panelu: trzyma stan (info o systemie,
  // najnowsza próbka, status połączenia) i rozdaje go kartom.
  import Header from "./lib/Header.svelte";
  import StatCard from "./lib/StatCard.svelte";
  import LiveChart from "./lib/LiveChart.svelte";
  import Disks from "./lib/Disks.svelte";
  import Temps from "./lib/Temps.svelte";
  import Containers from "./lib/Containers.svelte";
  import Catalog from "./lib/Catalog.svelte";
  import Login from "./lib/Login.svelte";
  import { getJSON, streamMetrics } from "./lib/api.js";
  import { bytes, bps, rdzenie } from "./lib/format.js";

  // $state to reaktywny stan Svelte 5 — zmiana wartości
  // automatycznie odświeża wszystko, co z niej korzysta.
  let system = $state(null);
  let sample = $state(null);
  let online = $state(false);

  // Bramka logowania: zanim pokażemy panel, pytamy serwer,
  // czy hasło jest ustawione i czy mamy ważną sesję.
  let auth = $state(null); // null = jeszcze sprawdzamy

  async function checkAuth() {
    try {
      auth = await getJSON("/api/v1/auth/status");
    } catch {
      // serwer nie odpowiada — spróbujemy ponownie za chwilę
      setTimeout(checkAuth, 2000);
    }
  }

  // $effect uruchamia się po zamontowaniu komponentu;
  // zwracana funkcja sprząta (zamyka strumień) przy odmontowaniu.
  $effect(() => {
    checkAuth();
  });

  // Dane pobieramy dopiero PO zalogowaniu — inaczej dostalibyśmy
  // same odpowiedzi 401.
  $effect(() => {
    if (!auth?.authenticated) return;
    getJSON("/api/v1/system").then((s) => (system = s)).catch(() => {});
    const stop = streamMetrics(
      (s) => (sample = s),
      (ok) => (online = ok),
    );
    return stop;
  });

  // $derived liczy wartości pochodne z próbki.
  let maxTemp = $derived(
    sample?.temps?.length
      ? Math.max(...sample.temps.map((t) => t.celsius))
      : null,
  );
  // "Główny" dysk do karty u góry — ten o największej pojemności.
  let mainDisk = $derived(
    sample?.disks?.length
      ? sample.disks.reduce((a, b) => (b.totalBytes > a.totalBytes ? b : a))
      : null,
  );

  const GB = 1024 * 1024; // kB → GB (pamięć raportujemy w kB)
</script>

{#if auth == null}
  <!-- jeszcze nie wiemy, czy trzeba się logować -->
{:else if !auth.setupDone}
  <Login setup onSuccess={checkAuth} />
{:else if !auth.authenticated}
  <Login onSuccess={checkAuth} />
{:else}
<div class="layout">
  <Header {system} {online} />

  <div class="grid-stats">
    <StatCard
      label="Procesor"
      value={sample?.cpu?.usagePercent}
      line1={rdzenie(sample?.cpu?.cores)}
      line2={`obciążenie ${sample?.cpu?.load1?.toFixed(2) ?? "—"}`}
    />
    <StatCard
      label="Pamięć"
      value={sample?.mem?.usedPercent}
      line1={sample
        ? `${(sample.mem.usedKb / GB).toFixed(1).replace(".", ",")} / ${(sample.mem.totalKb / GB).toFixed(1).replace(".", ",")} GB`
        : ""}
      line2={sample ? `cache ${(sample.mem.cachedKb / GB).toFixed(1).replace(".", ",")} GB` : ""}
    />
    <StatCard
      label="Temperatura"
      value={maxTemp}
      unit="°C"
      max={90}
      line1={maxTemp == null ? "brak czujników" : "najcieplejszy czujnik"}
      line2={maxTemp == null ? "(maszyna wirtualna?)" : ""}
    />
    <StatCard
      label={mainDisk ? `Dysk ${mainDisk.mount}` : "Dysk"}
      value={mainDisk?.usedPercent}
      line1={mainDisk ? `${bytes(mainDisk.usedBytes)} / ${bytes(mainDisk.totalBytes)}` : ""}
      line2={mainDisk?.fstype ?? ""}
    />
  </div>

  <div class="grid-charts">
    <LiveChart
      title="Procesor"
      percent
      format={(v) => `${Math.round(v)}%`}
      series={[{ label: "CPU", color: "#53d6e8", get: (s) => s.cpu.usagePercent, fill: true }]}
      {sample}
    />
    <LiveChart
      title="Pamięć"
      percent
      format={(v) => `${Math.round(v)}%`}
      series={[{ label: "RAM", color: "#53d6e8", get: (s) => s.mem.usedPercent, fill: true }]}
      {sample}
    />
    <LiveChart
      title="Sieć"
      format={bps}
      series={[
        { label: "pobieranie", color: "#53d6e8", get: (s) => s.net.rxBps, fill: true },
        { label: "wysyłanie", color: "#ffb454", get: (s) => s.net.txBps, fill: false },
      ]}
      {sample}
    />
  </div>

  <div class="containers">
    <Containers />
  </div>

  <div class="containers">
    <Catalog />
  </div>

  <div class="grid-bottom">
    <Disks disks={sample?.disks ?? []} />
    <Temps temps={sample?.temps ?? []} />
  </div>
</div>
{/if}

<style>
  .containers { margin-bottom: 1rem; }
</style>
