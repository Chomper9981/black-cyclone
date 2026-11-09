// Hero screenshots: three app screens stacked in perspective. The front one
// changes with the tabs, a click on a screen behind, a swipe, the arrow keys,
// or by itself every few seconds (paused on hover / focus / hidden tab, off
// with reduced motion). On a mouse, the front screen leans toward the pointer.
(function () {
  var deck = document.querySelector(".deck");
  if (!deck) return;
  var shots = Array.prototype.slice.call(deck.querySelectorAll(".shot"));
  var tabs = Array.prototype.slice.call(deck.querySelectorAll(".deck-tabs button"));
  var n = shots.length;
  var front = 0;
  var reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  var AUTO_MS = 6000;
  var timer = null;
  var paused = false;

  function show(i) {
    front = (i + n) % n;
    shots.forEach(function (s, k) {
      var pos = (k - front + n) % n;
      s.setAttribute("data-pos", pos);
      s.setAttribute("aria-hidden", pos === 0 ? "false" : "true");
    });
    tabs.forEach(function (t, k) { t.setAttribute("aria-pressed", k === front ? "true" : "false"); });
  }

  function restart() {
    clearInterval(timer);
    if (reduce) return;
    timer = setInterval(function () { if (!paused && !document.hidden) show(front + 1); }, AUTO_MS);
  }

  tabs.forEach(function (t, k) {
    t.addEventListener("click", function () { show(k); restart(); });
  });
  shots.forEach(function (s, k) {
    s.addEventListener("click", function () { if (k !== front) { show(k); restart(); } });
  });

  deck.addEventListener("mouseenter", function () { paused = true; });
  deck.addEventListener("mouseleave", function () { paused = false; reset(); });
  deck.addEventListener("focusin", function () { paused = true; });
  deck.addEventListener("focusout", function () { paused = false; });
  deck.addEventListener("keydown", function (e) {
    if (e.key === "ArrowRight") { show(front + 1); restart(); }
    else if (e.key === "ArrowLeft") { show(front - 1); restart(); }
  });

  // Swipe on touch screens.
  var x0 = null;
  deck.addEventListener("touchstart", function (e) { x0 = e.touches[0].clientX; }, { passive: true });
  deck.addEventListener("touchend", function (e) {
    if (x0 === null) return;
    var dx = e.changedTouches[0].clientX - x0;
    x0 = null;
    if (Math.abs(dx) > 40) { show(front + (dx < 0 ? 1 : -1)); restart(); }
  }, { passive: true });

  // Lean toward the pointer (mouse only).
  function reset() {
    deck.style.setProperty("--tilt-x", "0deg");
    deck.style.setProperty("--tilt-y", "0deg");
  }
  if (!reduce && window.matchMedia("(pointer: fine)").matches) {
    deck.addEventListener("mousemove", function (e) {
      var r = deck.getBoundingClientRect();
      var px = (e.clientX - r.left) / r.width - 0.5;
      var py = (e.clientY - r.top) / r.height - 0.5;
      deck.style.setProperty("--tilt-y", (px * 14).toFixed(2) + "deg");
      deck.style.setProperty("--tilt-x", (-py * 10).toFixed(2) + "deg");
    });
  }

  show(0);
  restart();
})();
