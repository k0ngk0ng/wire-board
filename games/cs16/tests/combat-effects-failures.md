# Combat effect failure scenarios

- Gunfire adds an invented line between the weapon and the target. Remove those lines while retaining muzzle flash and surface impact feedback.
- A predicted shot or local scene ray creates blood before the server confirms a player hit. Only an authorized current-life hit may produce blood.
- A hit targets a hidden player, an old life, missing coordinates, a non-player object or a non-trace damage source. Do not reveal a hidden actor or invent a contact point.
- An unarmored body/head/leg hit has no native blood sprite, or the effect uses the player's origin instead of a matching original model hitbox. Respect the available armor and helmet projection.
- Native sprite frame dimensions, palette transparency, frame origins or blending are lost during presentation.
- An HE explode event removes the grenade without either native explosion sprite or its already available spatial sound. Flashbang and smoke events must not receive an HE fireball.
- Repeated events, reconnect, a new round or a new epoch replay old effects; sustained fire allocates without a bound; expired effects or GPU resources remain alive.
- Effects are drawn through opaque map geometry or create a new hit HUD. Blood and explosion sprites belong in the world and retain depth testing.
- The renderer fixture assumes the original runtime was executed. Keep static original resources, compatible reference semantics and actual browser observations separate.

Existing hit events contain the player's position and hitgroup, not the original TraceResult end point or damage direction. A matching animated MDL hitbox center is a documented visual approximation; it does not change damage or add network fields. Full GoldSrc blood-stream/decal particle physics remains outside the available event contract.
