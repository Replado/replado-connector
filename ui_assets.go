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
<header>
  <h1>Replado Connector</h1>
  <p id="about" class="muted"></p>
</header>
<main>
  <div id="outdated" class="notice" role="status" hidden></div>
  <div id="error" class="alert" role="alert" hidden></div>
  <section class="card" aria-labelledby="h-connection">
    <h2 id="h-connection">1. Mit Replado verbinden</h2>
    <div id="connected" hidden>
      <p id="connected-text"></p>
      <p id="contact" class="muted"></p>
      <p id="installed" class="notice" role="status" hidden>Der Replado Connector ist jetzt auf diesem Rechner installiert und startet automatisch mit. Die heruntergeladene Datei wird nicht mehr gebraucht und kann gelöscht werden.</p>
      <button type="button" id="logout" class="secondary">Abmelden</button>
    </div>
    <form id="login" hidden>
      <p class="muted">Melden Sie sich mit Ihrem Replado-Zugang an. Das geht nur als Administrator.</p>
      <label>Adresse Ihres Replado<input id="server" type="url" autocomplete="url" required placeholder="https://replado.ihre-firma.de"></label>
      <label>E-Mail<input id="email" type="email" autocomplete="username" required></label>
      <label>Passwort<input id="password" type="password" autocomplete="current-password" required></label>
      <label id="code-label" hidden>Code aus der Authenticator-App<input id="code" inputmode="numeric" autocomplete="one-time-code"></label>
      <button type="submit" id="login-button">Anmelden</button>
    </form>
  </section>
  <section class="card" aria-labelledby="h-folders">
    <h2 id="h-folders">2. Ordner der Software freigeben</h2>
    <p class="muted">Der Connector liest nur die Ordner, die hier stehen. Er durchsucht nicht den ganzen Rechner und ändert keine Dateien.</p>
    <ul id="folders" class="list"></ul>
    <p id="no-folders" class="muted" hidden>Noch kein Ordner freigegeben.</p>
    <h3>Ordner hinzufügen</h3>
    <div class="row">
      <input id="path" aria-label="Ordner" placeholder="z. B. C:\Topfakt\Daten">
      <button type="button" id="browse" class="secondary">Ordner suchen</button>
      <button type="button" id="check">Prüfen</button>
    </div>
    <div id="browser" class="browser" hidden>
      <p id="browser-path" class="mono"></p>
      <ul id="browser-list" class="list"></ul>
      <div class="row">
        <button type="button" id="browser-up" class="secondary">Eine Ebene höher</button>
        <button type="button" id="browser-pick">Diesen Ordner wählen</button>
      </div>
    </div>
    <div id="preview" class="preview" hidden>
      <h3>Gefunden in <span id="preview-path" class="mono"></span></h3>
      <ul id="preview-list" class="list"></ul>
      <p id="preview-empty" class="muted" hidden>In diesem Ordner wurde keine Datenbank gefunden. Bitte den Datenordner der Software wählen.</p>
      <label class="confirm"><input type="checkbox" id="confirm"> <span id="confirm-text"></span></label>
      <button type="button" id="add" disabled>Ordner freigeben</button>
    </div>
  </section>
  <section class="card" aria-labelledby="h-scan">
    <h2 id="h-scan">3. Daten einlesen</h2>
    <p id="scan-text"></p>
    <button type="button" id="rescan" class="secondary">Jetzt neu einlesen</button>
  </section>
  <section class="card" aria-labelledby="h-log">
    <h2 id="h-log">Protokoll</h2>
    <p class="muted">Was der Connector gelesen und an Replado geschickt hat. Inhalte stehen hier nicht, nur Tabelle und Zeilenzahl.</p>
    <div class="table-wrap">
      <table>
        <thead><tr><th>Zeit</th><th>Vorgang</th><th>Tabelle</th><th>Zeilen</th><th>Hinweis</th></tr></thead>
        <tbody id="log"></tbody>
      </table>
    </div>
    <p id="no-log" class="muted" hidden>Noch keine Einträge.</p>
    <button type="button" id="reload-log" class="secondary">Protokoll aktualisieren</button>
  </section>
</main>
<script src="/app.js"></script>
</body>
</html>
`

const uiCSS = `* { box-sizing: border-box; }
[hidden] { display: none !important; }
header, main, section, nav, footer { display: block; }
body { margin: 0; background: #f4f4f1; color: #1d231f; font: 17px/1.5 "Segoe UI", system-ui, -apple-system, Tahoma, sans-serif; }
header, main { max-width: 860px; margin: 0 auto; padding: 0 16px; }
header { padding-top: 24px; }
h1 { font-size: 1.6em; margin: 0; }
h2 { font-size: 1.2em; margin: 0 0 8px; }
h3 { font-size: 1em; margin: 16px 0 8px; }
.muted { color: #5d665f; }
.mono { font-family: Consolas, "Courier New", monospace; word-wrap: break-word; word-break: break-all; }
.card { background: #ffffff; border: 1px solid #dfe3dc; border-radius: 12px; padding: 20px; margin: 16px 0; }
label { display: block; margin: 10px 0; font-weight: bold; }
label input { display: block; width: 100%; margin-top: 4px; font-weight: normal; }
input { font: inherit; padding: 10px 12px; border: 1px solid #c9cec6; border-radius: 8px; background: #fbfbf9; color: #1d231f; }
button { font: inherit; font-weight: bold; padding: 10px 18px; margin: 4px 8px 4px 0; border-radius: 8px; border: 1px solid #1f5c46; background: #1f5c46; color: #ffffff; cursor: pointer; }
button.secondary { background: #ffffff; color: #1d231f; border-color: #c9cec6; }
button[disabled] { opacity: 0.5; cursor: default; }
.row input { width: 100%; margin-bottom: 6px; }
.list { list-style: none; padding: 0; margin: 0; }
.list li { padding: 10px 0; border-bottom: 1px solid #dfe3dc; overflow: hidden; }
.list li:last-child { border-bottom: 0; }
.list li > button { float: right; margin: 0 0 0 12px; }
.browser, .preview { margin-top: 12px; padding: 12px; border: 1px solid #dfe3dc; border-radius: 10px; }
.browser .list { max-height: 320px; overflow: auto; }
.browser .list button { width: 100%; text-align: left; float: none; margin: 0; }
.confirm { font-weight: normal; background: #f4f4f1; padding: 12px 12px 12px 44px; border-radius: 8px; position: relative; }
.confirm input { position: absolute; left: 12px; top: 14px; width: 22px; height: 22px; margin: 0; }
.alert { background: #fbeceb; color: #a3271f; padding: 12px 16px; border-radius: 8px; margin-top: 16px; }
.ok { color: #1f5c46; font-weight: bold; }
.notice { background: #e6f2ec; color: #1f5c46; padding: 12px 16px; border-radius: 8px; margin-top: 16px; }
.table-wrap { overflow-x: auto; }
table { width: 100%; border-collapse: collapse; font-size: 0.95em; }
th, td { text-align: left; padding: 6px 8px; border-bottom: 1px solid #dfe3dc; vertical-align: top; }
@media (prefers-color-scheme: dark) {
  body { background: #141815; color: #e9ece8; }
  .muted { color: #a5ada6; }
  .card { background: #1d221e; border-color: #2f3630; }
  input { background: #141815; color: #e9ece8; border-color: #3a423b; }
  button { background: #5fb38f; border-color: #5fb38f; color: #0e1a14; }
  button.secondary { background: transparent; color: #e9ece8; border-color: #3a423b; }
  .list li, th, td { border-color: #2f3630; }
  .browser, .preview { border-color: #2f3630; }
  .confirm { background: #141815; }
  .alert { background: #3a1f1d; color: #f19189; }
  .ok { color: #5fb38f; }
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
  var state = null;
  var browsing = '';

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

  function render(s) {
    state = s;
    $('about').textContent = 'Version ' + s.version + ' · ' + s.os;
    $('outdated').textContent = s.outdated ? 'Der Replado Connector wurde auf Version ' + s.version + ' aktualisiert. Bitte diese Seite schließen und die Einstellungen über das Symbol neben der Uhr neu öffnen.' : '';
    show($('outdated'), !!s.outdated);
    show($('connected'), s.connected);
    show($('login'), !s.connected);
    show($('installed'), s.connected && s.downloaded);
    if (s.connected) {
      $('connected-text').textContent = 'Verbunden mit ' + s.server + ' als ' + s.account + '.';
      var st = s.status || {};
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
    } else if (!$('server').value) {
      $('server').value = s.server || '';
    }
    var list = $('folders');
    list.textContent = '';
    (s.folders || []).forEach(function (f) {
      var item = el('li');
      var text = el('div');
      text.appendChild(el('div', f.path, 'mono'));
      text.appendChild(el('div', 'Lizenz bestätigt am ' + when(f.confirmed_at), 'muted'));
      item.appendChild(text);
      var remove = el('button', 'Entfernen', 'secondary');
      remove.type = 'button';
      remove.onclick = function () { run('remove-folder', { path: f.path }); };
      item.appendChild(remove);
      list.appendChild(item);
    });
    show($('no-folders'), !(s.folders || []).length);
    var status = s.status || {};
    $('scan-text').textContent = status.scanned_at
      ? 'Zuletzt eingelesen: ' + when(status.scanned_at) + ' · ' + (status.sources === 1 ? '1 Datenbank gefunden.' : status.sources + ' Datenbanken gefunden.')
      : 'Noch nicht eingelesen. Das geschieht automatisch nach der Freigabe eines Ordners.';
    $('confirm-text').textContent = s.confirmation;
  }

  function run(action, data, after) {
    showError('');
    api(action, data, function (s) {
      if (s && s.version) { render(s); loadLog(); }
      if (after) after(s);
    });
  }

  function loadLog() {
    api('log', {}, function (entries) {
      var body = $('log');
      body.textContent = '';
      entries.forEach(function (e) {
        var row = el('tr');
        row.appendChild(el('td', when(e.time)));
        var what = e.kind === 'abfrage' ? ({ describe_table: 'Aufbau gelesen', query: 'Zeilen gelesen', search: 'Gesucht' }[e.tool] || e.tool) : (e.text || e.kind);
        row.appendChild(el('td', what));
        row.appendChild(el('td', e.table || ''));
        row.appendChild(el('td', e.kind === 'abfrage' ? String(e.rows || 0) : ''));
        row.appendChild(el('td', e.error || ''));
        body.appendChild(row);
      });
      show($('no-log'), !entries.length);
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
        render(r);
        loadLog();
      }, function (message) {
        disable($('login-button'), false);
        showError(message);
      });
      return false;
    };
    $('logout').onclick = function () { run('logout'); };
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
        if (s && s.version) {
          show($('preview'), false);
          $('path').value = '';
        }
      });
    };
    $('rescan').onclick = function () {
      run('rescan', {}, function () { $('scan-text').textContent = 'Wird neu eingelesen …'; });
    };
    $('reload-log').onclick = loadLog;
    run('state');
    loadLog();
    setInterval(function () { api('state', {}, render, function () {}); loadLog(); }, 10000);
  }

  if (document.readyState === 'loading') {
    document.attachEvent ? document.attachEvent('onreadystatechange', function () { if (document.readyState === 'complete') start(); }) : document.addEventListener('DOMContentLoaded', start);
  } else {
    start();
  }
})();
`
