<script>
  // Lista dysków z paskami zajętości i progami kolorów.
  import { bytes, stateColor } from "./format.js";

  let { disks = [] } = $props();
</script>

<div class="card">
  <h2>Dyski</h2>
  {#if disks.length === 0}
    <p class="empty">Nie znaleziono zamontowanych dysków.</p>
  {:else}
    {#each disks as d (d.mount)}
      <div class="disk">
        <div class="top">
          <b class="mono">{d.mount}</b>
          <span>{bytes(d.usedBytes)} / {bytes(d.totalBytes)}</span>
        </div>
        <div class="track">
          <i style="width:{d.usedPercent}%; background:{stateColor(d.usedPercent / 100)}"></i>
        </div>
      </div>
    {/each}
  {/if}
</div>

<style>
  .disk { margin-bottom: 1rem; }
  .disk:last-child { margin-bottom: 0.2rem; }

  .top {
    display: flex;
    justify-content: space-between;
    font-size: 0.82rem;
    margin-bottom: 0.45rem;
  }
  .top span { color: var(--dim); }

  .track {
    height: 7px;
    border-radius: 4px;
    background: var(--track);
    overflow: hidden;
  }
  .track i {
    display: block;
    height: 100%;
    border-radius: 4px;
    transition: width 0.6s ease;
  }
</style>
