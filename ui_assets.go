package main

const uiHTML = `<!doctype html>
<html lang="de">
<head>
<meta charset="utf-8">
<meta http-equiv="X-UA-Compatible" content="IE=edge">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Replado Connector</title>
<link rel="stylesheet" href="/app.css">
</head>
<body>
<header class="top">
  <div class="top-inner">
    <div class="brand">
      <span class="logo" aria-hidden="true">R</span>
      <span class="brand-text"><span class="name">Replado Connector</span><span id="about" class="about"></span></span>
    </div>
    <span id="pill" class="pill" role="status"></span>
  </div>
  <div id="tabs" class="tabs" role="tablist" aria-label="Bereiche" hidden>
    <button type="button" role="tab" id="tab-overview" aria-controls="view-overview" data-view="overview">Übersicht</button><button type="button" role="tab" id="tab-folders" aria-controls="view-folders" data-view="folders">Ordner <span id="folder-count" class="count"></span></button><button type="button" role="tab" id="tab-log" aria-controls="view-log" data-view="log">Protokoll</button>
  </div>
</header>
<main>
  <div id="outdated" class="notice" role="status" hidden></div>
  <div id="error" class="alert" role="alert" hidden></div>

  <section id="setup" class="card narrow" aria-labelledby="h-login" hidden>
    <h1 id="h-login">Mit Replado verbinden</h1>
    <p class="muted">Melden Sie sich mit Ihrem Replado-Zugang an. Das geht nur als Administrator.</p>
    <form id="login">
      <label>Adresse Ihres Replado<input id="server" type="url" autocomplete="url" required placeholder="https://replado.ihre-firma.de"></label>
      <label>E-Mail<input id="email" type="email" autocomplete="username" required></label>
      <label>Passwort<input id="password" type="password" autocomplete="current-password" required></label>
      <label id="code-label" hidden>Code aus der Authenticator-App<input id="code" inputmode="numeric" autocomplete="one-time-code"></label>
      <button type="submit" id="login-button" class="wide">Anmelden</button>
    </form>
  </section>

  <section id="view-overview" class="view" role="tabpanel" aria-labelledby="tab-overview" tabindex="-1" hidden>
    <p id="installed" class="notice" role="status" hidden>Der Replado Connector ist jetzt auf diesem Rechner installiert und startet automatisch mit. Die heruntergeladene Datei wird nicht mehr gebraucht und kann gelöscht werden.</p>
    <div id="first-folder" class="card highlight" hidden>
      <h2>Nächster Schritt: Ordner freigeben</h2>
      <p>Geben Sie den Datenordner Ihrer Software frei. Erst dann kann Replado dort nachsehen.</p>
      <button type="button" id="first-folder-button">Ordner freigeben</button>
    </div>
    <div class="card">
      <h2>Verbindung</h2>
      <p id="connected-text"></p>
      <p id="contact"></p>
    </div>
    <div class="card">
      <h2>Daten</h2>
      <p id="scan-text"></p>
      <p id="folder-summary" class="muted"></p>
      <button type="button" id="rescan" class="secondary">Jetzt neu einlesen</button>
    </div>
    <p class="footer-actions"><button type="button" id="logout" class="link">Abmelden und Verbindung trennen</button></p>
  </section>

  <section id="view-folders" class="view" role="tabpanel" aria-labelledby="tab-folders" tabindex="-1" hidden>
    <div class="card">
      <div class="card-head">
        <h2>Freigegebene Ordner</h2>
        <button type="button" id="open-add" class="secondary">Ordner hinzufügen</button>
      </div>
      <p class="muted">Der Connector liest nur diese Ordner. Er durchsucht nicht den ganzen Rechner und ändert keine Dateien.</p>
      <ul id="folders" class="list"></ul>
      <p id="no-folders" class="empty" hidden>Noch kein Ordner freigegeben.</p>
    </div>
    <div id="add-panel" class="card" hidden>
      <h2>Ordner hinzufügen</h2>
      <label for="path">Ordner</label>
      <input id="path" placeholder="z. B. C:\Topfakt\Daten">
      <div class="actions">
        <button type="button" id="browse" class="secondary">Ordner suchen</button><button type="button" id="check">Prüfen</button><button type="button" id="close-add" class="link">Abbrechen</button>
      </div>
      <div id="browser" class="browser" hidden>
        <p id="browser-path" class="mono"></p>
        <ul id="browser-list" class="list"></ul>
        <div class="actions">
          <button type="button" id="browser-up" class="secondary">Eine Ebene höher</button><button type="button" id="browser-pick">Diesen Ordner wählen</button>
        </div>
      </div>
      <div id="preview" class="preview" hidden>
        <h3>Gefunden in <span id="preview-path" class="mono"></span></h3>
        <ul id="preview-list" class="list"></ul>
        <p id="preview-empty" class="muted" hidden>In diesem Ordner wurde keine Datenbank gefunden. Bitte den Datenordner der Software wählen.</p>
        <label class="confirm"><input type="checkbox" id="confirm"> <span id="confirm-text"></span></label>
        <button type="button" id="add" disabled>Ordner freigeben</button>
      </div>
    </div>
  </section>

  <section id="view-log" class="view" role="tabpanel" aria-labelledby="tab-log" tabindex="-1" hidden>
    <div class="card">
      <div class="card-head">
        <h2>Protokoll</h2>
        <label class="inline">Anzeigen
          <select id="log-filter">
            <option value="">Alle Einträge</option>
            <option value="abfragen">Abfragen von Replado</option>
            <option value="fehler">Nur Fehler</option>
            <option value="einrichtung">Einrichtung und Updates</option>
          </select>
        </label>
      </div>
      <p class="muted">Was der Connector gelesen und an Replado geschickt hat, die neuesten Einträge oben. Inhalte stehen hier nicht, nur Tabelle und Zeilenzahl.</p>
      <ul id="log" class="log"></ul>
      <p id="no-log" class="empty" hidden>Keine Einträge.</p>
      <button type="button" id="more-log" class="secondary" hidden>Ältere Einträge anzeigen</button>
      <p class="small muted">Der Connector behält nur die jüngsten Einträge, insgesamt höchstens etwa 2 MB. Ältere löscht er selbst.</p>
    </div>
  </section>
</main>
<script src="/app.js"></script>
</body>
</html>
`

const uiCSS = `* { box-sizing: border-box; }
[hidden] { display: none !important; }
header, main, section, nav, footer { display: block; }
html { background: #f4f5f2; }
body { margin: 0; background: #f4f5f2; color: #1d231f; font: 16px/1.5 "Segoe UI", system-ui, -apple-system, Tahoma, sans-serif; }
.top { background: #ffffff; border-bottom: 1px solid #e1e5df; }
.top-inner, .tabs, main { max-width: 820px; margin: 0 auto; padding: 0 20px; }
.top-inner { padding-top: 16px; padding-bottom: 14px; overflow: hidden; }
.brand { float: left; }
.logo { display: inline-block; vertical-align: middle; width: 36px; height: 36px; line-height: 36px; text-align: center; border-radius: 9px; background: #1f5c46; color: #ffffff; font-weight: bold; font-size: 18px; margin-right: 10px; }
.brand-text { display: inline-block; vertical-align: middle; }
.name { display: block; font-weight: 600; font-size: 17px; line-height: 1.2; }
.about { display: block; font-size: 13px; color: #5d665f; }
.pill { float: right; margin-top: 6px; padding: 4px 12px 4px 26px; border-radius: 999px; font-size: 14px; font-weight: 600; position: relative; background: #eef0ec; color: #4a524c; }
.pill:before { content: ""; position: absolute; left: 11px; top: 50%; margin-top: -4px; width: 8px; height: 8px; border-radius: 50%; background: #8b938d; }
.pill.ok { background: #e3f1ea; color: #1f5c46; }
.pill.ok:before { background: #2e8a64; }
.pill.warn { background: #fbeceb; color: #a3271f; }
.pill.warn:before { background: #c0392b; }
.tabs { white-space: nowrap; overflow-x: auto; }
.tabs button { background: transparent; color: #4a524c; border: 0; border-bottom: 3px solid transparent; border-radius: 0; margin: 0 22px 0 0; padding: 10px 2px 9px; font-weight: 600; }
.tabs button:hover { color: #1d231f; background: transparent; }
.tabs button[aria-selected="true"] { color: #1f5c46; border-bottom-color: #1f5c46; }
.count { display: inline-block; min-width: 22px; padding: 0 7px; margin-left: 4px; border-radius: 999px; background: #eef0ec; color: #4a524c; font-size: 13px; line-height: 20px; text-align: center; }
.count:empty { display: none; }
main { padding-top: 8px; padding-bottom: 40px; }
h1 { font-size: 22px; margin: 0 0 6px; }
h2 { font-size: 18px; margin: 0 0 8px; }
h3 { font-size: 16px; margin: 16px 0 8px; }
p { margin: 0 0 10px; }
.muted { color: #5d665f; }
.small { font-size: 14px; margin-top: 14px; }
.mono { font-family: Consolas, "Courier New", monospace; word-wrap: break-word; word-break: break-all; }
.card { background: #ffffff; border: 1px solid #e1e5df; border-radius: 14px; padding: 20px 22px; margin: 16px 0; box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04); }
.card.narrow { max-width: 460px; margin: 32px auto; }
.card.highlight { border-color: #9cc9b3; background: #f2f9f5; }
.card-head { overflow: hidden; margin-bottom: 4px; }
.card-head h2 { float: left; margin-top: 6px; }
.card-head button, .card-head .inline { float: right; margin: 0; }
label { display: block; margin: 12px 0; font-weight: 600; }
label input { display: block; width: 100%; margin-top: 4px; font-weight: normal; }
label.inline { font-weight: 600; font-size: 15px; }
label.inline select { margin-left: 8px; }
input, select { font: inherit; padding: 9px 12px; border: 1px solid #c9cec6; border-radius: 9px; background: #ffffff; color: #1d231f; }
#path { width: 100%; margin-top: 4px; }
input:focus, select:focus, button:focus { outline: 3px solid #9cc9b3; outline-offset: 1px; }
.tabs button:focus { outline-offset: -3px; }
button:focus:not(:focus-visible) { outline: 0; }
button { font: inherit; font-weight: 600; padding: 9px 18px; margin: 4px 8px 4px 0; border-radius: 9px; border: 1px solid #1f5c46; background: #1f5c46; color: #ffffff; cursor: pointer; }
button:hover { background: #184a38; }
button.secondary { background: #ffffff; color: #1d231f; border-color: #c9cec6; }
button.secondary:hover { background: #f4f5f2; }
button.link { background: transparent; border-color: transparent; color: #1f5c46; padding-left: 4px; padding-right: 4px; text-decoration: underline; }
button.link:hover { background: transparent; color: #184a38; }
button.wide { width: 100%; margin: 10px 0 0; }
button[disabled] { opacity: 0.5; cursor: default; }
.actions { margin-top: 8px; }
.footer-actions { text-align: right; }
.list { list-style: none; padding: 0; margin: 0; }
.list li { padding: 12px 0; border-bottom: 1px solid #e9ece7; overflow: hidden; }
.list li:last-child { border-bottom: 0; }
.list li > button { float: right; margin: 0 0 0 12px; }
.empty { color: #5d665f; background: #f7f8f5; border-radius: 10px; padding: 14px 16px; margin-top: 8px; }
.browser, .preview { margin-top: 14px; padding: 14px; border: 1px solid #e1e5df; border-radius: 12px; background: #fafbf9; }
.browser .list { max-height: 300px; overflow: auto; }
.browser .list li { padding: 4px 0; border: 0; }
.browser .list button { width: 100%; text-align: left; float: none; margin: 0; }
.confirm { font-weight: normal; background: #f4f5f2; padding: 12px 12px 12px 44px; border-radius: 10px; position: relative; }
.confirm input { position: absolute; left: 12px; top: 14px; width: 22px; height: 22px; margin: 0; }
.alert { background: #fbeceb; color: #a3271f; padding: 12px 16px; border-radius: 10px; margin-top: 16px; }
.ok { color: #1f5c46; font-weight: 600; }
.notice { background: #e3f1ea; color: #1f5c46; padding: 12px 16px; border-radius: 10px; margin-top: 16px; }
.log { list-style: none; padding: 0; margin: 8px 0 12px; }
.log li { padding: 10px 0 10px 16px; border-bottom: 1px solid #e9ece7; position: relative; }
.log li:before { content: ""; position: absolute; left: 0; top: 18px; width: 7px; height: 7px; border-radius: 50%; background: #9cc9b3; }
.log li.failed:before { background: #c0392b; }
.log li.setup:before { background: #8b938d; }
.log .when { float: right; color: #5d665f; font-size: 14px; margin-left: 12px; }
.log .what { font-weight: 600; }
.log .detail { color: #5d665f; font-size: 14px; }
.log .problem { color: #a3271f; font-size: 14px; }
@media (max-width: 560px) {
  .brand { float: none; }
  .pill { float: none; display: inline-block; margin: 10px 0 0 46px; }
  .card-head h2, .card-head button, .card-head .inline { float: none; display: block; }
  .card-head button { margin-top: 8px; }
  .log .when { float: none; display: block; margin: 0; }
}
@media (prefers-color-scheme: dark) {
  html, body { background: #121513; color: #e9ece8; }
  .top { background: #1a1e1b; border-color: #2c322d; }
  .about, .muted, .log .when, .log .detail { color: #a5ada6; }
  .card { background: #1a1e1b; border-color: #2c322d; box-shadow: none; }
  .card.highlight { background: #18291f; border-color: #2f5a44; }
  input, select { background: #121513; color: #e9ece8; border-color: #3a423b; }
  button { background: #5fb38f; border-color: #5fb38f; color: #0e1a14; }
  button:hover { background: #74c4a1; }
  button.secondary { background: transparent; color: #e9ece8; border-color: #3a423b; }
  button.secondary:hover { background: #222823; }
  button.link, button.link:hover { background: transparent; border-color: transparent; color: #8fd3b4; }
  .tabs button, .tabs button:hover { color: #a5ada6; background: transparent; border-color: transparent; }
  .tabs button:hover { color: #e9ece8; }
  .tabs button[aria-selected="true"] { color: #8fd3b4; border-bottom-color: #8fd3b4; }
  .count, .pill { background: #262c27; color: #c9cfca; }
  .pill.ok { background: #1d3229; color: #8fd3b4; }
  .pill.warn { background: #3a1f1d; color: #f19189; }
  .list li, .log li { border-color: #2c322d; }
  .empty, .confirm { background: #121513; }
  .browser, .preview { border-color: #2c322d; background: #151916; }
  .alert { background: #3a1f1d; color: #f19189; }
  .log .problem { color: #f19189; }
  .ok { color: #8fd3b4; }
  .notice { background: #1d3229; color: #8fd3b4; }
}
`

const uiJS = `(function () {
  'use strict';
  var token = '';
  try { token = window.sessionStorage.getItem('replado-token') || ''; } catch (e) {}
  var fromQuery = /[?&]t=([A-Za-z0-9_-]+)/.exec(location.search);
  if (fromQuery) {
    token = fromQuery[1];
    try { window.sessionStorage.setItem('replado-token', token); } catch (e) {}
    if (window.history && window.history.replaceState) window.history.replaceState(null, '', '/');
  } else if (location.hash.length > 1) {
    token = location.hash.substring(1);
    try { window.sessionStorage.setItem('replado-token', token); } catch (e) {}
    if (window.history && window.history.replaceState) window.history.replaceState(null, '', '/');
  }
  var $ = function (id) { return document.getElementById(id); };
  var VIEWS = ['overview', 'folders', 'log'];
  var PAGE = 25;
  var state = null;
  var browsing = '';
  var view = 'overview';
  var logLimit = PAGE;
  try { view = window.sessionStorage.getItem('replado-view') || 'overview'; } catch (e) {}

  function show(node, visible) {
    if (visible) node.removeAttribute('hidden'); else node.setAttribute('hidden', 'hidden');
  }

  function disable(node, off) {
    node.disabled = !!off;
  }

  function el(tag, text, cls) {
    var node = document.createElement(tag);
    if (text !== undefined && text !== null) node.textContent = text;
    if (cls) node.className = cls;
    return node;
  }

  function showError(message) {
    $('error').textContent = message || '';
    show($('error'), !!message);
  }

  function api(action, data, done, failed) {
    var xhr = new XMLHttpRequest();
    xhr.open('POST', '/api/' + action, true);
    xhr.setRequestHeader('Content-Type', 'application/json');
    xhr.setRequestHeader('X-Replado-Token', token);
    xhr.onreadystatechange = function () {
      if (xhr.readyState !== 4) return;
      var body = {};
      try { body = xhr.responseText ? JSON.parse(xhr.responseText) : {}; } catch (e) { body = { error: xhr.responseText }; }
      if (xhr.status >= 200 && xhr.status < 300) {
        if (done) done(body);
        return;
      }
      var message = xhr.status === 401 ? 'Diese Seite ist abgelaufen. Bitte den Replado Connector neu öffnen.' : (xhr.status === 0 ? 'Der Replado Connector ist nicht mehr geöffnet. Bitte neu starten.' : (body.error || 'Unbekannter Fehler'));
      if (failed) failed(message); else showError(message);
    };
    xhr.send(JSON.stringify(data || {}));
  }

  function when(value) {
    if (!value) return '';
    var m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/.exec(value);
    if (!m) return value;
    return m[3] + '.' + m[2] + '.' + m[1] + ', ' + m[4] + ':' + m[5];
  }

  function select(next, focus) {
    var known = false;
    for (var i = 0; i < VIEWS.length; i++) if (VIEWS[i] === next) known = true;
    view = known ? next : 'overview';
    try { window.sessionStorage.setItem('replado-view', view); } catch (e) {}
    for (var j = 0; j < VIEWS.length; j++) {
      var name = VIEWS[j];
      var tab = $('tab-' + name);
      var active = name === view;
      tab.setAttribute('aria-selected', active ? 'true' : 'false');
      tab.tabIndex = active ? 0 : -1;
      show($('view-' + name), active && state && state.connected);
    }
    if (focus) $('tab-' + view).focus();
    if (view === 'log') loadLog();
  }

  function setPill(text, kind) {
    $('pill').textContent = text;
    $('pill').className = 'pill' + (kind ? ' ' + kind : '');
  }

  function render(s) {
    var first = state === null;
    state = s;
    $('about').textContent = 'Version ' + s.version + ' · ' + s.os;
    $('outdated').textContent = s.outdated ? 'Der Replado Connector wurde auf Version ' + s.version + ' aktualisiert. Bitte diese Seite schließen und die Einstellungen über das Symbol neben der Uhr neu öffnen.' : '';
    show($('outdated'), !!s.outdated);
    show($('setup'), !s.connected);
    show($('tabs'), s.connected);
    show($('installed'), s.connected && s.downloaded);
    var st = s.status || {};
    var folders = s.folders || [];
    if (!s.connected) {
      setPill('Nicht verbunden', '');
      if (!$('server').value) $('server').value = s.server || '';
    } else if (st.last_error) {
      setPill('Problem', 'warn');
    } else if (st.last_contact) {
      setPill('Verbunden', 'ok');
    } else {
      setPill('Startet', '');
    }
    if (s.connected) {
      $('connected-text').textContent = 'Verbunden mit ' + s.server + ' als ' + s.account + '.';
      if (st.last_error) {
        $('contact').textContent = 'Problem: ' + st.last_error;
        $('contact').className = 'alert';
      } else if (st.last_contact) {
        $('contact').textContent = 'Letzter Kontakt mit Replado: ' + when(st.last_contact);
        $('contact').className = 'ok';
      } else {
        $('contact').textContent = 'Der Hintergrunddienst startet gerade.';
        $('contact').className = 'muted';
      }
    }
    show($('first-folder'), s.connected && !folders.length);
    $('folder-count').textContent = folders.length ? String(folders.length) : '';
    $('folder-summary').textContent = folders.length === 1 ? '1 Ordner freigegeben.' : (folders.length ? folders.length + ' Ordner freigegeben.' : 'Noch kein Ordner freigegeben.');
    var list = $('folders');
    list.textContent = '';
    folders.forEach(function (f) {
      var item = el('li');
      var text = el('div');
      text.appendChild(el('div', f.path, 'mono'));
      text.appendChild(el('div', 'Lizenz bestätigt am ' + when(f.confirmed_at), 'muted'));
      var remove = el('button', 'Entfernen', 'secondary');
      remove.type = 'button';
      remove.setAttribute('aria-label', 'Freigabe entfernen: ' + f.path);
      remove.onclick = function () { run('remove-folder', { path: f.path }); };
      item.appendChild(remove);
      item.appendChild(text);
      list.appendChild(item);
    });
    show($('no-folders'), !folders.length);
    $('scan-text').textContent = st.scanned_at
      ? 'Zuletzt eingelesen: ' + when(st.scanned_at) + ' · ' + (st.sources === 1 ? '1 Datenbank gefunden.' : st.sources + ' Datenbanken gefunden.')
      : 'Noch nicht eingelesen. Das geschieht automatisch nach der Freigabe eines Ordners.';
    $('confirm-text').textContent = s.confirmation;
    if (first || !s.connected) select(view, false);
    else for (var i = 0; i < VIEWS.length; i++) show($('view-' + VIEWS[i]), VIEWS[i] === view);
  }

  function run(action, data, after) {
    showError('');
    api(action, data, function (s) {
      if (s && s.version) render(s);
      if (view === 'log') loadLog();
      if (after) after(s);
    });
  }

  function describe(e) {
    if (e.kind === 'abfrage') return ({ describe_table: 'Aufbau gelesen', query: 'Zeilen gelesen', search: 'Gesucht' }[e.tool] || e.tool);
    return e.text || e.kind;
  }

  function loadLog() {
    if (!state || !state.connected) return;
    api('log', { filter: $('log-filter').value, limit: logLimit }, function (r) {
      var list = $('log');
      list.textContent = '';
      var entries = r.entries || [];
      entries.forEach(function (e) {
        var item = el('li', null, e.error ? 'failed' : (e.kind === 'abfrage' ? '' : 'setup'));
        item.appendChild(el('span', when(e.time), 'when'));
        item.appendChild(el('div', describe(e), 'what'));
        var parts = [];
        if (e.table) parts.push('Tabelle ' + e.table);
        if (e.kind === 'abfrage') parts.push((e.rows || 0) === 1 ? '1 Zeile' : (e.rows || 0) + ' Zeilen');
        if (parts.length) item.appendChild(el('div', parts.join(' · '), 'detail'));
        if (e.error) item.appendChild(el('div', e.error, 'problem'));
        list.appendChild(item);
      });
      show($('no-log'), !entries.length);
      show($('more-log'), !!r.more);
    }, function () {});
  }

  function browse(path) {
    api('dirs', { path: path }, function (r) {
      browsing = r.path;
      show($('browser'), true);
      $('browser-path').textContent = r.path || 'Laufwerke und Speicherorte';
      disable($('browser-up'), !r.path);
      disable($('browser-pick'), !r.path);
      var list = $('browser-list');
      list.textContent = '';
      (r.dirs || []).forEach(function (d) {
        var item = el('li');
        var open = el('button', d.name, 'secondary');
        open.type = 'button';
        open.onclick = function () { browse(d.path); };
        item.appendChild(open);
        list.appendChild(item);
      });
      if (!(r.dirs || []).length) list.appendChild(el('li', 'Keine Unterordner.', 'muted'));
      $('browser-up').onclick = function () { browse(r.parent || ''); };
    });
  }

  function preview() {
    showError('');
    show($('preview'), false);
    $('confirm').checked = false;
    disable($('add'), true);
    api('preview', { path: $('path').value }, function (r) {
      $('path').value = r.path;
      $('preview-path').textContent = r.path;
      var list = $('preview-list');
      list.textContent = '';
      r.sources.forEach(function (s) {
        var item = el('li');
        var text = el('div');
        text.appendChild(el('strong', s.software || 'Unbekannte Software'));
        text.appendChild(el('div', s.path, 'mono'));
        var rows = 0;
        for (var i = 0; i < s.tables.length; i++) rows += s.tables[i].rows;
        text.appendChild(el('div', s.tables.length + ' Tabellen, ' + rows + ' Zeilen', 'muted'));
        item.appendChild(text);
        list.appendChild(item);
      });
      show($('preview-empty'), !r.sources.length);
      $('confirm-text').textContent = r.confirmation;
      show($('preview'), true);
    });
  }

  function openAdd() {
    select('folders', false);
    show($('add-panel'), true);
    $('path').focus();
  }

  function closeAdd() {
    show($('add-panel'), false);
    show($('browser'), false);
    show($('preview'), false);
    $('path').value = '';
  }

  function tabKeys(event) {
    var key = event.keyCode;
    var index = 0;
    for (var i = 0; i < VIEWS.length; i++) if (VIEWS[i] === view) index = i;
    if (key === 39) index = (index + 1) % VIEWS.length;
    else if (key === 37) index = (index + VIEWS.length - 1) % VIEWS.length;
    else if (key === 36) index = 0;
    else if (key === 35) index = VIEWS.length - 1;
    else return;
    if (event.preventDefault) event.preventDefault();
    select(VIEWS[index], true);
  }

  function start() {
    if (!token) {
      showError('Diese Seite ist abgelaufen. Bitte den Replado Connector neu öffnen.');
      return;
    }
    $('login').onsubmit = function (event) {
      if (event && event.preventDefault) event.preventDefault();
      disable($('login-button'), true);
      showError('');
      api('login', { server: $('server').value, email: $('email').value, password: $('password').value, code: $('code').value }, function (r) {
        disable($('login-button'), false);
        if (r.second_factor) {
          show($('code-label'), true);
          $('code').focus();
          showError(r.message);
          return;
        }
        $('password').value = '';
        $('code').value = '';
        view = 'overview';
        render(r);
        select('overview', false);
      }, function (message) {
        disable($('login-button'), false);
        showError(message);
      });
      return false;
    };
    for (var i = 0; i < VIEWS.length; i++) {
      $('tab-' + VIEWS[i]).onclick = function () { select(this.getAttribute('data-view'), false); };
      $('tab-' + VIEWS[i]).onkeydown = tabKeys;
    }
    $('logout').onclick = function () { run('logout'); };
    $('first-folder-button').onclick = openAdd;
    $('open-add').onclick = openAdd;
    $('close-add').onclick = closeAdd;
    $('browse').onclick = function () { browse($('path').value); };
    $('browser-pick').onclick = function () {
      $('path').value = browsing;
      show($('browser'), false);
      preview();
    };
    $('check').onclick = preview;
    $('confirm').onclick = function () { disable($('add'), !$('confirm').checked); };
    $('add').onclick = function () {
      run('add-folder', { path: $('path').value, confirmed: $('confirm').checked }, function (s) {
        if (s && s.version) closeAdd();
      });
    };
    $('rescan').onclick = function () {
      run('rescan', {}, function () { $('scan-text').textContent = 'Wird neu eingelesen …'; });
    };
    $('log-filter').onchange = function () { logLimit = PAGE; loadLog(); };
    $('more-log').onclick = function () { logLimit += PAGE; loadLog(); };
    run('state');
    setInterval(function () {
      api('state', {}, render, function () {});
      if (view === 'log') loadLog();
    }, 10000);
  }

  if (document.readyState === 'loading') {
    document.attachEvent ? document.attachEvent('onreadystatechange', function () { if (document.readyState === 'complete') start(); }) : document.addEventListener('DOMContentLoaded', start);
  } else {
    start();
  }
})();
`
