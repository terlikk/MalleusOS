// ============================================================
// MalleusOS — skrypt strony projektu
// 1. Pobiera najnowsze wydanie z GitHub API i podmienia linki.
// 2. Obsługuje przycisk "kopiuj" w sekcji Szybki start.
// ============================================================

const REPO = "terlikk/MalleusOS";
const RELEASES_PAGE = `https://github.com/${REPO}/releases`;

// ---------- Linki "Pobierz" z GitHub API ----------
// Endpoint /releases/latest zwraca JSON z najnowszym wydaniem
// i listą jego plików (assets). Jeśli wydań jeszcze nie ma,
// GitHub odpowiada błędem 404 — wtedy zostawiamy linki
// prowadzące na stronę Releases (fallback ustawiony w HTML).
async function wczytajNajnowszeWydanie() {
  const info = document.getElementById("release-info");

  try {
    const odp = await fetch(`https://api.github.com/repos/${REPO}/releases/latest`);
    if (!odp.ok) throw new Error(`GitHub API: ${odp.status}`);

    const wydanie = await odp.json();

    // Szukamy plików po nazwie — tak będą się nazywać binarki z CI.
    const linki = [
      { id: "dl-amd64", nazwa: "malleus-linux-amd64" },
      { id: "dl-arm64", nazwa: "malleus-linux-arm64" },
    ];

    for (const { id, nazwa } of linki) {
      const asset = (wydanie.assets || []).find((a) => a.name === nazwa);
      if (asset) {
        const a = document.getElementById(id);
        a.href = asset.browser_download_url;
        a.textContent = asset.name;
      }
    }

    info.textContent = `najnowsze wydanie: ${wydanie.tag_name}`;
  } catch {
    // Brak wydań albo brak sieci — nic nie psujemy, linki z HTML
    // dalej prowadzą na stronę Releases.
    info.innerHTML =
      `brak wydań — binarki pojawią się wkrótce, zajrzyj na ` +
      `<a href="${RELEASES_PAGE}" rel="noopener">stronę Releases</a>`;
  }
}

// ---------- Przycisk "kopiuj" ----------
// navigator.clipboard to nowoczesne API schowka (wymaga HTTPS
// albo localhost — na Vercelu i przy podglądzie lokalnym działa).
function ustawKopiowanie() {
  const przycisk = document.getElementById("btn-copy");
  const kod = document.getElementById("quickstart-code");
  if (!przycisk || !kod) return;

  przycisk.addEventListener("click", async () => {
    try {
      await navigator.clipboard.writeText(kod.textContent.trim());
      przycisk.textContent = "skopiowano ✓";
      przycisk.classList.add("copied");
      // Po 2 sekundach wracamy do stanu wyjściowego.
      setTimeout(() => {
        przycisk.textContent = "kopiuj";
        przycisk.classList.remove("copied");
      }, 2000);
    } catch {
      przycisk.textContent = "błąd :(";
    }
  });
}

wczytajNajnowszeWydanie();
ustawKopiowanie();
