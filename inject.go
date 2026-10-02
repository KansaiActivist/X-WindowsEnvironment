//go:build windows

package main


const injectedJS = `
(function () {
  if (window.__xbrowser_injected) return;
  window.__xbrowser_injected = true;

  var ALLOWED_SUFFIXES = [
    'x.com', 'twitter.com', 'twimg.com', 't.co', 'twimg.net'
  ];

  function isAllowedHost(host) {
    host = (host || '').toLowerCase();
    for (var i = 0; i < ALLOWED_SUFFIXES.length; i++) {
      var s = ALLOWED_SUFFIXES[i];
      if (host === s || host.indexOf('.' + s) === host.length - s.length - 1) {
        return true;
      }
    }
    return false;
  }

  function post(type, data) {
    try {
      window.chrome.webview.postMessage(JSON.stringify({ type: type, data: data }));
    } catch (e) {  }
  }

  document.addEventListener('click', function (e) {
    var el = e.target;
    while (el && el.tagName !== 'A') el = el.parentElement;
    if (!el || !el.href) return;
    var u;
    try {
      u = new URL(el.href, location.href);
    } catch (err) {
      return;
    }
    if (!isAllowedHost(u.hostname)) {
      e.preventDefault();
      e.stopPropagation();
      post('open_external', u.href);
      return;
    }
    if (el.target && el.target !== '' && el.target.toLowerCase() !== '_self') {
      e.preventDefault();
      e.stopPropagation();
      location.href = u.href;
    }
  }, true);

  window.open = function (url, target, features) {
    try {
      var u = new URL(url, location.href);
      if (!isAllowedHost(u.hostname)) {
        post('open_external', u.href);
      } else {
        location.href = u.href;
      }
    } catch (e) {  }
    return null;
  };

  var lastWheel = 0;
  window.addEventListener('wheel', function (e) {
    if (!e.ctrlKey) return;
    e.preventDefault();
    var now = Date.now();
    if (now - lastWheel < 60) return;
    lastWheel = now;
    post('zoom', e.deltaY < 0 ? 'in' : 'out');
  }, { passive: false, capture: true });
  window.addEventListener('keydown', function (e) {
    if (!e.ctrlKey || e.altKey) return;
    if (e.key === '+' || e.key === '=' || e.key === ';') { e.preventDefault(); post('zoom', 'in'); }
    else if (e.key === '-' || e.key === '_') { e.preventDefault(); post('zoom', 'out'); }
    else if (e.key === '0') { e.preventDefault(); post('zoom', 'reset'); }
  }, true);

  function reportTitle() { post('title', document.title); post('url', location.href); }
  var titleEl = document.querySelector('title');
  if (titleEl) {
    new MutationObserver(reportTitle).observe(titleEl, { childList: true });
  }
  window.addEventListener('popstate', reportTitle);
  reportTitle();
})();
`
