const glowControllers = new WeakMap();

function bindGlow(element) {
  if (glowControllers.has(element)) return;
  let frame = 0;
  let point = null;
  const draw = () => {
    frame = 0;
    if (!point) return;
    const rect = element.getBoundingClientRect();
    element.style.setProperty("--mouse-x", `${point.x - rect.left}px`);
    element.style.setProperty("--mouse-y", `${point.y - rect.top}px`);
  };
  const move = (event) => {
    point = { x: event.clientX, y: event.clientY };
    if (!frame) frame = requestAnimationFrame(draw);
  };
  const enter = (event) => {
    element.style.setProperty("--glow-opacity", "1");
    move(event);
  };
  const leave = () => {
    point = null;
    element.style.setProperty("--glow-opacity", "0");
    if (frame) cancelAnimationFrame(frame);
    frame = 0;
  };
  element.addEventListener("pointerenter", enter);
  element.addEventListener("pointermove", move);
  element.addEventListener("pointerleave", leave);
  element.addEventListener("pointercancel", leave);
  glowControllers.set(element, { enter, move, leave });
}

export function bindGlowButtons(root) {
  root.querySelectorAll(".glow-button").forEach(bindGlow);
}
