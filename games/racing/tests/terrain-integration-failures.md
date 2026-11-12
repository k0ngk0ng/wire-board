# Terrain integration failures (before implementation)

- Off-road distance changes traction/resistance even when actual ground remains asphalt.
- Ground is guessed from the nearest VROAD node instead of original polygon material.
- Numeric terrain geometry differs from the visible original collision mesh, swaps Y/Z, or loses the shared origin/X reflection.
- The catalog binds a different track/route hash or altered source blob.
- A ground polygon, raw material ID, vertex, face, or original wheel location is lost during conversion.
- Client loads every track or skips its selected collision catalog; wrong/missing/corrupt material data is accepted as ready.
- Upper and lower road layers choose each other's surface; reset samples an unrelated layer.
- Unsupported/unknown material or no mesh hit becomes a guessed grass penalty.
- Go and TS terrain lookup/traction disagree; static catalogs leak into ordinary dynamic frames.

The conversion E2E must use all four delivered original collision files, compare every source coordinate and face, preserve all old route/rule/visual bytes, and reproduce identical output.
