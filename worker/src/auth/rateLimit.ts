interface RateBucket {
  count: number;
  resetAt: number;
}

const buckets = new Map<string, RateBucket>();

const WINDOW_MS = 60_000;
const MAX_ATTEMPTS = 30;

export function checkRateLimit(ip: string): boolean {
  const now = Date.now();
  const bucket = buckets.get(ip);
  if (!bucket || now >= bucket.resetAt) {
    buckets.set(ip, { count: 1, resetAt: now + WINDOW_MS });
    return true;
  }
  if (bucket.count >= MAX_ATTEMPTS) {
    return false;
  }
  bucket.count += 1;
  return true;
}

export function resetRateLimit(ip: string): void {
  buckets.delete(ip);
}
