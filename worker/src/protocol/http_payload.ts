export interface HTTPReqPayload {
  password: string;
  method: string;
  url: string;
  headers: [string, string][];
  body: Uint8Array;
}

export interface HTTPRespPayload {
  status: number;
  headers: [string, string][];
  body: Uint8Array;
}

function pushU16(out: number[], n: number): void {
  out.push((n >> 8) & 0xff, n & 0xff);
}

function pushU32(out: number[], n: number): void {
  out.push((n >> 24) & 0xff, (n >> 16) & 0xff, (n >> 8) & 0xff, n & 0xff);
}

function pushString(out: number[], s: string): void {
  const bytes = new TextEncoder().encode(s);
  pushU16(out, bytes.length);
  for (const b of bytes) {
    out.push(b);
  }
}

export function encodeHTTPReqPayload(p: HTTPReqPayload): Uint8Array {
  const out: number[] = [];
  pushString(out, p.password);
  pushString(out, p.method);
  pushString(out, p.url);
  pushU16(out, p.headers.length);
  for (const [k, v] of p.headers) {
    pushString(out, k);
    pushString(out, v);
  }
  pushU32(out, p.body.length);
  for (const b of p.body) {
    out.push(b);
  }
  return new Uint8Array(out);
}

export function decodeHTTPReqPayload(data: Uint8Array): HTTPReqPayload {
  const view = new DataView(data.buffer, data.byteOffset, data.byteLength);
  let off = 0;

  const readString = (): string => {
    if (off + 2 > data.length) {
      throw new Error("missing string length");
    }
    const len = view.getUint16(off, false);
    off += 2;
    if (off + len > data.length) {
      throw new Error("string exceeds payload");
    }
    const str = new TextDecoder().decode(data.slice(off, off + len));
    off += len;
    return str;
  };

  const password = readString();
  const method = readString();
  const url = readString();

  if (off + 2 > data.length) {
    throw new Error("missing header count");
  }
  const headerCount = view.getUint16(off, false);
  off += 2;

  const headers: [string, string][] = [];
  for (let i = 0; i < headerCount; i++) {
    headers.push([readString(), readString()]);
  }

  if (off + 4 > data.length) {
    throw new Error("missing body length");
  }
  const bodyLen = view.getUint32(off, false);
  off += 4;
  if (off + bodyLen > data.length) {
    throw new Error("invalid body length");
  }
  const body = data.slice(off, off + bodyLen);
  return { password, method, url, headers, body };
}

export function encodeHTTPRespPayload(p: HTTPRespPayload): Uint8Array {
  const out: number[] = [];
  pushU16(out, p.status);
  pushU16(out, p.headers.length);
  for (const [k, v] of p.headers) {
    pushString(out, k);
    pushString(out, v);
  }
  pushU32(out, p.body.length);
  for (const b of p.body) {
    out.push(b);
  }
  return new Uint8Array(out);
}

export function decodeHTTPRespPayload(data: Uint8Array): HTTPRespPayload {
  const view = new DataView(data.buffer, data.byteOffset, data.byteLength);
  if (data.length < 4) {
    throw new Error("http resp payload too short");
  }
  const status = view.getUint16(0, false);
  const headerCount = view.getUint16(2, false);
  let off = 4;

  const readString = (): string => {
    if (off + 2 > data.length) {
      throw new Error("missing string length");
    }
    const len = view.getUint16(off, false);
    off += 2;
    if (off + len > data.length) {
      throw new Error("string exceeds payload");
    }
    const str = new TextDecoder().decode(data.slice(off, off + len));
    off += len;
    return str;
  };

  const headers: [string, string][] = [];
  for (let i = 0; i < headerCount; i++) {
    headers.push([readString(), readString()]);
  }

  if (off + 4 > data.length) {
    throw new Error("missing body length");
  }
  const bodyLen = view.getUint32(off, false);
  off += 4;
  if (off + bodyLen > data.length) {
    throw new Error("invalid body length");
  }
  const body = data.slice(off, off + bodyLen);
  return { status, headers, body };
}
