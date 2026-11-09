// Keep the reading position when switching language.
// Both pages have the same sections in the same order, so the position is sent
// as "section index + fraction through it" rather than raw pixels (the two
// languages have different text lengths).
(function () {
  var NAV = 64; // sticky header height

  function sections() {
    return Array.prototype.slice.call(document.querySelectorAll("main > section"));
  }

  function currentPos() {
    var y = window.scrollY + NAV;
    var list = sections();
    for (var i = list.length - 1; i >= 0; i--) {
      var s = list[i];
      if (y >= s.offsetTop) {
        var frac = Math.min(1, (y - s.offsetTop) / Math.max(1, s.offsetHeight));
        return i + "-" + frac.toFixed(3);
      }
    }
    return "0-0";
  }

  document.addEventListener("click", function (e) {
    var a = e.target.closest && e.target.closest("a.lang");
    if (!a || window.scrollY === 0) return;
    e.preventDefault();
    var url = new URL(a.href);
    url.searchParams.set("pos", currentPos());
    location.href = url.href;
  });

  var pos = new URLSearchParams(location.search).get("pos");
  if (!pos) return;
  history.replaceState(null, "", location.pathname + location.hash);

  function restore() {
    var m = /^(\d+)-([\d.]+)$/.exec(pos);
    var s = m && sections()[+m[1]];
    if (!s) return;
    var top = s.offsetTop + parseFloat(m[2]) * s.offsetHeight - NAV;
    window.scrollTo({ top: Math.max(0, top), behavior: "instant" });
  }
  restore(); // scripts load deferred, so layout exists already
  window.addEventListener("load", restore); // again once fonts/images settle
})();
