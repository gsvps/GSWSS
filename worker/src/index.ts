import { Hono } from "hono";
import { handleWebSocket, type Env } from "./handler/websocket";

const app = new Hono<{ Bindings: Env }>();

app.get("/", (c) => {
  return c.text("GS Protocol Worker");
});

app.get("/ws", async (c) => {
  return handleWebSocket(c.req.raw, c.env);
});

export default app;
