import { validateFetchURL } from "../auth/validate";
import { checkRateLimit } from "../auth/rateLimit";
import { isHTML, isRewritableResource, rewriteHTML, rewriteResource } from "./htmlRewrite";

export interface Env {
  PASSWORD: string;
}

const SKIP_RESP_HEADERS = new Set([
  "connection",
  "keep-alive",
  "proxy-connection",
  "transfer-encoding",
  "te",
  "trailer",
  "upgrade",
  "content-encoding",
  "content-length",
  "content-security-policy",
  "content-security-policy-report-only",
  "x-frame-options",
]);

const BLOCKED_REQ_HEADERS = new Set([
  "host",
  "connection",
  "cookie",
  "content-length",
  "transfer-encoding",
  "cf-connecting-ip",
  "cf-ray",
  "cf-ipcountry",
  "cf-visitor",
  "x-forwarded-for",
  "x-forwarded-proto",
  "x-real-ip",
]);

const COOKIE_NAME = "gs_pwd";

/** handleURLProxy serves /fetch?url=... via Worker fetch() with content rewriting. */
export async function handleURLProxy(request: Request, env: Env): Promise<Response> {
  const ip = request.headers.get("CF-Connecting-IP") ?? "unknown";
  if (!checkRateLimit(ip)) {
    return new Response("Too Many Requests", { status: 429 });
  }

  const reqURL = new URL(request.url);
  const target = reqURL.searchParams.get("url");
  if (!target) {
    return new Response(helpText(), {
      status: 400,
      headers: { "Content-Type": "text/plain; charset=utf-8" },
    });
  }

  const password = resolvePassword(request, reqURL, env);
  if (!password) {
    return new Response("Unauthorized: login at / or add ?password=YOUR_PASSWORD", { status: 401 });
  }

  const targetErr = validateFetchURL(target);
  if (targetErr) {
    return new Response(`Bad Request: ${targetErr}`, { status: 400 });
  }

  const method = request.method.toUpperCase();
  const hasBody = method !== "GET" && method !== "HEAD";

  let upstream: Response;
  try {
    upstream = await fetch(target, {
      method,
      redirect: "follow",
      headers: buildUpstreamHeaders(request, target),
      body: hasBody ? request.body : undefined,
    });
  } catch {
    return new Response("Failed to fetch target URL", { status: 502 });
  }

  const contentType = upstream.headers.get("Content-Type");
  const origin = reqURL.origin;
  let body: BodyInit | null = upstream.body;
  const headers = new Headers();

  if (upstream.body && (isHTML(contentType) || isRewritableResource(contentType))) {
    const text = await upstream.text();
    if (isHTML(contentType)) {
      body = rewriteHTML(text, target, origin);
      headers.set("Content-Type", "text/html; charset=utf-8");
    } else {
      body = rewriteResource(text, target, origin);
      const ct = contentType?.split(";")[0].trim() ?? "text/plain";
      headers.set("Content-Type", contentType?.includes("charset") ? contentType : `${ct}; charset=utf-8`);
    }
  } else {
    upstream.headers.forEach((value, key) => {
      if (!SKIP_RESP_HEADERS.has(key.toLowerCase())) {
        headers.set(key, value);
      }
    });
  }

  headers.set("X-GS-Proxy-Status", String(upstream.status));
  headers.set("X-GS-Proxy-URL", target);
  headers.delete("x-frame-options");
  headers.delete("content-security-policy");

  const fromQuery = reqURL.searchParams.get("password") ?? reqURL.searchParams.get("pwd");
  if (fromQuery && fromQuery === env.PASSWORD) {
    headers.append(
      "Set-Cookie",
      `${COOKIE_NAME}=${encodeURIComponent(fromQuery)}; Path=/; Max-Age=86400; HttpOnly; Secure; SameSite=Lax`,
    );
  }

  return new Response(body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers,
  });
}

function buildUpstreamHeaders(request: Request, target: string): Headers {
  const out = new Headers();
  const defaultUA =
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36";

  out.set("User-Agent", request.headers.get("User-Agent") ?? defaultUA);

  request.headers.forEach((value, key) => {
    const lower = key.toLowerCase();
    if (BLOCKED_REQ_HEADERS.has(lower) || lower.startsWith("cf-")) {
      return;
    }
    if (lower === "referer" || lower === "origin") {
      return;
    }
    out.set(key, value);
  });

  if (!out.has("Accept")) {
    out.set("Accept", request.headers.get("Accept") ?? "*/*");
  }
  if (!out.has("Accept-Language")) {
    out.set("Accept-Language", request.headers.get("Accept-Language") ?? "en-US,en;q=0.9,zh-CN;q=0.8");
  }

  const referer = mapProxyReferer(request.headers.get("Referer"), target);
  if (referer) {
    out.set("Referer", referer);
  }

  const origin = mapProxyOrigin(request.headers.get("Origin"), target);
  if (origin) {
    out.set("Origin", origin);
  }

  return out;
}

function mapProxyReferer(referer: string | null, target: string): string | null {
  const mapped = unwrapProxyURL(referer);
  if (mapped) {
    return mapped;
  }
  if (!referer) {
    try {
      return new URL(target).origin + "/";
    } catch {
      return null;
    }
  }
  return referer;
}

function mapProxyOrigin(origin: string | null, target: string): string | null {
  if (origin) {
    const mapped = unwrapProxyURL(origin);
    if (mapped) {
      try {
        return new URL(mapped).origin;
      } catch {
        return null;
      }
    }
  }
  try {
    return new URL(target).origin;
  } catch {
    return null;
  }
}

function unwrapProxyURL(raw: string | null): string | null {
  if (!raw) {
    return null;
  }
  try {
    const u = new URL(raw);
    if (u.pathname === "/fetch") {
      return u.searchParams.get("url");
    }
  } catch {
    // ignore
  }
  return null;
}

/** handleAuth sets the password cookie and redirects back to browse shell. */
export function handleAuth(request: Request, env: Env): Response {
  const reqURL = new URL(request.url);
  const password = reqURL.searchParams.get("password") ?? reqURL.searchParams.get("pwd");
  const redirect = reqURL.searchParams.get("redirect") ?? "/";

  if (!password || password !== env.PASSWORD) {
    return new Response("Invalid password", { status: 401 });
  }

  const headers = new Headers({
    Location: redirect,
    "Set-Cookie": `${COOKIE_NAME}=${encodeURIComponent(password)}; Path=/; Max-Age=86400; HttpOnly; Secure; SameSite=Lax`,
  });
  return new Response(null, { status: 302, headers });
}

function resolvePassword(request: Request, reqURL: URL, env: Env): string | null {
  const fromQuery = reqURL.searchParams.get("password") ?? reqURL.searchParams.get("pwd");
  if (fromQuery) {
    return fromQuery === env.PASSWORD ? fromQuery : null;
  }
  const fromCookie = parseCookie(request.headers.get("Cookie"), COOKIE_NAME);
  if (fromCookie && fromCookie === env.PASSWORD) {
    return fromCookie;
  }
  const bearer = parseBearer(request.headers.get("Authorization"));
  if (bearer && bearer === env.PASSWORD) {
    return bearer;
  }
  return null;
}

function parseCookie(header: string | null, name: string): string | null {
  if (!header) {
    return null;
  }
  for (const part of header.split(";")) {
    const [k, ...rest] = part.trim().split("=");
    if (k === name) {
      return decodeURIComponent(rest.join("="));
    }
  }
  return null;
}

function parseBearer(auth: string | null): string | null {
  if (!auth) {
    return null;
  }
  const m = auth.match(/^Bearer\s+(.+)$/i);
  return m ? m[1].trim() : null;
}

function helpText(): string {
  return `Missing url parameter. Open / to use the browse frame.`;
}

export function browseHTML(initialURL?: string): string {
  const start = initialURL ? escapeJs(initialURL) : "https://www.google.com";
  return `<!DOCTYPE html>
<html lang="zh">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>GS Browse — URL Proxy</title>
  <style>
    * { box-sizing: border-box; margin: 0; }
    html, body { height: 100%; font-family: system-ui, -apple-system, "Segoe UI", sans-serif; }
    body { display: flex; flex-direction: column; background: #0f172a; color: #e2e8f0; }
    #bar {
      display: flex; gap: 8px; align-items: center; padding: 10px 12px;
      background: #1e293b; border-bottom: 1px solid #334155; flex-shrink: 0;
    }
    #bar input[type=url], #bar input[type=password], #bar input[type=text] {
      padding: 8px 12px; border: 1px solid #475569; border-radius: 8px;
      background: #0f172a; color: #f8fafc; font-size: 14px;
    }
    #addr { flex: 1; min-width: 0; }
    #pwd { width: 120px; }
    button {
      padding: 8px 14px; border: none; border-radius: 8px; cursor: pointer;
      font-size: 13px; font-weight: 500; white-space: nowrap;
    }
    .btn-go { background: #2563eb; color: #fff; }
    .btn-go:hover { background: #1d4ed8; }
    .btn-site { background: #334155; color: #e2e8f0; }
    .btn-site:hover { background: #475569; }
    #frame-wrap { flex: 1; position: relative; background: #fff; }
    #frame { position: absolute; inset: 0; width: 100%; height: 100%; border: none; }
    #overlay {
      position: absolute; inset: 0; display: flex; align-items: center; justify-content: center;
      background: rgba(15,23,42,.92); z-index: 10;
    }
    #overlay.hidden { display: none; }
    .card {
      background: #1e293b; padding: 24px; border-radius: 12px; width: min(400px, 92vw);
      box-shadow: 0 8px 32px rgba(0,0,0,.4);
    }
    .card h2 { margin-bottom: 12px; font-size: 18px; }
    .card p { color: #94a3b8; font-size: 13px; margin-bottom: 16px; line-height: 1.5; }
    .card label { display: block; font-size: 12px; color: #94a3b8; margin-bottom: 4px; }
    .card input { width: 100%; margin-bottom: 12px; }
    .hint { font-size: 12px; color: #64748b; margin-top: 12px; }
  </style>
</head>
<body>
  <div id="bar">
    <button type="button" class="btn-site" data-url="https://www.google.com">Google</button>
    <button type="button" class="btn-site" data-url="https://www.youtube.com">YouTube</button>
    <button type="button" class="btn-site" data-url="https://www.cloudflare.com">Cloudflare</button>
    <input id="addr" type="url" placeholder="https://..." value="https://www.google.com"/>
    <button type="button" class="btn-go" id="btnGo">前往</button>
  </div>
  <div id="frame-wrap">
    <div id="overlay">
      <div class="card">
        <h2>GS Browse</h2>
        <p>在下方框架内通过 Worker fetch 浏览网页，链接、JS/CSS/API 请求均会自动走代理。</p>
        <label>密码</label>
        <input id="pwd" type="password" placeholder="change-me" autocomplete="current-password"/>
        <button type="button" class="btn-go" id="btnLogin" style="width:100%">登录并打开</button>
        <p class="hint">与 config.yaml / wrangler PASSWORD 一致</p>
      </div>
    </div>
    <iframe id="frame" title="GS Browse Frame" sandbox="allow-scripts allow-forms allow-same-origin allow-popups allow-downloads allow-modals"></iframe>
  </div>
  <script>
    var DEFAULT_URL = "${start}";
    var frame = document.getElementById("frame");
    var addr = document.getElementById("addr");
    var overlay = document.getElementById("overlay");

    function fetchSrc(url) {
      return "/fetch?url=" + encodeURIComponent(url);
    }

    function navigate(url) {
      if (!url) return;
      if (!/^https?:\\/\\//i.test(url)) url = "https://" + url;
      addr.value = url;
      frame.src = fetchSrc(url);
    }

    function loginAndOpen() {
      var pwd = document.getElementById("pwd").value;
      if (!pwd) { alert("请输入密码"); return; }
      var u = addr.value || DEFAULT_URL;
      window.location.href = "/auth?password=" + encodeURIComponent(pwd) +
        "&redirect=" + encodeURIComponent("/?url=" + encodeURIComponent(u) + "&authed=1");
    }

    document.getElementById("btnGo").onclick = function() { navigate(addr.value); };
    document.getElementById("btnLogin").onclick = loginAndOpen;
    document.querySelectorAll(".btn-site").forEach(function(btn) {
      btn.onclick = function() {
        addr.value = btn.getAttribute("data-url");
        navigate(addr.value);
      };
    });
    addr.addEventListener("keydown", function(e) {
      if (e.key === "Enter") navigate(addr.value);
    });

    (function init() {
      var params = new URLSearchParams(location.search);
      var start = params.get("url") || DEFAULT_URL;
      addr.value = start;
      if (params.has("authed")) {
        overlay.classList.add("hidden");
        navigate(start);
      }
    })();
  </script>
</body>
</html>`;
}

function escapeJs(s: string): string {
  return s.replace(/\\/g, "\\\\").replace(/"/g, '\\"').replace(/'/g, "\\'");
}
