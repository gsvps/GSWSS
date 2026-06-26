import { connect } from "cloudflare:sockets";
import {
  decodeConnectPayload,
  decodeFrame,
  encodeErrorPayload,
  encodeFrame,
  ErrorCode,
  FrameType,
} from "../protocol/frame";
import { authFailedMessage, validateTarget } from "../auth/validate";
import { checkRateLimit, resetRateLimit } from "../auth/rateLimit";

export interface Env {
  PASSWORD: string;
  WEBSOCKET_PATH?: string;
}

export async function handleWebSocket(request: Request, env: Env): Promise<Response> {
  const upgradeHeader = request.headers.get("Upgrade");
  if (upgradeHeader !== "websocket") {
    return new Response("Expected WebSocket", { status: 426 });
  }

  const ip = request.headers.get("CF-Connecting-IP") ?? "unknown";
  if (!checkRateLimit(ip)) {
    return new Response("Too Many Requests", { status: 429 });
  }

  const pair = new WebSocketPair();
  const [client, server] = Object.values(pair);

  server.accept();
  handleSession(server, env, ip).catch(() => {
    try {
      server.close(1011, "internal error");
    } catch {
      // already closed
    }
  });

  return new Response(null, { status: 101, webSocket: client });
}

async function handleSession(ws: WebSocket, env: Env, ip: string): Promise<void> {
  let remote: Socket | null = null;
  let remoteWriter: WritableStreamDefaultWriter<Uint8Array> | null = null;
  let heartbeatTimer: ReturnType<typeof setInterval> | null = null;
  let pipeAbort: AbortController | null = null;

  const cleanup = () => {
    if (heartbeatTimer) {
      clearInterval(heartbeatTimer);
      heartbeatTimer = null;
    }
    pipeAbort?.abort();
    try {
      remote?.close();
    } catch {
      // ignore
    }
  };

  ws.addEventListener("close", cleanup);
  ws.addEventListener("error", cleanup);

  ws.addEventListener("message", async (event) => {
    try {
      const data = event.data;
      if (!(data instanceof ArrayBuffer)) {
        return;
      }
      const frame = decodeFrame(new Uint8Array(data));

      if (!remote) {
        if (frame.type !== FrameType.CONNECT) {
          sendError(ws, ErrorCode.INVALID_FRAME, "expected CONNECT frame");
          ws.close(1002, "protocol error");
          return;
        }

        let payload;
        try {
          payload = decodeConnectPayload(frame.payload);
        } catch {
          sendError(ws, ErrorCode.INVALID_FRAME, "invalid CONNECT payload");
          ws.close(1002, "protocol error");
          return;
        }

        if (payload.password !== env.PASSWORD) {
          sendError(ws, authFailedMessage().code, authFailedMessage().message);
          ws.close(1008, "auth failed");
          return;
        }

        const targetErr = validateTarget(payload.host, payload.port);
        if (targetErr) {
          sendError(ws, ErrorCode.INVALID_TARGET, targetErr);
          ws.close(1008, "invalid target");
          return;
        }

        let socket: Socket;
        try {
          socket = connect({
            hostname: payload.host,
            port: payload.port,
          });
          await socket.opened;
        } catch {
          sendError(ws, ErrorCode.CONNECT_FAILED, "failed to connect target");
          ws.close(1011, "connect failed");
          return;
        }

        remote = socket;
        remoteWriter = socket.writable.getWriter();
        resetRateLimit(ip);
        pipeAbort = new AbortController();

        // CONNECT ack — client treats empty DATA as success
        ws.send(
          encodeFrame({
            version: 1,
            type: FrameType.DATA,
            flags: 0,
            payload: new Uint8Array(0),
          }),
        );

        pipeRemoteToWS(socket, ws, pipeAbort.signal);
        heartbeatTimer = startHeartbeat(ws);
        return;
      }

      if (frame.type === FrameType.DATA && remoteWriter) {
        await remoteWriter.write(frame.payload);
      } else if (frame.type === FrameType.CLOSE) {
        ws.close(1000, "closed");
        await remote?.close();
      } else if (frame.type === FrameType.PING) {
        ws.send(
          encodeFrame({ version: 1, type: FrameType.PONG, flags: 0, payload: new Uint8Array(0) }),
        );
      }
    } catch {
      sendError(ws, ErrorCode.INVALID_FRAME, "invalid frame");
      ws.close(1002, "protocol error");
    }
  });
}

async function pipeRemoteToWS(remote: Socket, ws: WebSocket, signal: AbortSignal): Promise<void> {
  const reader = remote.readable.getReader();
  try {
    while (!signal.aborted) {
      const { done, value } = await reader.read();
      if (done) {
        ws.send(
          encodeFrame({
            version: 1,
            type: FrameType.CLOSE,
            flags: 0,
            payload: new Uint8Array(0),
          }),
        );
        ws.close(1000, "target closed");
        return;
      }
      ws.send(
        encodeFrame({
          version: 1,
          type: FrameType.DATA,
          flags: 0,
          payload: value,
        }),
      );
    }
  } catch {
    if (!signal.aborted) {
      sendError(ws, ErrorCode.CONNECT_FAILED, "target connection error");
      ws.close(1011, "target error");
    }
  } finally {
    reader.releaseLock();
  }
}

function startHeartbeat(ws: WebSocket): ReturnType<typeof setInterval> {
  const timer = setInterval(() => {
    try {
      ws.send(
        encodeFrame({
          version: 1,
          type: FrameType.PING,
          flags: 0,
          payload: new Uint8Array(0),
        }),
      );
    } catch {
      clearInterval(timer);
    }
  }, 30_000);
  return timer;
}

function sendError(ws: WebSocket, code: number, message: string): void {
  try {
    ws.send(
      encodeFrame({
        version: 1,
        type: FrameType.ERROR,
        flags: 0,
        payload: encodeErrorPayload({ code, message }),
      }),
    );
  } catch {
    // ignore
  }
}
