(function () {
  function syncQuery(source) {
    var q = source.value;
    document.querySelectorAll('[data-sync-query-link]').forEach(function (a) {
      var url = new URL(a.href, window.location.href);
      if (q) {
        url.searchParams.set('q', q);
      } else {
        url.searchParams.delete('q');
      }
      a.href = url.toString();
    });
  }

  document.querySelectorAll('[data-sync-query]').forEach(function (el) {
    el.addEventListener('input', function () { syncQuery(el); });
    syncQuery(el);
  });
})();
