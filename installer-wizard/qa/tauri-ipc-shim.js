// QA-only browser shim for the installer-wizard Tauri IPC bridge (CAT-TAURI-1). NOT part of the shipped app.
// The wizard's backend commands touch the local host (network/SMB scan, file dialogs, config files); a
// browser cannot do those, so this shim answers with honest empty/default values. It lets the wizard's
// real UI render and navigate; it does NOT prove the Rust commands (that needs the native binary).
(function () {
  var cmds = {
    scan_network: function () { return []; },
    scan_smb_shares: function () { return []; },
    browse_smb_share: function () { return []; },
    load_configuration: function () { return Promise.reject('QA shim: no filesystem in browser'); },
    save_configuration: function () { return Promise.reject('QA shim: no filesystem in browser'); },
    get_default_config_path: function () { return '/qa-shim/config.json'; }
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
