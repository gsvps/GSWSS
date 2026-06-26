import {
  MAGIC,
  MAX_PAYLOAD_SIZE,
  VERSION,
  decodeFrame,
  type Frame,
  type FrameTypeValue,
} from "./frame";

export const VERSION2 = 2;
export const HEADER_SIZE_V2 = 16;

export const FrameTypeV2 = {
  SESSION: 7,
  SESSION_OK: 8,
} as const;

export function encodeTargetPayload(host: string, port: number): Uint8Array {
  const hostBytes = new TextEncoder().encode(host);
  const buf = new Uint8Array(2 + hostBytes.length + 2);
  const view = new DataView(buf.buffer);
  let off = putString(buf, view, 0, hostBytes);
  view.setUint16(off, port, false);
  return buf;
}

export function decodeTargetPayload(data: Uint8Array): { host: string; port: number } {
  const view = new DataView(data.buffer, data.byteOffset, data.byteLength);
  const [host, off1] = getString(data, view, 0);
  if (off1 + 2 > data.length) {
    throw new Error("target payload missing port");
  }
  const port = view.getUint16(off1, false);
  return { host, port };
}

export function encodeSessionPayload(password: string): Uint8Array {
  const passBytes = new TextEncoder().encode(password);
  const buf = new Uint8Array(2 + passBytes.length);
  putString(buf, new DataView(buf.buffer), 0, passBytes);
  return buf;
}

export function decodeSessionPayload(data: Uint8Array): string {
  const view = new DataView(data.buffer, data.byteOffset, data.byteLength);
  const [password] = getString(data, view, 0);
  return password;
}

export function encodeFrameV2(frame: Frame & { streamId: number }): Uint8Array {
  if (frame.payload.length > MAX_PAYLOAD_SIZE) {
    throw new Error("payload too large");
  }
  const buf = new Uint8Array(HEADER_SIZE_V2 + frame.payload.length);
  const view = new DataView(buf.buffer);
  view.setUint32(0, MAGIC, false);
  buf[4] = VERSION2;
  buf[5] = frame.type;
  view.setUint32(6, frame.streamId, false);
  view.setUint32(10, frame.payload.length, false);
  buf.set(frame.payload, HEADER_SIZE_V2);
  return buf;
}

export function decodeFrameV2(data: Uint8Array): Frame & { streamId: number } {
  if (data.length < HEADER_SIZE_V2) {
    throw new Error("frame too short");
  }
  const view = new DataView(data.buffer, data.byteOffset, data.byteLength);
  const length = view.getUint32(10, false);
  if (length > MAX_PAYLOAD_SIZE) {
    throw new Error("payload too large");
  }
  if (data.length < HEADER_SIZE_V2 + length) {
    throw new Error("incomplete frame");
  }
  return {
    version: VERSION2,
    type: data[5] as FrameTypeValue,
    flags: 0,
    streamId: view.getUint32(6, false),
    payload: data.slice(HEADER_SIZE_V2, HEADER_SIZE_V2 + length),
  };
}

export function decodeFrameAny(data: Uint8Array): Frame & { streamId: number } {
  if (data.length < 5) {
    throw new Error("frame too short");
  }
  const view = new DataView(data.buffer, data.byteOffset, data.byteLength);
  if (view.getUint32(0, false) !== MAGIC) {
    throw new Error("invalid magic");
  }
  const version = data[4];
  if (version === VERSION) {
    const frame = decodeFrame(data);
    return { ...frame, streamId: 1 };
  }
  if (version === VERSION2) {
    return decodeFrameV2(data);
  }
  throw new Error("unsupported version");
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
