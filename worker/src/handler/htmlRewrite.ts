import { buildInjectScript } from "./proxyInject";

const PROXY_ATTRS = [
  "href",
  "src",
  "action",
  "poster",
  "data-src",
  "formaction",
  "content",
  "data",
  "icon",
  "xlink:href",
];

const REWRITABLE_TYPES = new Set([
  "text/html",
  "application/xhtml+xml",
  "text/css",
  "text/javascript",
  "application/javascript",
  "application/x-javascript",
  "application/json",
  "application/ld+json",
  "text/json",
  "application/manifest+json",
]);

export function absProxy(raw: string, base: URL, origin: string): string {
  const trimmed = raw.trim();
  if (
    !trimmed ||
    trimmed.startsWith("#") ||
    trimmed.startsWith("javascript:") ||
    trimmed.startsWith("data:") ||
    trimmed.startsWith("mailto:") ||
    trimmed.startsWith("blob:")
  ) {
    return raw;
  }
  try {
    const abs = new URL(trimmed, base).href;
    if (abs.startsWith(origin + "/fetch")) {
      return abs;
    }
    if (abs.startsWith("http://") || abs.startsWith("https://")) {
      return `${origin}/fetch?url=${encodeURIComponent(abs)}`;
    }
  } catch {
    // keep original
  }
  return raw;
}

/** rewriteHTML rewrites resource and navigation URLs to go through /fetch. */
export function rewriteHTML(html: string, pageURL: string, origin: string): string {
  const base = new URL(pageURL);
  const proxyURL = (raw: string): string => absProxy(raw, base, origin);

  let out = html;
  for (const attr of PROXY_ATTRS) {
    const re = new RegExp(`(\\s${attr.replace(":", "\\:")}\\s*=\\s*)(["'])([^"']*)\\2`, "gi");
    out = out.replace(re, (_m, prefix: string, quote: string, val: string) => {
      return `${prefix}${quote}${proxyURL(val)}${quote}`;
    });
  }

  out = out.replace(/\ssrcset\s*=\s*(["'])([^"']*)\1/gi, (_m, quote: string, val: string) => {
    const parts = val.split(",").map((part) => {
      const bits = part.trim().split(/\s+/);
      if (bits[0]) {
        bits[0] = proxyURL(bits[0]);
      }
      return bits.join(" ");
    });
    return ` srcset=${quote}${parts.join(", ")}${quote}`;
  });

  // CSS url(...) inside inline style attributes
  out = out.replace(/\burl\s*\(\s*(["']?)([^"')]+)\1\s*\)/gi, (_m, quote: string, val: string) => {
    return `url(${quote}${proxyURL(val.trim())}${quote})`;
  });

  const inject = buildInjectScript(origin, pageURL);
  const baseTag = `<base href="${escapeAttr(base.origin + "/")}">`;

  if (/<head[^>]*>/i.test(out)) {
    out = out.replace(/<head([^>]*)>/i, `<head$1>${baseTag}${inject}`);
  } else if (/<html[^>]*>/i.test(out)) {
    out = out.replace(/<html([^>]*)>/i, `<html$1><head>${baseTag}${inject}</head>`);
  } else {
    out = baseTag + inject + out;
  }

  return out;
}

/** rewriteResource rewrites absolute URLs inside JS/CSS/JSON responses. */
export function rewriteResource(text: string, pageURL: string, origin: string): string {
  const base = new URL(pageURL);

  let out = text.replace(/\burl\s*\(\s*(["']?)([^"')]+)\1\s*\)/gi, (_m, quote: string, val: string) => {
    return `url(${quote}${absProxy(val.trim(), base, origin)}${quote})`;
  });

  out = out.replace(/https?:\/\/[^\s"'<>\\)]+/g, (match) => {
    if (match.startsWith(origin + "/fetch")) {
      return match;
    }
    return absProxy(match, base, origin);
  });

  return out;
}

export function isHTML(contentType: string | null): boolean {
  if (!contentType) {
    return false;
  }
  const ct = contentType.split(";")[0].trim().toLowerCase();
  return ct === "text/html" || ct === "application/xhtml+xml";
}

export function isRewritableResource(contentType: string | null): boolean {
  if (!contentType) {
    return false;
  }
  const ct = contentType.split(";")[0].trim().toLowerCase();
  return REWRITABLE_TYPES.has(ct);
}

function escapeAttr(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}
