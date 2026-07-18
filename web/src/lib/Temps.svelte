<script>
  // Czujniki temperatur. Na maszynach wirtualnych zwykle ich nie ma —
  // wtedy uczciwie o tym mówimy zamiast pokazywać pustkę.
  import { stateColor } from "./format.js";

  let { temps = [] } = $props();
</script>

<div class="card">
  <h2>Temperatury</h2>
  {#if temps.length === 0}
    <p class="empty">
      Brak czujników — na maszynie wirtualnej to normalne.
      Na prawdziwym sprzęcie pojawią się tu odczyty.
    </p>
  {:else}
    <ul>
      {#each temps as t (t.label)}
        <li>
          <span class="label">{t.label}</span>
          <b class="mono" style="color:{stateColor(t.celsius / 90)}">
            {Math.round(t.celsius)}°C
          </b>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  ul { list-style: none; }
  li {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    padding: 0.45rem 0;
    border-top: 1px solid var(--edge);
    font-size: 0.85rem;
  }
  li:first-child { border-top: none; }
  .label { color: var(--dim); }
</style>
