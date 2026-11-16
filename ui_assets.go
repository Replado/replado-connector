package main

const uiHTML = `<!doctype html>
<html lang="de">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Replado Connector</title>
<link rel="stylesheet" href="/app.css">
<script src="/app.js" defer></script>
</head>
<body>
<header>
  <h1>Replado Connector</h1>
  <p id="about" class="muted"></p>
</header>
<main>
  <div id="error" class="alert" role="alert" hidden></div>
  <section class="card" aria-labelledby="h-connection">
    <h2 id="h-connection">1. Mit Replado verbinden</h2>
    <div id="connected" hidden>
      <p id="connected-text"></p>
      <p id="contact" class="muted"></p>
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
</body>
</html>
`

const uiCSS = `:root {
  --bg: #f4f4f1;
  --card: #ffffff;
  --text: #1d231f;
  --muted: #5d665f;
  --border: #dfe3dc;
  --accent: #1f5c46;
  --accent-text: #ffffff;
  --danger: #a3271f;
  --danger-bg: #fbeceb;
  color-scheme: light dark;
}
@media (prefers-color-scheme: dark) {
  :root {
    --bg: #141815;
    --card: #1d221e;
    --text: #e9ece8;
    --muted: #a5ada6;
    --border: #2f3630;
    --accent: #5fb38f;
    --accent-text: #0e1a14;
    --danger: #f19189;
    --danger-bg: #3a1f1d;
  }
}
* { box-sizing: border-box; }
[hidden] { display: none !important; }
body { margin: 0; background: var(--bg); color: var(--text); font: 17px/1.5 system-ui, -apple-system, "Segoe UI", sans-serif; }
header, main { max-width: 860px; margin: 0 auto; padding: 0 16px; }
header { padding-top: 24px; }
h1 { font-size: 1.6rem; margin: 0; }
h2 { font-size: 1.2rem; margin: 0 0 8px; }
h3 { font-size: 1rem; margin: 16px 0 8px; }
.muted { color: var(--muted); }
.mono { font-family: ui-monospace, Consolas, monospace; word-break: break-all; }
.card { background: var(--card); border: 1px solid var(--border); border-radius: 12px; padding: 20px; margin: 16px 0; }
label { display: block; margin: 10px 0; font-weight: 600; }
label input { display: block; width: 100%; margin-top: 4px; font-weight: 400; }
input { font: inherit; padding: 10px 12px; border: 1px solid var(--border); border-radius: 8px; background: var(--bg); color: var(--text); }
button { font: inherit; font-weight: 600; padding: 10px 18px; border-radius: 8px; border: 1px solid var(--accent); background: var(--accent); color: var(--accent-text); cursor: pointer; }
button.secondary { background: transparent; color: var(--text); border-color: var(--border); }
button:disabled { opacity: 0.5; cursor: not-allowed; }
.row { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; }
.row input { flex: 1 1 260px; }
.list { list-style: none; padding: 0; margin: 0; }
.list li { display: flex; justify-content: space-between; align-items: center; gap: 12px; flex-wrap: wrap; padding: 10px 0; border-bottom: 1px solid var(--border); }
.list li:last-child { border-bottom: 0; }
.browser, .preview { margin-top: 12px; padding: 12px; border: 1px solid var(--border); border-radius: 10px; }
.browser .list { max-height: 320px; overflow: auto; }
.browser .list button { width: 100%; text-align: left; }
.confirm { display: flex; gap: 10px; align-items: flex-start; font-weight: 400; background: var(--bg); padding: 12px; border-radius: 8px; }
.confirm input { width: 22px; height: 22px; margin: 2px 0 0; flex: none; }
.alert { background: var(--danger-bg); color: var(--danger); padding: 12px 16px; border-radius: 8px; margin-top: 16px; }
.ok { color: var(--accent); font-weight: 600; }
.table-wrap { overflow-x: auto; }
table { width: 100%; border-collapse: collapse; font-size: 0.95rem; }
th, td { text-align: left; padding: 6px 8px; border-bottom: 1px solid var(--border); vertical-align: top; }
`

const uiJS = `(function () {
  'use strict';
  var token = '';
  try { token = sessionStorage.getItem('replado-token') || ''; } catch (e) {}
  if (location.hash.length > 1) {
    token = location.hash.slice(1);
    try { sessionStorage.setItem('replado-token', token); } catch (e) {}
    history.replaceState(null, '', '/');
  }
  var $ = function (id) { return document.getElementById(id); };
  var state = null;
  var browsing = '';

  function el(tag, text, cls) {
    var node = document.createElement(tag);
    if (text !== undefined && text !== null) node.textContent = text;
    if (cls) node.className = cls;
    return node;
  }

  function showError(message) {
    $('error').textContent = message || '';
    $('error').hidden = !message;
  }

  function api(action, data) {
    return fetch('/api/' + action, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Replado-Token': token },
      body: JSON.stringify(data || {})
    }).then(function (response) {
      return response.text().then(function (text) {
        var body = {};
        try { body = text ? JSON.parse(text) : {}; } catch (e) { body = { error: text }; }
        if (!response.ok) {
          if (response.status === 401) throw new Error('Diese Seite ist abgelaufen. Bitte den Replado Connector neu öffnen.');
          throw new Error(body.error || 'Unbekannter Fehler');
        }
        return body;
      });
    });
  }

  function when(value) {
    if (!value) return '';
    var d = new Date(value);
    if (isNaN(d.getTime())) return value;
    return d.toLocaleString('de-DE', { dateStyle: 'medium', timeStyle: 'short' });
  }

  function render(s) {
    state = s;
    $('about').textContent = 'Version ' + s.version + ' · ' + s.os;
    $('connected').hidden = !s.connected;
    $('login').hidden = s.connected;
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
      remove.addEventListener('click', function () {
        run(api('remove-folder', { path: f.path }));
      });
      item.appendChild(remove);
      list.appendChild(item);
    });
    $('no-folders').hidden = (s.folders || []).length > 0;
    var status = s.status || {};
    $('scan-text').textContent = status.scanned_at
      ? 'Zuletzt eingelesen: ' + when(status.scanned_at) + ' · ' + (status.sources === 1 ? '1 Datenbank gefunden.' : status.sources + ' Datenbanken gefunden.')
      : 'Noch nicht eingelesen. Das geschieht automatisch nach der Freigabe eines Ordners.';
    $('confirm-text').textContent = s.confirmation;
  }

  function run(promise) {
    showError('');
    return promise.then(function (s) { if (s && s.version) { render(s); loadLog(); } return s; }).catch(function (e) { showError(e.message); });
  }

  function loadLog() {
    return api('log').then(function (entries) {
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
      $('no-log').hidden = entries.length > 0;
    }).catch(function (e) { showError(e.message); });
  }

  function browse(path) {
    api('dirs', { path: path }).then(function (r) {
      browsing = r.path;
      $('browser').hidden = false;
      $('browser-path').textContent = r.path || 'Laufwerke und Speicherorte';
      $('browser-up').disabled = !r.path;
      $('browser-pick').disabled = !r.path;
      var list = $('browser-list');
      list.textContent = '';
      (r.dirs || []).forEach(function (d) {
        var item = el('li');
        var open = el('button', d.name, 'secondary');
        open.type = 'button';
        open.addEventListener('click', function () { browse(d.path); });
        item.appendChild(open);
        list.appendChild(item);
      });
      if (!(r.dirs || []).length) list.appendChild(el('li', 'Keine Unterordner.', 'muted'));
      $('browser-up').onclick = function () { browse(r.parent || ''); };
    }).catch(function (e) { showError(e.message); });
  }

  function preview() {
    showError('');
    $('preview').hidden = true;
    $('confirm').checked = false;
    $('add').disabled = true;
    api('preview', { path: $('path').value }).then(function (r) {
      $('path').value = r.path;
      $('preview-path').textContent = r.path;
      var list = $('preview-list');
      list.textContent = '';
      r.sources.forEach(function (s) {
        var item = el('li');
        var text = el('div');
        text.appendChild(el('strong', s.software || 'Unbekannte Software'));
        text.appendChild(el('div', s.path, 'mono'));
        var rows = s.tables.reduce(function (sum, t) { return sum + t.rows; }, 0);
        text.appendChild(el('div', s.tables.length + ' Tabellen, ' + rows.toLocaleString('de-DE') + ' Zeilen', 'muted'));
        item.appendChild(text);
        list.appendChild(item);
      });
      $('preview-empty').hidden = r.sources.length > 0;
      $('confirm-text').textContent = r.confirmation;
      $('preview').hidden = false;
    }).catch(function (e) { showError(e.message); });
  }

  document.addEventListener('DOMContentLoaded', function () {
    if (!token) {
      showError('Diese Seite ist abgelaufen. Bitte den Replado Connector neu öffnen.');
      return;
    }
    $('login').addEventListener('submit', function (event) {
      event.preventDefault();
      $('login-button').disabled = true;
      showError('');
      api('login', { server: $('server').value, email: $('email').value, password: $('password').value, code: $('code').value })
        .then(function (r) {
          if (r.second_factor) {
            $('code-label').hidden = false;
            $('code').focus();
            showError(r.message);
            return;
          }
          $('password').value = '';
          $('code').value = '';
          render(r);
        })
        .catch(function (e) { showError(e.message); })
        .then(function () { $('login-button').disabled = false; });
    });
    $('logout').addEventListener('click', function () { run(api('logout')); });
    $('browse').addEventListener('click', function () { browse($('path').value); });
    $('browser-pick').addEventListener('click', function () {
      $('path').value = browsing;
      $('browser').hidden = true;
      preview();
    });
    $('check').addEventListener('click', preview);
    $('confirm').addEventListener('change', function () { $('add').disabled = !$('confirm').checked; });
    $('add').addEventListener('click', function () {
      run(api('add-folder', { path: $('path').value, confirmed: $('confirm').checked })).then(function (s) {
        if (s && s.version) {
          $('preview').hidden = true;
          $('path').value = '';
        }
      });
    });
    $('rescan').addEventListener('click', function () {
      run(api('rescan')).then(function () { $('scan-text').textContent = 'Wird neu eingelesen …'; });
    });
    $('reload-log').addEventListener('click', loadLog);
    run(api('state'));
    loadLog();
    setInterval(function () { api('state').then(render).catch(function () {}); loadLog(); }, 10000);
  });
})();
`
