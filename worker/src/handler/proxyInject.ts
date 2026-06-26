/** buildInjectScript returns runtime hooks so dynamic requests also go through /fetch. */
export function buildInjectScript(origin: string, pageURL: string): string {
  const baseHref = new URL(pageURL).origin + "/";
  return `<script>(function(){
var O=${JSON.stringify(origin)},P="/fetch",B=${JSON.stringify(baseHref)};
function skip(u){if(!u)return 1;var s=String(u).trim();if(!s||s.charAt(0)==="#")return 1;if(/^(javascript|data|blob|about):/i.test(s))return 1;if(s.indexOf(O+P)===0)return 1;return 0}
function px(u){if(skip(u))return u;try{var a=new URL(u,B).href;if(a.indexOf(O+P)===0)return a;if(/^https?:/i.test(a))return P+"?url="+encodeURIComponent(a)}catch(e){}return u}
function pxSrcset(v){return v.split(",").map(function(p){var b=p.trim().split(/\\s+/);if(b[0])b[0]=px(b[0]);return b.join(" ")}).join(", ")}
var f=window.fetch;window.fetch=function(i,n){var u=typeof i==="string"?i:(i&&i.url?i.url:String(i));var p=px(u);if(p!==u){if(typeof i==="string")return f(p,n);if(i instanceof Request)return f(new Request(p,i),n);return f(p,n)}return f.apply(this,arguments)};
var xo=XMLHttpRequest.prototype.open;XMLHttpRequest.prototype.open=function(m,u){var a=[].slice.call(arguments);a[1]=px(u);return xo.apply(this,a)};
var sa=Element.prototype.setAttribute;Element.prototype.setAttribute=function(k,v){if(v&&/^(href|src|action|poster|data-src|srcset|formaction)$/i.test(k)){v=k.toLowerCase()==="srcset"?pxSrcset(v):px(v)}return sa.call(this,k,v)};
function patchProp(C,prop){if(!C||!C.prototype)return;var d=Object.getOwnPropertyDescriptor(C.prototype,prop);if(!d||!d.set)return;Object.defineProperty(C.prototype,prop,{get:d.get,set:function(v){d.set.call(this,px(v))},configurable:1,enumerable:d.enumerable})}
patchProp(HTMLScriptElement,"src");patchProp(HTMLImageElement,"src");patchProp(HTMLIFrameElement,"src");patchProp(HTMLMediaElement,"src");patchProp(HTMLLinkElement,"href");patchProp(HTMLAnchorElement,"href");
new MutationObserver(function(ms){ms.forEach(function(m){if(m.type!=="attributes"||!m.target.getAttribute)return;var n=m.attributeName;if(!n||!/^(href|src|action|poster|data-src|srcset|formaction)$/i.test(n))return;var v=m.target.getAttribute(n);if(!v)return;var p=n.toLowerCase()==="srcset"?pxSrcset(v):px(v);if(p!==v)m.target.setAttribute(n,p)})}).observe(document.documentElement,{subtree:1,attributes:1,attributeFilter:["href","src","action","poster","data-src","srcset","formaction"]});
document.addEventListener("click",function(e){var el=e.target.closest("a[href]");if(!el)return;var h=el.getAttribute("href");if(!h||h.charAt(0)==="#")return;e.preventDefault();location.href=px(h)},1);
document.addEventListener("submit",function(e){var fm=e.target;if(!fm||!fm.action)return;var a=px(fm.getAttribute("action")||fm.action);if(a!==fm.action)fm.action=a},1);
var ps=history.pushState,rs=history.replaceState;history.pushState=function(s,t,u){if(u)u=px(u);return ps.call(this,s,t,u)};history.replaceState=function(s,t,u){if(u)u=px(u);return rs.call(this,s,t,u)};
var wo=window.open;window.open=function(u,n,f){if(u)arguments[0]=px(u);return wo.apply(this,arguments)};
if(window.EventSource){var ES=window.EventSource;window.EventSource=function(u,c){return new ES(px(u),c)}}
if(navigator.sendBeacon){var sb=navigator.sendBeacon.bind(navigator);navigator.sendBeacon=function(u,d){return sb(px(u),d)}}
})();</script>`;
}
