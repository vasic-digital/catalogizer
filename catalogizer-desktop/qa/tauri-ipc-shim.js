// QA-only browser shim for the Tauri IPC bridge (CAT-TAURI-1). NOT part of the shipped app:
// it is injected only by vite.qa.config.ts so the real React UI can be exercised in a plain browser
// over the LAN. It mirrors the Rust commands in src-tauri/src/main.rs (config state + make_http_request)
// and forwards HTTP to the real Catalogizer API through the vite proxy (/qa-api -> QA_API_URL).
(function () {
  var cfg = { server_url: location.origin + '/qa-api', auth_token: null, theme: 'dark', auto_start: false };
  var cmds = {
    get_config: function () { return Object.assign({}, cfg); },
    update_config: function (a) { cfg = Object.assign({}, a.newConfig); },
    set_server_url: function (a) { cfg.server_url = a.url; },
    set_auth_token: function (a) { cfg.auth_token = a.token; },
    clear_auth_token: function () { cfg.auth_token = null; },
    get_app_version: function () { return 'qa-browser-shim'; },
    get_platform: function () { return 'linux'; },
    get_arch: function () { return 'x86_64'; },
    make_http_request: function (a) {
      var base = String(cfg.server_url || '').replace(/\/+$/, '');
      if (String(a.url).replace(/\/+$/, '').indexOf(base) !== 0) {
        return Promise.reject('URL must be within configured server: ' + cfg.server_url);
      }
      return fetch(a.url, { method: (a.method || 'GET').toUpperCase(), headers: a.headers || {}, body: a.body })
        .then(function (r) { return r.text(); })
        .catch(function (e) { return Promise.reject('Request failed: ' + e); });
    }
  };
  window.__QA_IPC_CALLS__ = [];
  window.__TAURI_INTERNALS__ = Object.assign(window.__TAURI_INTERNALS__ || {}, {
    invoke: function (cmd, args) {
      window.__QA_IPC_CALLS__.push(cmd);
      if (!cmds[cmd]) return Promise.reject('QA shim: command not available in browser: ' + cmd);
      try { return Promise.resolve(cmds[cmd](args || {})); } catch (e) { return Promise.reject(String(e)); }
    },
    transformCallback: function () { return 0; },
    metadata: { currentWindow: { label: 'main' }, currentWebview: { label: 'main', windowLabel: 'main' } }
  });
})();
