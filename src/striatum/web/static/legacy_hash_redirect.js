// RFC 0022 V1: legacy hash-route redirect for bookmarked SPA URLs.
// The browser doesn't send the # to the server, so this small island
// runs on every page load and rewrites #/foo to /foo before the user
// notices.
(function () {
  var hash = window.location.hash || "";
  if (hash.length < 2 || hash.charAt(0) !== "#" || hash.charAt(1) !== "/") return;
  var target = hash.substring(1);
  if (target.indexOf("..") !== -1) return;
  window.location.replace(target);
})();
