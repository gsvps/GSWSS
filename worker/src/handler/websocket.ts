import { connect } from "cloudflare:sockets";
import {
  decodeConnectPayload,
  decodeFrame,
  encodeErrorPayload,
  encodeFrame,
  ErrorCode,
  FrameType,
} from "../protocol/frame";
import {
  decodeFrameAny,
  decodeSessionPayload,
  decodeTargetPayload,
  encodeFrameV2,
  encodeSessionPayload,
  encodeTargetPayload,
} from "../protocol/frame_v2";
import { authFailedMessage, validateTarget } from "../auth/validate";
import { checkRateLimit, resetRateLimit } from "../auth/rateLimit";

export interface Env {
  PASSWORD: string;
  WEBSOCKET_PATH?: string;
}

const BATCH_FLUSH = 16 * 1024;

interface StreamState {
  socket: Socket;
  writer: WritableStreamDefaultWriter<Uint8Array>;
  abort: AbortController;
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
  let mode: "v1" | "v2" | null = null;

  // v1 state
  const v1State = {
    remote: null as Socket | null,
    remoteWriter: null as WritableStreamDefaultWriter<Uint8Array> | null,
    pipeAbort: null as AbortController | null,
  };

  // v2 state
  let v2Authed = false;
  const v2Streams = new Map<number, StreamState>();

  let heartbeatTimer: ReturnType<typeof setInterval> | null = null;

  const cleanup = () => {
    if (heartbeatTimer) {
      clearInterval(heartbeatTimer);
      heartbeatTimer = null;
    }
    v1State.pipeAbort?.abort();
    try {
      v1State.remote?.close();
    } catch {
      // ignore
    }
    for (const st of v2Streams.values()) {
      st.abort.abort();
      try {
        st.socket.close();
      } catch {
        // ignore
      }
    }
    v2Streams.clear();
  };

  ws.addEventListener("close", cleanup);
  ws.addEventListener("error", cleanup);

  ws.addEventListener("message", async (event) => {
    try {
      const data = event.data;
      if (!(data instanceof ArrayBuffer)) {
        return;
      }
      const bytes = new Uint8Array(data);

      if (mode === null) {
        if (bytes.length < 5 || bytes[4] === 1) {
          mode = "v1";
        } else if (bytes[4] === 2) {
          mode = "v2";
        } else {
          sendErrorV1(ws, ErrorCode.INVALID_FRAME, "unsupported protocol version");
          ws.close(1002, "protocol error");
          return;
        }
      }

      if (mode === "v1") {
        await handleV1Message(ws, env, ip, bytes, v1State, (t) => {
          heartbeatTimer = t;
        });
        return;
      }

      const frame = decodeFrameAny(bytes);
      const streamId = frame.streamId ?? 0;

      if (frame.type === FrameType.SESSION) {
        if (streamId !== 0) {
          sendErrorV2(ws, 0, ErrorCode.INVALID_FRAME, "SESSION must use stream 0");
          return;
        }
        let password: string;
        try {
          password = decodeSessionPayload(frame.payload);
        } catch {
          sendErrorV2(ws, 0, ErrorCode.INVALID_FRAME, "invalid SESSION payload");
          return;
        }
        if (password !== env.PASSWORD) {
          sendErrorV2(ws, 0, authFailedMessage().code, authFailedMessage().message);
          ws.close(1008, "auth failed");
          return;
        }
        v2Authed = true;
        resetRateLimit(ip);
        ws.send(
          encodeFrameV2({
            version: 2,
            type: FrameType.SESSION_OK,
            flags: 0,
            streamId: 0,
            payload: new Uint8Array(0),
          }),
        );
        if (!heartbeatTimer) {
          heartbeatTimer = startHeartbeatV2(ws);
        }
        return;
      }

      if (!v2Authed && frame.type !== FrameType.PING) {
        sendErrorV2(ws, streamId, ErrorCode.AUTH_FAILED, "session not authenticated");
        ws.close(1008, "auth required");
        return;
      }

      if (frame.type === FrameType.CONNECT) {
        if (streamId === 0) {
          sendErrorV2(ws, 0, ErrorCode.INVALID_FRAME, "CONNECT requires stream > 0");
          return;
        }
        if (v2Streams.has(streamId)) {
          sendErrorV2(ws, streamId, ErrorCode.INVALID_FRAME, "stream already open");
          return;
        }

        let host: string;
        let port: number;
        try {
          ({ host, port } = decodeTargetPayload(frame.payload));
        } catch {
          sendErrorV2(ws, streamId, ErrorCode.INVALID_FRAME, "invalid CONNECT payload");
          return;
        }

        const targetErr = validateTarget(host, port);
        if (targetErr) {
          sendErrorV2(ws, streamId, ErrorCode.INVALID_TARGET, targetErr);
          return;
        }

        let socket: Socket;
        try {
          socket = connect({ hostname: host, port });
          await socket.opened;
        } catch {
          sendErrorV2(ws, streamId, ErrorCode.CONNECT_FAILED, "failed to connect target");
          return;
        }

        const abort = new AbortController();
        const st: StreamState = {
          socket,
          writer: socket.writable.getWriter(),
          abort,
        };
        v2Streams.set(streamId, st);

        ws.send(
          encodeFrameV2({
            version: 2,
            type: FrameType.DATA,
            flags: 0,
            streamId,
            payload: new Uint8Array(0),
          }),
        );

        pipeRemoteToWSV2(socket, ws, streamId, abort.signal, v2Streams);
        return;
      }

      if (frame.type === FrameType.DATA && streamId > 0) {
        const st = v2Streams.get(streamId);
        if (!st) {
          return;
        }
        await st.writer.write(frame.payload);
        return;
      }

      if (frame.type === FrameType.CLOSE && streamId > 0) {
        closeV2Stream(ws, streamId, v2Streams);
        return;
      }

      if (frame.type === FrameType.PING) {
        ws.send(
          encodeFrameV2({
            version: 2,
            type: FrameType.PONG,
            flags: 0,
            streamId: 0,
            payload: new Uint8Array(0),
          }),
        );
      }
    } catch {
      sendErrorV1(ws, ErrorCode.INVALID_FRAME, "invalid frame");
      ws.close(1002, "protocol error");
    }
  });
}

interface V1State {
  remote: Socket | null;
  remoteWriter: WritableStreamDefaultWriter<Uint8Array> | null;
  pipeAbort: AbortController | null;
}

async function handleV1Message(
  ws: WebSocket,
  env: Env,
  ip: string,
  bytes: Uint8Array,
  st: V1State,
  setHeartbeat: (t: ReturnType<typeof setInterval> | null) => void,
): Promise<void> {
  const frame = decodeFrame(bytes);

  if (!st.remote) {
    if (frame.type !== FrameType.CONNECT) {
      sendErrorV1(ws, ErrorCode.INVALID_FRAME, "expected CONNECT frame");
      ws.close(1002, "protocol error");
      return;
    }

    let payload;
    try {
      payload = decodeConnectPayload(frame.payload);
    } catch {
      sendErrorV1(ws, ErrorCode.INVALID_FRAME, "invalid CONNECT payload");
      ws.close(1002, "protocol error");
      return;
    }

    if (payload.password !== env.PASSWORD) {
      sendErrorV1(ws, authFailedMessage().code, authFailedMessage().message);
      ws.close(1008, "auth failed");
      return;
    }

    const targetErr = validateTarget(payload.host, payload.port);
    if (targetErr) {
      sendErrorV1(ws, ErrorCode.INVALID_TARGET, targetErr);
      ws.close(1008, "invalid target");
      return;
    }

    let socket: Socket;
    try {
      socket = connect({ hostname: payload.host, port: payload.port });
      await socket.opened;
    } catch {
      sendErrorV1(ws, ErrorCode.CONNECT_FAILED, "failed to connect target");
      ws.close(1011, "connect failed");
      return;
    }

    st.remote = socket;
    st.remoteWriter = socket.writable.getWriter();
    resetRateLimit(ip);
    st.pipeAbort = new AbortController();

    ws.send(
      encodeFrame({
        version: 1,
        type: FrameType.DATA,
        flags: 0,
        payload: new Uint8Array(0),
      }),
    );

    pipeRemoteToWSV1(socket, ws, st.pipeAbort.signal);
    setHeartbeat(startHeartbeatV1(ws));
    return;
  }

  if (frame.type === FrameType.DATA && st.remoteWriter) {
    await st.remoteWriter.write(frame.payload);
  } else if (frame.type === FrameType.CLOSE) {
    ws.close(1000, "closed");
    await st.remote?.close();
  } else if (frame.type === FrameType.PING) {
    ws.send(
      encodeFrame({ version: 1, type: FrameType.PONG, flags: 0, payload: new Uint8Array(0) }),
    );
  }
}

async function pipeRemoteToWSV1(remote: Socket, ws: WebSocket, signal: AbortSignal): Promise<void> {
  const reader = remote.readable.getReader();
  try {
    while (!signal.aborted) {
      const { done, value } = await reader.read();
      if (done) {
        ws.send(
          encodeFrame({ version: 1, type: FrameType.CLOSE, flags: 0, payload: new Uint8Array(0) }),
        );
        ws.close(1000, "target closed");
        return;
      }
      ws.send(
        encodeFrame({ version: 1, type: FrameType.DATA, flags: 0, payload: value }),
      );
    }
  } catch {
    if (!signal.aborted) {
      sendErrorV1(ws, ErrorCode.CONNECT_FAILED, "target connection error");
      ws.close(1011, "target error");
    }
  } finally {
    reader.releaseLock();
  }
}

async function pipeRemoteToWSV2(
  remote: Socket,
  ws: WebSocket,
  streamId: number,
  signal: AbortSignal,
  streams: Map<number, StreamState>,
): Promise<void> {
  const reader = remote.readable.getReader();
  let pending: Uint8Array[] = [];
  let pendingLen = 0;

  const flush = () => {
    if (pendingLen === 0) {
      return;
    }
    let payload: Uint8Array;
    if (pending.length === 1) {
      payload = pending[0];
    } else {
      payload = new Uint8Array(pendingLen);
      let off = 0;
      for (const chunk of pending) {
        payload.set(chunk, off);
        off += chunk.length;
      }
    }
    pending = [];
    pendingLen = 0;
    ws.send(
      encodeFrameV2({
        version: 2,
        type: FrameType.DATA,
        flags: 0,
        streamId,
        payload,
      }),
    );
  };

  try {
    while (!signal.aborted) {
      const { done, value } = await reader.read();
      if (done) {
        flush();
        ws.send(
          encodeFrameV2({
            version: 2,
            type: FrameType.CLOSE,
            flags: 0,
            streamId,
            payload: new Uint8Array(0),
          }),
        );
        streams.delete(streamId);
        return;
      }
      pending.push(value);
      pendingLen += value.length;
      if (pendingLen >= BATCH_FLUSH) {
        flush();
      }
    }
  } catch {
    if (!signal.aborted) {
      sendErrorV2(ws, streamId, ErrorCode.CONNECT_FAILED, "target connection error");
    }
  } finally {
    flush();
    reader.releaseLock();
    streams.delete(streamId);
  }
}

function closeV2Stream(ws: WebSocket, streamId: number, streams: Map<number, StreamState>): void {
  const st = streams.get(streamId);
  if (!st) {
    return;
  }
  st.abort.abort();
  try {
    st.socket.close();
  } catch {
    // ignore
  }
  streams.delete(streamId);
}

function startHeartbeatV1(ws: WebSocket): ReturnType<typeof setInterval> {
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

function startHeartbeatV2(ws: WebSocket): ReturnType<typeof setInterval> {
  const timer = setInterval(() => {
    try {
      ws.send(
        encodeFrameV2({
          version: 2,
          type: FrameType.PING,
          flags: 0,
          streamId: 0,
          payload: new Uint8Array(0),
        }),
      );
    } catch {
      clearInterval(timer);
    }
  }, 30_000);
  return timer;
}

function sendErrorV1(ws: WebSocket, code: number, message: string): void {
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

function sendErrorV2(ws: WebSocket, streamId: number, code: number, message: string): void {
  try {
    ws.send(
      encodeFrameV2({
        version: 2,
        type: FrameType.ERROR,
        flags: 0,
        streamId,
        payload: encodeErrorPayload({ code, message }),
      }),
    );
  } catch {
    // ignore
  }
}
