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

export function encodeHTTPReqPayload(p: HTTPReqPayload): Uint8Array {
  const parts: number[] = [];
  const chunks: Uint8Array[] = [];

  const pushString = (s: string) => {
    const bytes = new TextEncoder().encode(s);
    parts.push((bytes.length >> 8) & 0xff, bytes.length & 0xff);
    chunks.push(bytes);
  };

  pushString(p.password);
  pushString(p.method);
  pushString(p.url);

  parts.push((p.headers.length >> 8) & 0xff, p.headers.length & 0xff);
  for (const [k, v] of p.headers) {
    pushString(k);
    pushString(v);
  }

  const bodyLen = p.body.length;
  parts.push(
    (bodyLen >> 24) & 0xff,
    (bodyLen >> 16) & 0xff,
    (bodyLen >> 8) & 0xff,
    bodyLen & 0xff,
  );

  const header = new Uint8Array(parts);
  const totalLen = header.length + chunks.reduce((n, c) => n + c.length, 0) + bodyLen;
  const out = new Uint8Array(totalLen);
  let off = 0;
  out.set(header, off);
  off += header.length;
  for (const c of chunks) {
    out.set(c, off);
    off += c.length;
  }
  if (bodyLen > 0) {
    out.set(p.body, off);
  }
  return out;
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
  const parts: number[] = [];
  const chunks: Uint8Array[] = [];

  const pushString = (s: string) => {
    const bytes = new TextEncoder().encode(s);
    parts.push((bytes.length >> 8) & 0xff, bytes.length & 0xff);
    chunks.push(bytes);
  };

  parts.push((p.status >> 8) & 0xff, p.status & 0xff);
  parts.push((p.headers.length >> 8) & 0xff, p.headers.length & 0xff);
  for (const [k, v] of p.headers) {
    pushString(k);
    pushString(v);
  }

  const bodyLen = p.body.length;
  parts.push(
    (bodyLen >> 24) & 0xff,
    (bodyLen >> 16) & 0xff,
    (bodyLen >> 8) & 0xff,
    bodyLen & 0xff,
  );

  const header = new Uint8Array(parts);
  const totalLen = header.length + chunks.reduce((n, c) => n + c.length, 0) + bodyLen;
  const out = new Uint8Array(totalLen);
  let off = 0;
  out.set(header, off);
  off += header.length;
  for (const c of chunks) {
    out.set(c, off);
    off += c.length;
  }
  if (bodyLen > 0) {
    out.set(p.body, off);
  }
  return out;
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
