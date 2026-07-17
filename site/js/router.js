// ============================================================
// MalleusOS — prosty router podstron
//
// Strona jest podzielona na widoki (sekcje .view). W danej
// chwili widać dokładnie jeden — wybrany adresem po # (tzw.
// hash), np. strona.pl/#pobierz. Zmiana hasha nie przeładowuje
// strony, a przycisk "wstecz" przeglądarki działa normalnie.
// ============================================================
(function () {
  "use strict";

  const widoki = document.querySelectorAll(".view");
  const linki = document.querySelectorAll(".sidebar-nav a");
  const sidebar = document.getElementById("sidebar");
  const toggle = document.getElementById("menu-toggle");
  const DOMYSLNY = "start";

  // Który widok wskazuje aktualny adres? (bez # na początku)
  function biezacyWidok() {
    const id = window.location.hash.replace("#", "");
    return document.getElementById(id) && id ? id : DOMYSLNY;
  }

  function pokazWidok(id) {
    widoki.forEach((w) => w.classList.toggle("active", w.id === id));

    // aria-current mówi czytnikom ekranu, gdzie jesteśmy;
    // CSS używa go też do podświetlenia aktywnego linku.
    linki.forEach((a) => {
      if (a.getAttribute("href") === "#" + id) {
        a.setAttribute("aria-current", "page");
      } else {
        a.removeAttribute("aria-current");
      }
    });

    zamknijMenu();
    window.scrollTo(0, 0); // gdyby widok był wyższy niż ekran
  }

  window.addEventListener("hashchange", () => pokazWidok(biezacyWidok()));
  pokazWidok(biezacyWidok());

  // ---------- Rozwijane menu na telefonie ----------
  function zamknijMenu() {
    sidebar.classList.remove("open");
    toggle.setAttribute("aria-expanded", "false");
    toggle.setAttribute("aria-label", "Otwórz menu");
  }

  toggle.addEventListener("click", () => {
    const otwarte = sidebar.classList.toggle("open");
    toggle.setAttribute("aria-expanded", String(otwarte));
    toggle.setAttribute("aria-label", otwarte ? "Zamknij menu" : "Otwórz menu");
  });

  // Escape zamyka menu — standardowe zachowanie, którego
  // użytkownicy klawiatury się spodziewają.
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") zamknijMenu();
  });
})();
