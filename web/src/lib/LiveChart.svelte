<script>
  // Wykres na żywo oparty o uPlot — malutką (~40 kB) bibliotekę
  // rysującą na <canvas>, wybraną w projekcie właśnie za lekkość.
  //
  // Start: pobieramy historię z API dla wybranego zakresu.
  // Potem: każda próbka z SSE dokleja się na końcu, a punkty
  // starsze niż zakres wypadają z lewej — wykres "płynie".
  import uPlot from "uplot";
  import "uplot/dist/uPlot.min.css";
  import { getJSON } from "./api.js";

  let {
    title,
    series,        // [{ label, color, get: (sample) => liczba, fill }]
    sample = null, // najnowsza próbka z SSE (podaje rodzic)
    percent = false, // true → stała skala 0–100
    format = (v) => String(Math.round(v)), // opis wartości na osi Y
  } = $props();

  const RANGES = ["15m", "1h", "2h"];
  const SECONDS = { "15m": 900, "1h": 3600, "2h": 7200 };

  let range = $state("15m");
  let root; // element DOM na wykres (bind:this)
  let plot = null;
  let data = [[], ...series.map(() => [])];

  async function load() {
    try {
      const hist = await getJSON(`/api/v1/metrics/history?range=${range}`);
      data = [
        hist.map((s) => s.time / 1000),
        ...series.map((sd) => hist.map(sd.get)),
      ];
      plot?.setData(data);
    } catch {
      // API chwilowo niedostępne — wykres po prostu poczeka
    }
  }

  function push(s) {
    if (!plot) return;
    data[0].push(s.time / 1000);
    series.forEach((sd, i) => data[i + 1].push(sd.get(s)));
    // wytnij punkty starsze niż wybrany zakres
    const cut = s.time / 1000 - SECONDS[range];
    while (data[0].length && data[0][0] < cut) {
      data.forEach((arr) => arr.shift());
    }
    plot.setData(data);
  }

  function setRange(r) {
    range = r;
    load();
  }

  // Nowa próbka od rodzica → doklej do wykresu.
  $effect(() => {
    if (sample) push(sample);
  });

  // Montaż: tworzymy uPlot, dociągamy historię, pilnujemy szerokości.
  $effect(() => {
    if (!root) return;
    plot = new uPlot(makeOpts(root.clientWidth), data, root);
    load();
    const ro = new ResizeObserver(() => {
      plot?.setSize({ width: root.clientWidth, height: 140 });
    });
    ro.observe(root);
    return () => {
      ro.disconnect();
      plot?.destroy();
      plot = null;
    };
  });

  function makeOpts(width) {
    return {
      width,
      height: 140,
      legend: { show: false },
      cursor: { show: false },
      scales: {
        x: { time: true },
        y: percent
          ? { range: [0, 100] }
          : { range: (u, min, max) => [0, Math.max(max * 1.2, 1)] },
      },
      axes: [
        {
          stroke: "#8fa0b6",
          grid: { show: false },
          ticks: { show: false },
          font: "11px system-ui",
        },
        {
          stroke: "#8fa0b6",
          grid: { stroke: "rgba(238,242,247,0.06)", width: 1 },
          ticks: { show: false },
          font: "11px system-ui",
          size: 56,
          values: (u, splits) => splits.map(format),
        },
      ],
      series: [
        {},
        ...series.map((sd) => ({
          stroke: sd.color,
          width: 2,
          // "22" na końcu hexa = ~13% przezroczystości wypełnienia
          fill: sd.fill ? sd.color + "22" : undefined,
          points: { show: false },
        })),
      ],
    };
  }
</script>

<div class="card">
  <div class="head">
    <h2>{title}</h2>
    {#if series.length > 1}
      <div class="legend">
        {#each series as sd}
          <span><i style="background:{sd.color}"></i>{sd.label}</span>
        {/each}
      </div>
    {/if}
    <div class="ranges">
      {#each RANGES as r}
        <button class:on={range === r} onclick={() => setRange(r)}>{r}</button>
      {/each}
    </div>
  </div>
  <div class="plot" bind:this={root}></div>
</div>

<style>
  .head { display: flex; align-items: center; gap: 1rem; flex-wrap: wrap; }
  .head h2 { margin: 0; }

  .legend { display: flex; gap: 0.9rem; }
  .legend span { font-size: 0.74rem; color: var(--dim); }
  .legend i {
    display: inline-block;
    width: 14px;
    height: 3px;
    border-radius: 2px;
    margin-right: 0.35rem;
    vertical-align: middle;
  }

  .ranges { margin-left: auto; display: flex; gap: 0.25rem; }
  .ranges button {
    font-size: 0.72rem;
    font-family: inherit;
    color: var(--dim);
    background: none;
    border: none;
    padding: 0.18rem 0.6rem;
    border-radius: 999px;
    cursor: pointer;
  }
  .ranges button.on {
    color: #06232a;
    background: var(--cyan);
    font-weight: 600;
  }

  .plot { margin-top: 0.5rem; }
</style>
