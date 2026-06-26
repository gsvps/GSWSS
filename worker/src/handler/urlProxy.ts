import { validateFetchURL } from "../auth/validate";
import { checkRateLimit } from "../auth/rateLimit";

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
]);

/** handleURLProxy serves GET /fetch?url=...&password=... via Worker fetch(). */
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

  const password =
    reqURL.searchParams.get("password") ??
    reqURL.searchParams.get("pwd") ??
    parseBearer(request.headers.get("Authorization"));
  if (!password || password !== env.PASSWORD) {
    return new Response("Unauthorized: add ?password=YOUR_PASSWORD", { status: 401 });
  }

  const targetErr = validateFetchURL(target);
  if (targetErr) {
    return new Response(`Bad Request: ${targetErr}`, { status: 400 });
  }

  let upstream: Response;
  try {
    upstream = await fetch(target, {
      method: "GET",
      redirect: "follow",
      headers: {
        "User-Agent": request.headers.get("User-Agent") ?? "GS-URL-Proxy/1.0",
        Accept: request.headers.get("Accept") ?? "*/*",
      },
    });
  } catch {
    return new Response("Failed to fetch target URL", { status: 502 });
  }

  const headers = new Headers();
  upstream.headers.forEach((value, key) => {
    if (!SKIP_RESP_HEADERS.has(key.toLowerCase())) {
      headers.set(key, value);
    }
  });
  headers.set("X-GS-Proxy-Status", String(upstream.status));
  headers.set("X-GS-Proxy-URL", target);

  return new Response(upstream.body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers,
  });
}

function parseBearer(auth: string | null): string | null {
  if (!auth) {
    return null;
  }
  const m = auth.match(/^Bearer\s+(.+)$/i);
  return m ? m[1].trim() : null;
}

function helpText(): string {
  const host = "https://test.gsvps.com";
  return `GS URL Proxy — simple HTTP fetch via Worker

Usage:
  ${host}/fetch?url=<TARGET_URL>&password=<PASSWORD>

Examples:
  ${host}/fetch?url=https://www.google.com&password=YOUR_PASSWORD
  ${host}/fetch?url=https://whoer.net&password=YOUR_PASSWORD
  ${host}/fetch?url=https://www.cloudflare.com&password=YOUR_PASSWORD

Also works on root:
  ${host}/?url=https://example.com&password=YOUR_PASSWORD

WebSocket (GS client): ${host}/ws
Password: set in wrangler.toml / GitHub Secrets (not shown here).
`;
}

export function helpHTML(): string {
  return `<!DOCTYPE html>
<html lang="zh">
<head>
  <meta charset="utf-8"/>
  <title>GS URL Proxy</title>
  <style>
    body { font-family: system-ui, sans-serif; max-width: 720px; margin: 2rem auto; padding: 0 1rem; }
    code { background: #f4f4f5; padding: 2px 6px; border-radius: 4px; }
    input { width: 100%; padding: 8px; margin: 4px 0 12px; box-sizing: border-box; }
    button { padding: 8px 16px; cursor: pointer; }
  </style>
</head>
<body>
  <h1>GS URL Proxy</h1>
  <p>在浏览器里直接测试 Worker <code>fetch()</code> 转发，无需本地客户端。</p>
  <form id="f">
    <label>目标 URL</label>
    <input id="url" type="url" placeholder="https://whoer.net" required/>
    <label>密码</label>
    <input id="pwd" type="password" placeholder="change-me" required/>
    <button type="submit">打开</button>
  </form>
  <p style="color:#666;font-size:14px">等价于：<code>/fetch?url=...&amp;password=...</code></p>
  <script>
    document.getElementById("f").onsubmit = function(e) {
      e.preventDefault();
      var u = document.getElementById("url").value;
      var p = document.getElementById("pwd").value;
      location.href = "/fetch?url=" + encodeURIComponent(u) + "&password=" + encodeURIComponent(p);
    };
  </script>
</body>
</html>`;
}
