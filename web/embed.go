// Pakiet web wkleja zbudowany panel (katalog dist/) do wnętrza
// binarki. Dyrektywa //go:embed każe kompilatorowi zabrać pliki
// ze sobą — binarka serwuje potem panel bez żadnych plików na
// dysku, dzięki czemu całość działa offline z jednego pliku.
//
// Wzorzec "all:dist" bierze też pliki zaczynające się od kropki
// (np. .gitkeep, dzięki któremu pusty dist/ istnieje w repo
// i świeży klon kompiluje się bez Node'a).
package web

import "embed"

//go:embed all:dist
var Dist embed.FS
