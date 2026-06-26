import { Hono } from "hono";
import { handleWebSocket, type Env } from "./handler/websocket";

/** normalizePath ensures a leading slash and no trailing slash (except "/"). */
export function normalizePath(path: string): string {
  const trimmed = path.trim();
  if (!trimmed || trimmed === "/") {
    return "/";
  }
  const withSlash = trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
  return withSlash.replace(/\/+$/, "") || "/";
}

export function getWebSocketPath(env: Env): string {
  return normalizePath(env.WEBSOCKET_PATH || "/ws");
}

const app = new Hono<{ Bindings: Env }>();

app.use("*", async (c, next) => {
  const wsPath = getWebSocketPath(c.env);
  const pathname = new URL(c.req.url).pathname;
  if (c.req.method === "GET" && pathname === wsPath) {
    return handleWebSocket(c.req.raw, c.env);
  }
  return next();
});

app.get("/", (c) => {
  const wsPath = getWebSocketPath(c.env);
  return c.text(`GS Protocol Worker\nWebSocket path: ${wsPath}`);
});

app.notFound((c) => c.text("Not Found", 404));

export default app;
