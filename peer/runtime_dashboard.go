package main

import (
	"net/http"
)

func (s *OpsServer) dashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(nodeDashboardHTML))
}

func (s *OpsServer) adminDashboardStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method_not_allowed"})
		return
	}
	if !s.isAdminAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "admin_auth_required"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"adminEnabled": true,
		"actions": []string{
			"manual_mine",
			"mint",
		},
		"chainId":   s.evmChainID,
		"networkId": s.evmNetworkID,
	})
}

const nodeDashboardHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>PokoinPoS Node Dashboard</title>
  <style>
    :root {
      color-scheme: dark;
      --bg: #050816;
      --panel: rgba(15, 23, 42, .82);
      --panel-2: rgba(17, 27, 63, .9);
      --line: rgba(255, 255, 255, .1);
      --text: #f8fafc;
      --muted: #b8c4e6;
      --yellow: #facc15;
      --blue: #38bdf8;
      --green: #22c55e;
      --red: #fb7185;
      --orange: #fb923c;
      font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      min-height: 100vh;
      background:
        radial-gradient(circle at top left, rgba(56, 189, 248, .18), transparent 34rem),
        radial-gradient(circle at 80% 0%, rgba(250, 204, 21, .12), transparent 28rem),
        var(--bg);
      color: var(--text);
    }
    a { color: inherit; }
    .wrap { width: min(1180px, calc(100% - 32px)); margin: 0 auto; padding: 28px 0 44px; }
    .hero {
      display: grid;
      grid-template-columns: minmax(0, 1.4fr) minmax(280px, .6fr);
      gap: 18px;
      align-items: stretch;
      margin-bottom: 18px;
    }
    .card {
      border: 1px solid var(--line);
      background: linear-gradient(145deg, var(--panel), rgba(6, 10, 28, .72));
      box-shadow: 0 22px 80px rgba(0, 0, 0, .25);
      border-radius: 28px;
      padding: 22px;
      backdrop-filter: blur(16px);
    }
    .eyebrow {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      padding: 7px 11px;
      border-radius: 999px;
      border: 1px solid rgba(250, 204, 21, .22);
      background: rgba(250, 204, 21, .08);
      color: var(--yellow);
      font-weight: 800;
      font-size: 12px;
      letter-spacing: .08em;
      text-transform: uppercase;
    }
    h1 { margin: 18px 0 10px; font-size: clamp(34px, 6vw, 64px); line-height: .96; letter-spacing: -.05em; }
    h2 { margin: 0 0 14px; font-size: 18px; }
    p { margin: 0; color: var(--muted); line-height: 1.6; }
    .toolbar { display: flex; flex-wrap: wrap; gap: 10px; margin-top: 20px; align-items: center; }
    button, input {
      border: 1px solid var(--line);
      border-radius: 14px;
      background: rgba(255, 255, 255, .06);
      color: var(--text);
      padding: 11px 13px;
      font: inherit;
    }
    button { cursor: pointer; font-weight: 800; }
    button.primary { background: linear-gradient(135deg, var(--yellow), #f97316); color: #111827; border-color: transparent; }
    button:disabled { opacity: .5; cursor: not-allowed; }
    input { width: 100%; outline: none; }
    label { display: grid; gap: 7px; color: var(--muted); font-size: 12px; font-weight: 800; letter-spacing: .04em; text-transform: uppercase; }
    .status-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 18px; }
    .pill { display: inline-flex; align-items: center; gap: 8px; border-radius: 999px; padding: 8px 11px; border: 1px solid var(--line); color: var(--muted); }
    .dot { width: 9px; height: 9px; border-radius: 999px; background: var(--orange); box-shadow: 0 0 20px currentColor; }
    .dot.ok { background: var(--green); color: var(--green); }
    .dot.bad { background: var(--red); color: var(--red); }
    .grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 14px; margin: 18px 0; }
    .metric { min-height: 126px; }
    .metric .label { color: var(--muted); font-size: 12px; font-weight: 800; text-transform: uppercase; letter-spacing: .06em; }
    .metric .value { margin-top: 16px; font-size: 34px; font-weight: 950; letter-spacing: -.04em; }
    .metric .hint { margin-top: 8px; color: var(--muted); font-size: 13px; }
    .cols { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
    .list { display: grid; gap: 10px; }
    .row {
      display: flex;
      justify-content: space-between;
      gap: 14px;
      padding: 12px;
      border: 1px solid var(--line);
      border-radius: 16px;
      background: rgba(255, 255, 255, .04);
      color: var(--muted);
    }
    .row strong { color: var(--text); overflow-wrap: anywhere; }
    .endpoint { align-items: center; }
    .endpoint a { color: var(--blue); font-weight: 800; text-decoration: none; }
    .chart { display: grid; place-items: center; min-height: 220px; }
    .ring {
      width: 168px;
      height: 168px;
      border-radius: 999px;
      background: conic-gradient(var(--blue) 0deg, var(--blue) var(--peer-angle), rgba(250, 204, 21, .9) var(--peer-angle), rgba(250, 204, 21, .9) 360deg);
      position: relative;
      box-shadow: inset 0 0 28px rgba(0, 0, 0, .2), 0 18px 50px rgba(0, 0, 0, .25);
    }
    .ring::after {
      content: "";
      position: absolute;
      inset: 24px;
      border-radius: inherit;
      background: #0b1020;
      border: 1px solid var(--line);
    }
    .forms { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
    .form-grid { display: grid; gap: 12px; }
    pre {
      margin: 0;
      max-height: 220px;
      overflow: auto;
      white-space: pre-wrap;
      color: #dbeafe;
      background: rgba(2, 6, 23, .66);
      border: 1px solid var(--line);
      border-radius: 18px;
      padding: 14px;
    }
    .small { font-size: 12px; color: var(--muted); }
    @media (max-width: 900px) {
      .hero, .cols, .forms { grid-template-columns: 1fr; }
      .grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    }
    @media (max-width: 560px) {
      .wrap { width: min(100% - 20px, 1180px); padding-top: 12px; }
      .card { border-radius: 20px; padding: 16px; }
      .grid { grid-template-columns: 1fr; }
    }
  </style>
</head>
<body>
  <main class="wrap">
    <section class="hero">
      <div class="card">
        <span class="eyebrow">PokoinPoS node host</span>
        <h1>Local node dashboard</h1>
        <p>Monitor this host's PokoinPoS node from the ops API. Keep this port private unless you intentionally expose it behind your own access controls.</p>
        <div class="toolbar">
          <button class="primary" id="refresh">Refresh now</button>
          <a class="pill" href="/metrics" target="_blank" rel="noreferrer">Prometheus metrics</a>
          <a class="pill" href="/explorer/blocks" target="_blank" rel="noreferrer">Explorer blocks</a>
          <a class="pill" href="/endpoints" target="_blank" rel="noreferrer">Endpoint catalog</a>
        </div>
        <div class="status-row">
          <span class="pill"><span id="liveDot" class="dot"></span><span id="liveText">Checking node...</span></span>
          <span class="small" id="lastUpdated">Never refreshed</span>
        </div>
      </div>
      <div class="card">
        <h2>Operator token</h2>
        <p class="small">Optional. Stored only in this browser and required only for guarded operator actions.</p>
        <div class="form-grid" style="margin-top:14px">
          <label>Bearer token
            <input id="adminToken" type="password" autocomplete="off" placeholder="POKOINPOS_OPERATOR_TOKEN">
          </label>
          <div class="toolbar">
            <button id="saveToken">Save token</button>
            <button id="clearToken">Clear</button>
            <button id="checkAdmin">Check admin</button>
          </div>
          <p class="small" id="adminStatus">Admin status not checked.</p>
        </div>
      </div>
    </section>

    <section class="grid" id="metricsGrid"></section>

    <section class="cols">
      <div class="card">
        <h2>Peer topology</h2>
        <div class="chart">
          <div class="ring" id="peerRing" style="--peer-angle: 0deg"></div>
        </div>
        <div class="list">
          <div class="row"><span>Vetting nodes</span><strong id="vettingNodes">0</strong></div>
          <div class="row"><span>Regular peers</span><strong id="regularPeers">0</strong></div>
          <div class="row"><span>Bootstrap peers</span><strong id="bootstrapPeers">0</strong></div>
          <div class="row"><span>Live P2P links</span><strong id="remotePeers">0</strong></div>
          <div class="row"><span>Authorized validators</span><strong id="authorizedValidators">0</strong></div>
        </div>
      </div>
      <div class="card">
        <h2>Node details</h2>
        <div class="list" id="detailsList"></div>
      </div>
    </section>

    <section class="card" style="margin-top:14px">
      <h2>Admin actions</h2>
      <div class="forms">
        <div class="form-grid">
          <label>Mine slot
            <input id="mineSlot" type="number" min="1" value="1">
          </label>
          <button class="primary" id="mineButton">Mine requested slot</button>
        </div>
        <div class="form-grid">
          <label>Mint recipient
            <input id="mintTo" placeholder="0x...">
          </label>
          <label>Amount PKN
            <input id="mintAmount" type="number" min="1" value="1">
          </label>
          <button class="primary" id="mintButton">Mint PKN</button>
        </div>
        <div class="form-grid">
          <label>Payout wallet
            <input id="withdrawTo" placeholder="0x...">
          </label>
          <label>Withdraw amount PKN
            <input id="withdrawAmount" type="number" min="1" value="1">
          </label>
          <button class="primary" id="withdrawButton">Withdraw validator rewards</button>
        </div>
      </div>
      <div style="margin-top:14px">
        <pre id="actionLog">Admin action results will appear here.</pre>
      </div>
    </section>

    <section class="card" style="margin-top:14px">
      <h2>Dynamic validator allowlist</h2>
      <div class="list" id="validatorList"></div>
    </section>

    <section class="card" style="margin-top:14px">
      <h2>Bootstrap registry</h2>
      <p class="small">Shows manifest peers, vetting stage, observed uptime, and fallback discovery health for this node.</p>
      <div class="list" id="bootstrapList" style="margin-top:12px"></div>
    </section>

    <section class="card" style="margin-top:14px">
      <h2>Endpoint status</h2>
      <div class="list" id="endpointList"></div>
    </section>
  </main>

  <script>
    const $ = (id) => document.getElementById(id);
    const state = { health: null, ready: null, chain: null, endpoints: [], validators: [], bootstrap: null };
    const metricDefs = [
      ["Height", "height", "Best chain height"],
      ["Committed", "committedHeight", "Finalized height"],
      ["Peers", "peerCount", "Connected remote peers"],
      ["Mempool", "mempoolDepth", "Pending transactions"],
      ["Validator balance", "validatorStake", "Mining weight, capped at 97%"],
      ["Accepted blocks", "acceptedBlocks", "Blocks accepted by runtime"],
      ["Mined blocks", "minedBlocks", "Blocks mined by this node"],
      ["Transactions", "txCount", "Committed ledger transactions"],
      ["Uptime", "uptimeSeconds", "Runtime uptime"]
    ];

    function token() {
      return $("adminToken").value.trim();
    }

    function formatSeconds(value) {
      const seconds = Number(value || 0);
      const days = Math.floor(seconds / 86400);
      const hours = Math.floor((seconds % 86400) / 3600);
      const minutes = Math.floor((seconds % 3600) / 60);
      if (days > 0) return days + "d " + hours + "h";
      if (hours > 0) return hours + "h " + minutes + "m";
      return minutes + "m";
    }

    async function getJSON(path) {
      const res = await fetch(path, { cache: "no-store" });
      const text = await res.text();
      let body = {};
      try { body = text ? JSON.parse(text) : {}; } catch (_) { body = { raw: text }; }
      if (!res.ok) throw new Error(path + " returned " + res.status);
      return body;
    }

    async function postJSON(path, body) {
      const headers = { "Authorization": "Bearer " + token() };
      if (body) headers["Content-Type"] = "application/json";
      const res = await fetch(path, { method: "POST", headers, body: body ? JSON.stringify(body) : undefined });
      const text = await res.text();
      let parsed = {};
      try { parsed = text ? JSON.parse(text) : {}; } catch (_) { parsed = { raw: text }; }
      if (!res.ok) throw new Error(JSON.stringify(parsed, null, 2));
      return parsed;
    }

    function render() {
      const chain = state.chain || {};
      const ready = state.ready || {};
      const ok = ready.ready === true && chain.height !== undefined;
      $("liveDot").className = "dot " + (ok ? "ok" : "bad");
      $("liveText").textContent = ok ? "Node ready" : "Node not ready";
      $("lastUpdated").textContent = "Updated " + new Date().toLocaleTimeString();

      $("metricsGrid").innerHTML = metricDefs.map(([label, key, hint]) => {
        const raw = chain[key] ?? state.health?.[key] ?? 0;
        const value = key === "uptimeSeconds" ? formatSeconds(raw) : raw;
        return '<article class="card metric"><div class="label">' + label + '</div><div class="value">' + value + '</div><div class="hint">' + hint + '</div></article>';
      }).join("");

      const peerCount = Number(chain.peerCount || 0);
      const candidates = (state.bootstrap?.candidates && state.bootstrap.candidates.length) ? state.bootstrap.candidates : (state.bootstrap?.peers || []);
      const vettingNodes = candidates.filter((p) => (p.status || "peer") === "vetting").length;
      const regularPeers = candidates.filter((p) => ["peer", "candidate"].includes(p.status || "peer")).length;
      const bootstrapPeers = candidates.filter((p) => (p.status || "peer") === "bootstrap").length;
      const observerCount = candidates.reduce((max, p) => Math.max(max, Number(p.externalObservers || 0)), 0);
      const totalRegistryNodes = Math.max(candidates.length, 1);
      const peerAngle = totalRegistryNodes > 0 ? Math.round(((regularPeers + bootstrapPeers) / totalRegistryNodes) * 360) : 0;
      $("peerRing").style.setProperty("--peer-angle", peerAngle + "deg");
      $("vettingNodes").textContent = vettingNodes;
      $("regularPeers").textContent = regularPeers;
      $("bootstrapPeers").textContent = bootstrapPeers;
      $("remotePeers").textContent = peerCount;
      $("authorizedValidators").textContent = state.validators.filter((v) => v.authorized).length;

      const detailRows = [
        ["Currency", chain.currencySymbol || "PKN"],
        ["Readiness", ready.status || "unknown"],
        ["Finality depth", chain.finalityDepth ?? "-"],
        ["Accepted txs", state.health?.acceptedTxs ?? "-"],
        ["RPC URL", location.origin + "/rpc"],
        ["Dashboard URL", location.origin + "/dashboard"],
        ["Bootstrap manifest", state.bootstrap?.manifestUrl || "-"],
        ["External observers", observerCount + " / " + (state.bootstrap?.policy?.minimumExternalObservers || 3)]
      ];
      $("detailsList").innerHTML = detailRows.map(([k, v]) => '<div class="row"><span>' + k + '</span><strong>' + v + '</strong></div>').join("");

      $("endpointList").innerHTML = state.endpoints.map((e) => {
        const href = e.method === "GET" ? e.path : null;
        const label = href ? '<a href="' + href + '" target="_blank" rel="noreferrer">' + e.path + '</a>' : e.path;
        return '<div class="row endpoint"><span><strong>' + e.method + ' ' + label + '</strong><br><span class="small">' + e.summary + '</span></span><span class="small">' + e.authentication + '</span></div>';
      }).join("");

      $("validatorList").innerHTML = state.validators.length ? state.validators.map((v) => {
        const status = v.authorized ? "authorized" : "sync only";
        const locality = v.local ? "local" : (v.connected ? "connected" : "known");
        return '<div class="row"><span><strong>Peer ' + v.peerId + ' · ' + status + '</strong><br><span class="small">' + locality + ' · ' + v.validator + '</span></span><strong>' + v.stake + ' PKN</strong></div>';
      }).join("") : '<div class="row"><span>No validator identities advertised yet.</span><strong>-</strong></div>';

      const bootstrap = state.bootstrap || {};
      const peers = (bootstrap.candidates && bootstrap.candidates.length) ? bootstrap.candidates : (bootstrap.peers || []);
      const policy = bootstrap.policy || {};
      const header = '<div class="row"><span>Policy</span><strong>' + (policy.minimumUptimeRatio ? 'vetting ' + Math.round(policy.vettingMinimumUptimeRatio * 100) + '% / ' + policy.vettingDays + 'd · bootstrap ' + Math.round(policy.minimumUptimeRatio * 100) + '% / ' + policy.bootstrapMaturityDays + 'd · observers ' + (policy.minimumExternalObservers || 3) : '-') + '</strong></div>';
      const error = bootstrap.lastError ? '<div class="row"><span>Last manifest error</span><strong>' + bootstrap.lastError + '</strong></div>' : '';
      const rows = peers.length ? peers.map((p) => {
        const ratio = Math.round(Number(p.uptimeRatio365d || 0) * 10000) / 100;
        const vetting = Math.round(Number(p.vettingUptimeRatio || 0) * 10000) / 100;
        return '<div class="row"><span><strong>' + p.host + ':' + p.port + '</strong><br><span class="small">' + (p.label || p.id || 'bootstrap peer') + ' · ' + (p.status || 'peer') + ' · age ' + (p.ageDays || 0) + 'd · vetting ' + vetting + '% · observers ' + (p.externalObservers || 0) + '</span></span><strong>' + ratio + '% / 365d</strong></div>';
      }).join("") : '<div class="row"><span>No manifest peers loaded yet. Static fallback remains active.</span><strong>-</strong></div>';
      $("bootstrapList").innerHTML = header + error + rows;
    }

    async function refresh() {
      try {
        const [health, ready, chain, endpoints, validators, bootstrap] = await Promise.all([
          getJSON("/health"),
          getJSON("/ready").catch((err) => ({ status: err.message, ready: false })),
          getJSON("/chain/status"),
          getJSON("/endpoints"),
          getJSON("/chain/validators").catch(() => ({ validators: [] })),
          getJSON("/chain/bootstrap").catch(() => ({}))
        ]);
        state.health = health;
        state.ready = ready;
        state.chain = chain;
        state.endpoints = endpoints.endpoints || [];
        state.validators = validators.validators || [];
        state.bootstrap = bootstrap || {};
        render();
      } catch (err) {
        $("liveDot").className = "dot bad";
        $("liveText").textContent = err.message;
      }
    }

    async function checkAdmin() {
      try {
        const res = await fetch("/admin/dashboard/status", { headers: { "Authorization": "Bearer " + token() }, cache: "no-store" });
        const body = await res.json();
        $("adminStatus").textContent = res.ok ? "Admin enabled for chain " + body.chainId : "Admin check failed: " + (body.error || res.status);
      } catch (err) {
        $("adminStatus").textContent = "Admin check failed: " + err.message;
      }
    }

    function logAction(value) {
      $("actionLog").textContent = typeof value === "string" ? value : JSON.stringify(value, null, 2);
    }

    $("refresh").addEventListener("click", refresh);
    $("saveToken").addEventListener("click", () => {
      localStorage.setItem("pokoinpos_admin_token", token());
      $("adminStatus").textContent = "Token saved in this browser.";
    });
    $("clearToken").addEventListener("click", () => {
      localStorage.removeItem("pokoinpos_admin_token");
      $("adminToken").value = "";
      $("adminStatus").textContent = "Token cleared.";
    });
    $("checkAdmin").addEventListener("click", checkAdmin);
    $("mineButton").addEventListener("click", async () => {
      try {
        const slot = Math.max(1, Number($("mineSlot").value || 1));
        logAction(await postJSON("/admin/mine?slot=" + encodeURIComponent(slot)));
        refresh();
      } catch (err) {
        logAction(err.message);
      }
    });
    $("mintButton").addEventListener("click", async () => {
      try {
        const to = $("mintTo").value.trim();
        const amount = Math.max(1, Number($("mintAmount").value || 1));
        logAction(await postJSON("/admin/mint", { to, amount }));
        refresh();
      } catch (err) {
        logAction(err.message);
      }
    });
    $("withdrawButton").addEventListener("click", async () => {
      try {
        const to = $("withdrawTo").value.trim();
        const amount = Math.max(1, Number($("withdrawAmount").value || 1));
        logAction(await postJSON("/admin/withdraw", { to, amount }));
        refresh();
      } catch (err) {
        logAction(err.message);
      }
    });
    $("adminToken").value = localStorage.getItem("pokoinpos_admin_token") || "";
    refresh();
    setInterval(refresh, 10000);
  </script>
</body>
</html>`
