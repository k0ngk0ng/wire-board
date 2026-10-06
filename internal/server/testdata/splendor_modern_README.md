# Splendor candidate HTTP initial states

`splendor_modern_2.json`, `_3.json` and `_4.json` were exported from the private
`newSplendorModernCatalogState` constructor with all four modules enabled.
They preserve real shuffled initial decks, three distinct randomly sided city
tiles, empty player inventories, normal bank supply and a random starting seat.
No cards, discounts, points, posts or winners were granted. These are candidate
2025 component data, not independently verified complete physical catalogs.

`TestSplendorModernHTTPFixtureIntegrity` in `internal/game` checks every card
against the base/candidate catalogs, each city against the candidate catalog,
all deck counts, component uniqueness, initial resources and empty player state.
Update the fixture facts deliberately if the candidate catalog is corrected;
do not regenerate fixtures merely to make a failing game disappear.

`internal/server/splendor_modern_test.go` installs these states only after normal
HTTP authentication, seating, readiness and start. It removes disabled modules
for the 16 combinations (restoring a legal base noble selection when cities are
disabled), while retaining a base-only room draft to exercise actual-match
archiving. Actions are legal HTTP commands or the real server autoplay path.
Pending timeout tests reach their target phase by legal play; the blind-reserve
case explicitly chooses an Orient blind reservation after acquiring that post.

These fixtures do not enable public Cities/Orient configuration and do not prove
artwork completeness, all physical card facts, or browser/animation acceptance.
