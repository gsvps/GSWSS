const PROXY_ATTRS = ["href", "src", "action", "poster", "data-src"];

/** rewriteHTML rewrites resource and navigation URLs to go through /fetch. */
export function rewriteHTML(html: string, pageURL: string, origin: string): string {
  const base = new URL(pageURL);
  const proxyURL = (raw: string): string => {
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
  };

  let out = html;
  for (const attr of PROXY_ATTRS) {
    const re = new RegExp(`(\\s${attr}\\s*=\\s*)(["'])([^"']*)\\2`, "gi");
    out = out.replace(re, (_m, prefix: string, quote: string, val: string) => {
      return `${prefix}${quote}${proxyURL(val)}${quote}`;
    });
  }

  // srcset="url1 1x, url2 2x"
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

  const inject = `<script>(function(){
var O=${JSON.stringify(origin)};
var P="/fetch";
function px(u){
  if(!u||u.charAt(0)==="#"||u.indexOf("javascript:")===0||u.indexOf("data:")===0)return u;
  try{
    var a=new URL(u,document.baseURI||location.href).href;
    if(a.indexOf(O+P)===0)return a;
    if(a.indexOf("http")===0)return P+"?url="+encodeURIComponent(a);
  }catch(e){}
  return u;
}
document.addEventListener("click",function(e){
  var el=e.target.closest("a[href]");
  if(!el)return;
  var h=el.getAttribute("href");
  if(!h||h.charAt(0)==="#")return;
  e.preventDefault();
  location.href=px(h);
},true);
document.addEventListener("submit",function(e){
  var f=e.target;
  if(!f||!f.action)return;
  var a=px(f.getAttribute("action")||f.action);
  if(a!==f.action){f.action=a;}
},true);
})();</script>`;

  if (/<head[^>]*>/i.test(out)) {
    out = out.replace(/<head([^>]*)>/i, `<head$1>${inject}`);
  } else if (/<html[^>]*>/i.test(out)) {
    out = out.replace(/<html([^>]*)>/i, `<html$1><head>${inject}</head>`);
  } else {
    out = inject + out;
  }

  return out;
}

export function isHTML(contentType: string | null): boolean {
  if (!contentType) {
    return false;
  }
  const ct = contentType.split(";")[0].trim().toLowerCase();
  return ct === "text/html" || ct === "application/xhtml+xml";
}
