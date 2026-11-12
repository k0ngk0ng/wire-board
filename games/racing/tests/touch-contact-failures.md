# Native touch contact failure inventory

Written before the contact tracker implementation:

- A missing PointerEvent release leaves a held direction even after the native active Touch list no longer includes that finger.
- Ending one finger clears the other finger's throttle, or ending throttle clears steering.
- Reused Touch identifiers inherit the prior gesture; observing an unknown active identifier creates a press that never began on a held control.
- Steering slides remain on the original arrow, outside controls remains held, or pedal-to-brake sliding keeps acceleration active.
- Left/right together fail to neutralize; cancel/blur/hidden/rotation retains horn/rear/handbrake.
- Touch and compatibility pointer events double-own the same physical gesture; pointer buttons==0 fails to release mouse/pen input.
- Long-press callouts and selection steal release; canvas/HUD/select-all remain selectable. Real text editors and platform chat must retain editing/selection.
