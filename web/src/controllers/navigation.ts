// Directional focus navigation never changes game state. Confirm goes through
// the existing control's click handler and the game's own validation.
type Target = HTMLElement | SVGElement;
const selector =
  'button,select,input[type="number"],input[type="range"],input[type="checkbox"],input[type="radio"],[role="button"],[role="gridcell"],a[href],summary,[tabindex]';
const groupSelector =
  '[data-controller-group],[role="grid"],.poker-hand,.poker-actions';

function targets(root: Element): Target[] {
  return [...root.querySelectorAll<Target>(selector)].filter((node) => {
    const r = node.getBoundingClientRect();
    return (
      !node.matches(
        ":disabled,[data-controller-ignore],[data-fullscreen-board]",
      ) &&
      !node.closest("[hidden],[inert]") &&
      r.width > 0 &&
      r.height > 0 &&
      getComputedStyle(node).visibility !== "hidden"
    );
  });
}

export class ControllerNavigation {
  private focused?: Target;
  private lastTarget?: Target;
  private previous = new Set<string>();
  private repeatAt = new Map<string, number>();
  private heldSince = new Map<string, number>();
  private root: HTMLElement;
  private lastPoint?: { x: number; y: number; group: Element | null };
  private scope: Element;
  constructor(root: HTMLElement) {
    this.root = root;
    this.scope = root;
    root.addEventListener("focusin", this.remember);
  }
  clear() {
    this.previous.clear();
    this.repeatAt.clear();
    this.heldSince.clear();
  }
  dispose() {
    this.clear();
    this.focused?.removeAttribute("data-controller-focus");
    this.root.removeEventListener("focusin", this.remember);
  }
  private remember = (event: FocusEvent) => {
    const target = event.target;
    if (
      !(target instanceof HTMLElement || target instanceof SVGElement) ||
      !target.matches(selector) ||
      target.matches("[data-fullscreen-board]")
    )
      return;
    this.capture(target);
  };
  private capture(target: Target) {
    if (this.lastTarget !== target) this.heldSince.clear();
    this.lastTarget = target;
    const r = target.getBoundingClientRect();
    this.lastPoint = {
      x: r.x + r.width / 2,
      y: r.y + r.height / 2,
      group: target.closest(groupSelector),
    };
  }
  private initial(nodes: Target[]) {
    const last = this.lastPoint;
    if (last) {
      const group = last.group;
      const local =
        group?.isConnected && this.scope.contains(group)
          ? nodes.filter((n) => group.contains(n))
          : nodes;
      if (local.length)
        return local.reduce((best, node) => {
          const distance = (n: Target) => {
            const r = n.getBoundingClientRect();
            return Math.hypot(
              r.x + r.width / 2 - last.x,
              r.y + r.height / 2 - last.y,
            );
          };
          return distance(node) < distance(best) ? node : best;
        });
    }
    const board = nodes.filter((n) =>
      n.closest(
        '.poker-hand,[role="grid"],[data-controller-group="board"],[data-controller-group="grid"]',
      ),
    );
    return (
      board.find((n) => n.tabIndex === 0) ??
      board[0] ??
      nodes.find((n) => n.tabIndex === 0) ??
      nodes[0]
    );
  }
  private focus(target?: Target) {
    if (!target) return;
    this.focused?.removeAttribute("data-controller-focus");
    this.focused = target;
    target.setAttribute("data-controller-focus", "");
    target.focus({ preventScroll: true });
    // SVG viewBox cameras need game-owned panning instead of HTML scrolling.
    if (
      target.dispatchEvent(
        new Event("controller-reveal", {
          bubbles: true,
          cancelable: true,
        }),
      )
    )
      target.scrollIntoView({ block: "nearest", inline: "nearest" });
  }
  sample(actions: ReadonlySet<string>, at: number) {
    const dialogs = [
      ...(this.root.matches('[role="dialog"],dialog[open]') ? [this.root] : []),
      ...this.root.querySelectorAll<HTMLElement>(
        '[role="dialog"],dialog[open]',
      ),
    ].filter((e) => e.getClientRects().length && !e.closest("[inert]"));
    const scope = dialogs.at(-1) ?? this.root;
    if (scope !== this.scope) {
      this.scope = scope;
      this.lastPoint = undefined;
    }
    const active = document.activeElement;
    if (
      (active instanceof HTMLElement || active instanceof SVGElement) &&
      scope.contains(active) &&
      active.matches(selector) &&
      !active.matches(":disabled,[data-fullscreen-board]") &&
      active.getClientRects().length
    )
      this.capture(active);
    if (!actions.size) {
      this.previous.clear();
      this.repeatAt.clear();
      this.heldSince.clear();
      return;
    }
    const nodes = targets(scope);
    let current =
      nodes.find((n) => n === active) ??
      (this.focused &&
      this.focused === this.lastTarget &&
      nodes.includes(this.focused)
        ? this.focused
        : undefined);
    for (const action of actions) {
      const directional = /^nav(Up|Down|Left|Right)$/.test(action);
      if (this.previous.has(action)) {
        if (!directional || at < (this.repeatAt.get(action) ?? Infinity))
          continue;
        this.repeatAt.set(action, at + 110);
      } else {
        this.repeatAt.set(action, at + 350);
        this.heldSince.set(action, at);
      }
      if (
        directional &&
        current &&
        !current.dispatchEvent(
          new CustomEvent("controller-direction", {
            bubbles: true,
            cancelable: true,
            detail: { direction: action },
          }),
        )
      )
        continue;
      if (
        current instanceof HTMLInputElement &&
        current.matches('[type="number"],[type="range"]') &&
        (action === "navUp" || action === "navDown")
      ) {
        if (current.readOnly) continue;
        if (document.activeElement !== current) this.focus(current);
        if (!this.heldSince.has(action)) this.heldSince.set(action, at);
        const before = current.value,
          step =
            (action === "navUp" ? 1 : -1) *
            (at - (this.heldSince.get(action) ?? at) >= 1200 ? 10 : 1);
        try {
          current.stepUp(step);
        } catch {
          const min = parseFloat(current.min),
            max = parseFloat(current.max),
            unit = parseFloat(current.step);
          current.value = String(
            Math.min(
              Number.isFinite(max) ? max : Infinity,
              Math.max(
                Number.isFinite(min) ? min : -Infinity,
                (Number(before) || 0) +
                  step * (Number.isFinite(unit) && unit > 0 ? unit : 1),
              ),
            ),
          );
        }
        const after = current.value;
        if (after !== before) {
          current.value = before;
          Object.getOwnPropertyDescriptor(
            HTMLInputElement.prototype,
            "value",
          )!.set!.call(current, after);
          current.dispatchEvent(new Event("input", { bubbles: true }));
          current.dispatchEvent(new Event("change", { bubbles: true }));
        }
        continue;
      }
      if (
        current instanceof HTMLSelectElement &&
        (action === "navUp" || action === "navDown")
      ) {
        if (document.activeElement !== current) this.focus(current);
        const step = action === "navUp" ? -1 : 1;
        for (
          let index = current.selectedIndex + step;
          index >= 0 && index < current.options.length;
          index += step
        ) {
          if (
            current.options[index].disabled ||
            current.options[index].parentElement?.matches("optgroup:disabled")
          )
            continue;
          current.selectedIndex = index;
          current.dispatchEvent(new Event("change", { bubbles: true }));
          break;
        }
        continue;
      }
      if (directional) {
        if (!current) {
          current = this.initial(nodes);
        } else {
          const within = current.closest(groupSelector);
          const candidates = within
            ? nodes.filter((n) => within.contains(n))
            : nodes;
          const r = current.getBoundingClientRect(),
            x = r.x + r.width / 2,
            y = r.y + r.height / 2;
          const horizontal = action === "navLeft" || action === "navRight";
          const sign = action === "navLeft" || action === "navUp" ? -1 : 1;
          let score = Infinity,
            next: Target | undefined;
          for (const node of candidates) {
            if (node === current) continue;
            const currentRow = current.closest("[data-controller-row]");
            const cardRows =
              within?.hasAttribute("data-controller-cards") &&
              current instanceof HTMLElement &&
              node instanceof HTMLElement;
            if (currentRow || cardRows) {
              const flowY = (target: Target) => {
                let y = 0;
                for (
                  let e: HTMLElement | null =
                    target instanceof HTMLElement ? target : null;
                  e;
                  e =
                    e.offsetParent instanceof HTMLElement
                      ? e.offsetParent
                      : null
                )
                  y += e.offsetTop;
                return y;
              };
              const sameRow = currentRow
                ? currentRow.contains(node)
                : Math.abs(flowY(current) - flowY(node)) < 3;
              if (horizontal !== sameRow) continue;
            }
            const b = node.getBoundingClientRect(),
              dx = b.x + b.width / 2 - x,
              dy = b.y + b.height / 2 - y;
            const forward = sign * (horizontal ? dx : dy),
              across = Math.abs(horizontal ? dy : dx);
            if (forward <= 2) continue;
            const row = horizontal
              ? Math.min(r.height, b.height) / 2
              : Math.min(r.width, b.width) / 2;
            const distance = forward + across * 2 + (across > row ? 10000 : 0);
            if (distance < score) {
              score = distance;
              next = node;
            }
          }
          current = next ?? current;
        }
        this.focus(current);
      } else if (action === "confirm") {
        if (!current) {
          current = this.initial(nodes);
          this.focus(current);
          continue;
        }
        if (
          current instanceof HTMLSelectElement ||
          (current instanceof HTMLInputElement &&
            current.matches('[type="number"],[type="range"]'))
        ) {
          current =
            nodes
              .slice(nodes.indexOf(current) + 1)
              .find(
                (n) =>
                  !(n instanceof HTMLSelectElement) &&
                  !n.matches('input[type="number"],input[type="range"]'),
              ) ?? nodes[0];
          this.focus(current);
          continue;
        }
        if (current.getAttribute("aria-disabled") === "true") continue;
        if (current instanceof HTMLElement) current.click();
        else
          current.dispatchEvent(
            new MouseEvent("click", {
              bubbles: true,
              cancelable: true,
              detail: 0,
            }),
          );
      } else if (action === "cancel") {
        if (
          current &&
          !current.dispatchEvent(
            new Event("controller-cancel", { bubbles: true, cancelable: true }),
          )
        )
          continue;
        const cancel =
          scope.querySelector<HTMLElement>("[data-controller-cancel]") ??
          (dialogs.length
            ? scope.querySelector<HTMLElement>('button[aria-label="关闭"]')
            : null);
        cancel?.click();
      } else if (action === "nextGroup" || action === "previousGroup") {
        const candidate = current?.closest(groupSelector);
        const group = candidate && scope.contains(candidate) ? candidate : null;
        const index = current ? nodes.indexOf(current) : -1;
        const step = action === "nextGroup" ? 1 : -1;
        for (let i = 1; i <= nodes.length; i++) {
          const next =
            nodes[(index + step * i + nodes.length * 2) % nodes.length];
          if (!group || !group.contains(next)) {
            current = next;
            this.focus(next);
            break;
          }
        }
      } else {
        // Discrete game-owned shortcuts keep the current dialog's scope.
        (current ?? scope).dispatchEvent(
          new CustomEvent("controller-action", {
            bubbles: true,
            cancelable: true,
            detail: { action },
          }),
        );
      }
    }
    this.previous = new Set(actions);
    for (const action of this.repeatAt.keys())
      if (!actions.has(action)) {
        this.repeatAt.delete(action);
        this.heldSince.delete(action);
      }
  }
}
