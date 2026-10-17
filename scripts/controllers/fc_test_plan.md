# FC controller acceptance, written before implementation

- Original FC Controller I burgundy/gold background SVG; left cross, centre Select/Start, right upper row turbo B / turbo A and lower row B / A, aligned in two columns.
- Personal keyboard mapping is visible; configure buttons are unique and classic-4 does not display an FC faceplate.
- Native mouse down/up, continuous D-pad dragging, diagonals, centre release, capture outside bounds.
- Native multi-touch retains A/B while sliding a different finger on the D-pad; touch cancel clears only that pointer.
- Blur, document hide, disabled, profile replacement and unmount release owned pointer sources; no UI timers implement turbo.
- Turbo buttons are part of the faceplate, never a detached strip. Every individual physical control has a non-overlapping hit region and a stable logical-button data attribute.
- Settings remapping, duplicate rejection, save and changing model use the actual Settings component.
- Desktop and 320px screenshots, final hit testing and source background-image load are retained as reproducible artifacts.
