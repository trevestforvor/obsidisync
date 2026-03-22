package main

import (
	"net/http"
)

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Obsidisync</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #1a1a2e; color: #e0e0e0; min-height: 100vh; display: flex; align-items: center; justify-content: center; }
  .card { background: #16213e; border-radius: 12px; padding: 2rem; max-width: 480px; width: 90%; box-shadow: 0 4px 24px rgba(0,0,0,0.3); }
  h1 { font-size: 1.5rem; margin-bottom: 1rem; color: #a78bfa; }
  .status { padding: 1rem; border-radius: 8px; margin-bottom: 1rem; font-family: monospace; font-size: 0.875rem; white-space: pre-wrap; word-break: break-all; }
  .ok { background: #064e3b; border: 1px solid #059669; }
  .error { background: #7f1d1d; border: 1px solid #dc2626; }
  .loading { background: #1e293b; border: 1px solid #475569; }
  .dot { display: inline-block; width: 10px; height: 10px; border-radius: 50%; margin-right: 8px; vertical-align: middle; }
  .dot.green { background: #10b981; }
  .dot.red { background: #ef4444; }
  .dot.grey { background: #6b7280; }
  .label { font-size: 0.75rem; color: #9ca3af; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 0.25rem; }
  .refresh { color: #818cf8; cursor: pointer; font-size: 0.875rem; text-decoration: underline; border: none; background: none; }
  .token-section { margin-top: 1.5rem; border-top: 1px solid #2d3748; padding-top: 1rem; }
  .reveal-btn { background: #4c1d95; color: #e0e0e0; border: none; border-radius: 6px; padding: 0.5rem 1rem; cursor: pointer; font-size: 0.875rem; }
  .reveal-btn:hover { background: #5b21b6; }
  .token-display { margin-top: 0.5rem; padding: 0.75rem; background: #1e293b; border: 1px solid #475569; border-radius: 6px; font-family: monospace; font-size: 0.8rem; word-break: break-all; display: none; }
  .copy-btn { color: #818cf8; cursor: pointer; font-size: 0.75rem; text-decoration: underline; border: none; background: none; margin-top: 0.25rem; }
</style>
</head>
<body>
<div class="card">
  <h1>Obsidisync</h1>
  <div class="label">Health Status</div>
  <div id="status" class="status loading"><span class="dot grey"></span>Checking...</div>
  <button class="refresh" onclick="check()">Refresh</button>
  <div class="token-section">
    <div class="label">Vault Token</div>
    <button class="reveal-btn" id="revealBtn" onclick="revealToken()">Reveal Token</button>
    <div class="token-display" id="tokenDisplay"></div>
    <button class="copy-btn" id="copyBtn" style="display:none" onclick="copyToken()">Copy to clipboard</button>
  </div>
</div>
<script>
async function check() {
  const el = document.getElementById('status');
  el.className = 'status loading';
  el.innerHTML = '<span class="dot grey"></span>Checking...';
  try {
    const res = await fetch('/api/health');
    const data = await res.json();
    const ok = data.status === 'ok';
    el.className = 'status ' + (ok ? 'ok' : 'error');
    el.innerHTML = '<span class="dot ' + (ok ? 'green' : 'red') + '"></span>' + JSON.stringify(data, null, 2);
  } catch (e) {
    el.className = 'status error';
    el.innerHTML = '<span class="dot red"></span>Failed to reach /api/health\n' + e.message;
  }
}
check();
let _token = '';
async function revealToken() {
  try {
    const res = await fetch('/api/token');
    if (res.status === 401 || res.status === 302 || res.redirected) {
      window.location.href = '/api/token';
      return;
    }
    const data = await res.json();
    _token = data.token;
    document.getElementById('tokenDisplay').textContent = _token;
    document.getElementById('tokenDisplay').style.display = 'block';
    document.getElementById('copyBtn').style.display = 'inline';
    document.getElementById('revealBtn').style.display = 'none';
  } catch (e) {
    window.location.href = '/api/token';
  }
}
function copyToken() {
  navigator.clipboard.writeText(_token).then(() => {
    document.getElementById('copyBtn').textContent = 'Copied!';
    setTimeout(() => { document.getElementById('copyBtn').textContent = 'Copy to clipboard'; }, 2000);
  });
}
</script>
</body>
</html>`

func NewIndexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(indexHTML))
	}
}
