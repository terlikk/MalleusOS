<script>
  // Główny komponent panelu: trzyma stan (info o systemie,
  // najnowsza próbka, status połączenia) i rozdaje go kartom.
  import Header from "./lib/Header.svelte";
  import StatCard from "./lib/StatCard.svelte";
  import LiveChart from "./lib/LiveChart.svelte";
  import Disks from "./lib/Disks.svelte";
  import Temps from "./lib/Temps.svelte";
  import { getJSON, streamMetrics } from "./lib/api.js";
  import { bytes, bps, rdzenie } from "./lib/format.js";

  // $state to reaktywny stan Svelte 5 — zmiana wartości
  // automatycznie odświeża wszystko, co z niej korzysta.
  let system = $state(null);
  let sample = $state(null);
  let online = $state(false);

  // $effect uruchamia się po zamontowaniu komponentu;
  // zwracana funkcja sprząta (zamyka strumień) przy odmontowaniu.
  $effect(() => {
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

  <div class="grid-bottom">
    <Disks disks={sample?.disks ?? []} />
    <Temps temps={sample?.temps ?? []} />
  </div>
</div>
