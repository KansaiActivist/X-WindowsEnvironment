//go:build windows

package main

// injectedJS は各タブのページ読み込み時(document作成時)に必ず実行される。
// ・X/Twitter/twimg/t.co 以外へのリンククリックや window.open を横取りし、
//
//	Go側(handleTabMessage)に "open_external" として通知する。
//
// ・X自身のドメインへの target="_blank" や window.open も
//
//	"新しいネイティブウィンドウ" を一切開かせず、必ず同じタブの中で
//	遷移させる(WebView2はNewWindowRequestedを何もハンドルしないと、
//	既定の飾り気のないポップアップウィンドウを勝手に開いてしまうため)。
//
// ・タブのタイトル変更を "title" として通知し、タブ見出しに反映する。
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
    } catch (e) { /* WebView2以外での実行時など */ }
  }

  // <a>クリックの横取り (キャプチャフェーズで先取りする)
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
    // 同じXのドメインでも target="_blank" 等は新しいネイティブウィンドウを
    // 開かせず、今のタブの中でそのまま遷移させる。
    if (el.target && el.target !== '' && el.target.toLowerCase() !== '_self') {
      e.preventDefault();
      e.stopPropagation();
      location.href = u.href;
    }
  }, true);

  // window.open() の横取り。外部URLならOSの既定ブラウザーへ、
  // Xのドメイン宛でも新しいネイティブウィンドウは絶対に開かず、
  // 今のタブの中で遷移させる(常にnullを返し、本物のwindow.openは呼ばない)。
  window.open = function (url, target, features) {
    try {
      var u = new URL(url, location.href);
      if (!isAllowedHost(u.hostname)) {
        post('open_external', u.href);
      } else {
        location.href = u.href;
      }
    } catch (e) { /* 無視 */ }
    return null;
  };

  // 拡大縮小: Ctrl+ホイール / Ctrl+ +,-,0 (通常はGo側のアクセラレータで処理されるが、念のためJS側でも受ける)
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

  // タブ見出し用にタイトルの変化を通知する
  function reportTitle() { post('title', document.title); post('url', location.href); }
  var titleEl = document.querySelector('title');
  if (titleEl) {
    new MutationObserver(reportTitle).observe(titleEl, { childList: true });
  }
  window.addEventListener('popstate', reportTitle);
  reportTitle();
})();
`
