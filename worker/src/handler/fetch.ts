import {
  decodeHTTPReqPayload,
  encodeHTTPRespPayload,
} from "../protocol/http_payload";
import { encodeErrorPayload, encodeFrame, ErrorCode, FrameType } from "../protocol/frame";
import { authFailedMessage, validateFetchURL } from "../auth/validate";

const SKIP_REQ_HEADERS = new Set([
  "host",
  "connection",
  "proxy-connection",
  "proxy-authorization",
  "keep-alive",
  "transfer-encoding",
  "te",
  "trailer",
  "upgrade",
]);

const SKIP_RESP_HEADERS = new Set([
  "connection",
  "keep-alive",
  "proxy-connection",
  "transfer-encoding",
  "te",
  "trailer",
  "upgrade",
]);

export async function handleHTTPFetch(
  ws: WebSocket,
  env: { PASSWORD: string },
  payload: Uint8Array,
): Promise<void> {
  let req;
  try {
    req = decodeHTTPReqPayload(payload);
  } catch {
    sendHTTPError(ws, ErrorCode.INVALID_FRAME, "invalid HTTP_REQ payload");
    ws.close(1002, "protocol error");
    return;
  }

  if (req.password !== env.PASSWORD) {
    sendHTTPError(ws, authFailedMessage().code, authFailedMessage().message);
    ws.close(1008, "auth failed");
    return;
  }

  const targetErr = validateFetchURL(req.url);
  if (targetErr) {
    sendHTTPError(ws, ErrorCode.INVALID_TARGET, targetErr);
    ws.close(1008, "invalid target");
    return;
  }

  const headers = new Headers();
  for (const [key, value] of req.headers) {
    const lower = key.toLowerCase();
    if (SKIP_REQ_HEADERS.has(lower)) {
      continue;
    }
    headers.append(key, value);
  }

  let resp: Response;
  try {
    resp = await fetch(req.url, {
      method: req.method,
      headers,
      body: req.body.length > 0 ? req.body : undefined,
      redirect: "follow",
    });
  } catch {
    sendHTTPError(ws, ErrorCode.CONNECT_FAILED, "fetch failed");
    ws.close(1011, "fetch failed");
    return;
  }

  const body = new Uint8Array(await resp.arrayBuffer());
  const outHeaders: [string, string][] = [];
  resp.headers.forEach((value, key) => {
    if (SKIP_RESP_HEADERS.has(key.toLowerCase())) {
      return;
    }
    outHeaders.push([key, value]);
  });

  ws.send(
    encodeFrame({
      version: 1,
      type: FrameType.HTTP_RESP,
      flags: 0,
      payload: encodeHTTPRespPayload({
        status: resp.status,
        headers: outHeaders,
        body,
      }),
    }),
  );
  ws.close(1000, "done");
}

function sendHTTPError(ws: WebSocket, code: number, message: string): void {
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
