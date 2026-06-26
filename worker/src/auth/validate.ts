import { ErrorCode } from "../protocol/frame";

const PRIVATE_RANGES = [
  /^127\./,
  /^10\./,
  /^192\.168\./,
  /^172\.(1[6-9]|2\d|3[01])\./,
  /^0\./,
  /^localhost$/i,
  /^::1$/,
  /^fc00:/i,
  /^fe80:/i,
];

const BLOCKED_PORTS = new Set([22, 23, 25, 53, 135, 139, 445, 3389, 5985, 5986]);

export function validateTarget(host: string, port: number): string | null {
  const trimmed = host.trim();
  if (!trimmed || trimmed.length > 253) {
    return "invalid host length";
  }
  if (port === 0 || port > 65535) {
    return "invalid port";
  }
  if (BLOCKED_PORTS.has(port)) {
    return "port not allowed";
  }
  for (const re of PRIVATE_RANGES) {
    if (re.test(trimmed)) {
      return "private address not allowed";
    }
  }
  if (/[^\x21-\x7e]/.test(trimmed) && !/^[a-zA-Z0-9.-]+$/.test(trimmed)) {
    return "invalid host characters";
  }
  return null;
}

export function authFailedMessage(): { code: number; message: string } {
  return { code: ErrorCode.AUTH_FAILED, message: "authentication failed" };
}
