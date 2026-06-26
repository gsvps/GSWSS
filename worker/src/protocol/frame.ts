export const MAGIC = 0x47535031; // "GSP1"
export const VERSION = 1;
export const HEADER_SIZE = 12;
export const MAX_PAYLOAD_SIZE = 16 * 1024 * 1024;

export const FrameType = {
  CONNECT: 1,
  DATA: 2,
  PING: 3,
  PONG: 4,
  CLOSE: 5,
  ERROR: 6,
} as const;

export type FrameTypeValue = (typeof FrameType)[keyof typeof FrameType];

export const ErrorCode = {
  AUTH_FAILED: 1001,
  INVALID_TARGET: 1002,
  CONNECT_FAILED: 1003,
  RATE_LIMITED: 1004,
  INVALID_FRAME: 1005,
  INTERNAL: 1006,
} as const;

export interface Frame {
  version: number;
  type: FrameTypeValue;
  flags: number;
  payload: Uint8Array;
}

export interface ConnectPayload {
  host: string;
  port: number;
  password: string;
}

export interface ErrorPayload {
  code: number;
  message: string;
}

export function encodeConnectPayload(p: ConnectPayload): Uint8Array {
  const hostBytes = new TextEncoder().encode(p.host);
  const passBytes = new TextEncoder().encode(p.password);
  const buf = new Uint8Array(2 + hostBytes.length + 2 + 2 + passBytes.length);
  const view = new DataView(buf.buffer);
  let off = 0;
  off = putString(buf, view, off, hostBytes);
  view.setUint16(off, p.port, false);
  off += 2;
  putString(buf, view, off, passBytes);
  return buf;
}

export function decodeConnectPayload(data: Uint8Array): ConnectPayload {
  const view = new DataView(data.buffer, data.byteOffset, data.byteLength);
  const [host, off1] = getString(data, view, 0);
  if (off1 + 2 > data.length) {
    throw new Error("connect payload missing port");
  }
  const port = view.getUint16(off1, false);
  const [password] = getString(data, view, off1 + 2);
  return { host, port, password };
}

export function encodeErrorPayload(p: ErrorPayload): Uint8Array {
  const msgBytes = new TextEncoder().encode(p.message);
  const buf = new Uint8Array(2 + 2 + msgBytes.length);
  const view = new DataView(buf.buffer);
  view.setUint16(0, p.code, false);
  putString(buf, view, 2, msgBytes);
  return buf;
}

export function decodeErrorPayload(data: Uint8Array): ErrorPayload {
  const view = new DataView(data.buffer, data.byteOffset, data.byteLength);
  if (data.length < 2) {
    throw new Error("error payload too short");
  }
  const code = view.getUint16(0, false);
  const [message] = getString(data, view, 2);
  return { code, message };
}

export function encodeFrame(frame: Frame): Uint8Array {
  if (frame.payload.length > MAX_PAYLOAD_SIZE) {
    throw new Error("payload too large");
  }
  const buf = new Uint8Array(HEADER_SIZE + frame.payload.length);
  const view = new DataView(buf.buffer);
  view.setUint32(0, MAGIC, false);
  buf[4] = frame.version;
  buf[5] = frame.type;
  view.setUint16(6, frame.flags, false);
  view.setUint32(8, frame.payload.length, false);
  buf.set(frame.payload, HEADER_SIZE);
  return buf;
}

export function decodeFrame(data: Uint8Array): Frame {
  if (data.length < HEADER_SIZE) {
    throw new Error("frame too short");
  }
  const view = new DataView(data.buffer, data.byteOffset, data.byteLength);
  const magic = view.getUint32(0, false);
  if (magic !== MAGIC) {
    throw new Error("invalid magic");
  }
  const version = data[4];
  if (version !== VERSION) {
    throw new Error("unsupported version");
  }
  const length = view.getUint32(8, false);
  if (length > MAX_PAYLOAD_SIZE) {
    throw new Error("payload too large");
  }
  if (data.length < HEADER_SIZE + length) {
    throw new Error("incomplete frame");
  }
  return {
    version,
    type: data[5] as FrameTypeValue,
    flags: view.getUint16(6, false),
    payload: data.slice(HEADER_SIZE, HEADER_SIZE + length),
  };
}

function putString(buf: Uint8Array, view: DataView, off: number, s: Uint8Array): number {
  view.setUint16(off, s.length, false);
  buf.set(s, off + 2);
  return off + 2 + s.length;
}

function getString(data: Uint8Array, view: DataView, off: number): [string, number] {
  if (off + 2 > data.length) {
    throw new Error("missing string length");
  }
  const length = view.getUint16(off, false);
  off += 2;
  if (off + length > data.length) {
    throw new Error("string exceeds payload");
  }
  const str = new TextDecoder().decode(data.slice(off, off + length));
  return [str, off + length];
}
