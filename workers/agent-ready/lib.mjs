import { NodeHtmlMarkdown } from "node-html-markdown";

const nhm = new NodeHtmlMarkdown({
  useInlineLinks: true,
});

const HTML_CONTENT_TYPE_RE = /^(text\/html|application\/xhtml\+xml)\b/i;
const TITLE_RE = /<title>([\s\S]*?)<\/title>/i;
const MAIN_RE = /<main\b[^>]*>([\s\S]*?)<\/main>/i;
const BODY_RE = /<body\b[^>]*>([\s\S]*?)<\/body>/i;
const META_DESC_RE =
  /<meta\b[^>]+name=(["'])description\1[^>]+content=(["'])([\s\S]*?)\2[^>]*>/i;
const META_DESC_ALT_RE =
  /<meta\b[^>]+content=(["'])([\s\S]*?)\1[^>]+name=(["'])description\3[^>]*>/i;
const ROOT_RELATIVE_ATTR_RE = /\b(href|src)=("|')\/(?!\/)([^"']*)\2/gi;
const SCRIPT_RE = /<script\b[\s\S]*?<\/script>/gi;
const STYLE_RE = /<style\b[\s\S]*?<\/style>/gi;
const NOSCRIPT_RE = /<noscript\b[\s\S]*?<\/noscript>/gi;

export function isHtmlContentType(contentType = "") {
  return HTML_CONTENT_TYPE_RE.test(contentType);
}

export function appendHeaderToken(existingValue, token) {
  const values = existingValue
    ? existingValue
        .split(",")
        .map((value) => value.trim())
        .filter(Boolean)
    : [];
  const tokenLower = token.toLowerCase();
  if (!values.some((value) => value.toLowerCase() === tokenLower)) {
    values.push(token);
  }
  return values.join(", ");
}

export function parseAcceptHeader(headerValue) {
  if (!headerValue) {
    return [];
  }

  return headerValue
    .split(",")
    .map((part, order) => {
      const [mediaRange, ...params] = part.split(";").map((value) => value.trim().toLowerCase());
      let q = 1;

      for (const param of params) {
        if (!param.startsWith("q=")) {
          continue;
        }
        const parsed = Number.parseFloat(param.slice(2));
        if (Number.isFinite(parsed)) {
          q = parsed;
        }
      }

      return { mediaRange, order, q };
    })
    .filter(({ mediaRange, q }) => mediaRange && q > 0);
}

function mediaRangeSpecificity(mediaRange, mediaType) {
  const [rangeType, rangeSubtype] = mediaRange.split("/");
  const [targetType, targetSubtype] = mediaType.toLowerCase().split("/");
  if (!rangeType || !rangeSubtype) {
    return -1;
  }
  if (rangeType !== "*" && rangeType !== targetType) {
    return -1;
  }
  if (rangeSubtype !== "*" && rangeSubtype !== targetSubtype) {
    return -1;
  }

  let specificity = 0;
  if (rangeType !== "*") {
    specificity++;
  }
  if (rangeSubtype !== "*") {
    specificity++;
  }
  return specificity;
}

export function qualityForMediaType(headerValue, mediaType) {
  let best = { q: 0, specificity: -1, order: Number.POSITIVE_INFINITY };

  for (const item of parseAcceptHeader(headerValue)) {
    const specificity = mediaRangeSpecificity(item.mediaRange, mediaType);
    if (specificity < 0) {
      continue;
    }

    const isBetterMatch =
      item.q > best.q ||
      (item.q === best.q && specificity > best.specificity) ||
      (item.q === best.q && specificity === best.specificity && item.order < best.order);

    if (isBetterMatch) {
      best = { q: item.q, specificity, order: item.order };
    }
  }

  return best.q;
}

export function prefersMarkdown(headerValue) {
  const acceptItems = parseAcceptHeader(headerValue);
  const explicitlyRequestsMarkdown = acceptItems.some(
    ({ mediaRange }) => mediaRange === "text/markdown",
  );
  if (!explicitlyRequestsMarkdown) {
    return false;
  }

  const markdownQ = qualityForMediaType(headerValue, "text/markdown");
  if (markdownQ === 0) {
    return false;
  }

  const htmlQ = Math.max(
    qualityForMediaType(headerValue, "text/html"),
    qualityForMediaType(headerValue, "application/xhtml+xml"),
  );

  return markdownQ >= htmlQ;
}

function decodeHtmlEntities(value) {
  const named = {
    amp: "&",
    apos: "'",
    gt: ">",
    lt: "<",
    mdash: "—",
    nbsp: " ",
    quot: '"',
  };

  return value.replace(/&(#x[0-9a-f]+|#\d+|[a-z]+);/gi, (match, entity) => {
    const lower = entity.toLowerCase();
    if (lower.startsWith("#x")) {
      const codePoint = Number.parseInt(lower.slice(2), 16);
      return Number.isFinite(codePoint) ? String.fromCodePoint(codePoint) : match;
    }
    if (lower.startsWith("#")) {
      const codePoint = Number.parseInt(lower.slice(1), 10);
      return Number.isFinite(codePoint) ? String.fromCodePoint(codePoint) : match;
    }
    return named[lower] ?? match;
  });
}

function extractTagMatch(html, pattern) {
  const match = html.match(pattern);
  return match ? match[1].trim() : "";
}

function extractTitle(html) {
  return decodeHtmlEntities(extractTagMatch(html, TITLE_RE));
}

function extractDescription(html) {
  const directMatch = html.match(META_DESC_RE);
  if (directMatch?.[3]) {
    return decodeHtmlEntities(directMatch[3].trim());
  }

  const alternateMatch = html.match(META_DESC_ALT_RE);
  if (alternateMatch?.[2]) {
    return decodeHtmlEntities(alternateMatch[2].trim());
  }

  return "";
}

function makeLinksAbsolute(fragment, url) {
  return fragment.replace(
    ROOT_RELATIVE_ATTR_RE,
    (_, attribute, quote, path) => `${attribute}=${quote}${url.origin}/${path}${quote}`,
  );
}

function cleanHtmlFragment(fragment) {
  return fragment.replace(SCRIPT_RE, "").replace(STYLE_RE, "").replace(NOSCRIPT_RE, "");
}

export function extractReadableHtml(html, url) {
  const fragment =
    extractTagMatch(html, MAIN_RE) || extractTagMatch(html, BODY_RE) || html;
  return makeLinksAbsolute(cleanHtmlFragment(fragment), url);
}

function cleanMarkdown(markdown) {
  return markdown.replace(/[ \t]+\n/g, "\n").replace(/\n{3,}/g, "\n\n").trim();
}

export function buildLinkHeaderValues(url) {
  const canonicalUrl = new URL(url.pathname || "/", url.origin);
  canonicalUrl.search = url.search;
  canonicalUrl.hash = "";

  return [
    `<${canonicalUrl.toString()}>; rel="alternate"; type="text/markdown"`,
    `<${new URL("/index.xml", url.origin).toString()}>; rel="alternate"; type="application/rss+xml"`,
    `<${new URL("/llms.txt", url.origin).toString()}>; rel="describedby"; type="text/plain"`,
  ];
}

export function withDiscoveryHeaders(response, url) {
  const headers = new Headers(response.headers);
  headers.set("Vary", appendHeaderToken(headers.get("Vary"), "Accept"));
  for (const value of buildLinkHeaderValues(url)) {
    headers.append("Link", value);
  }

  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers,
  });
}

export function htmlToAgentMarkdown(html, url) {
  const title = extractTitle(html);
  const description = extractDescription(html);
  const readableHtml = extractReadableHtml(html, url);
  const body = cleanMarkdown(nhm.translate(readableHtml));
  const lines = [];

  if (title) {
    lines.push(`# ${title}`);
  }
  lines.push(`Source: ${url.toString()}`);

  if (description) {
    lines.push("");
    lines.push(description);
  }

  if (body) {
    lines.push("");
    lines.push("---");
    lines.push("");
    lines.push(body);
  }

  return `${cleanMarkdown(lines.join("\n"))}\n`;
}

export function buildMarkdownResponse(response, url, markdown) {
  const headers = new Headers(response.headers);
  headers.set("Content-Type", "text/markdown; charset=utf-8");
  headers.set("Vary", appendHeaderToken(headers.get("Vary"), "Accept"));
  headers.delete("Content-Encoding");
  headers.delete("Content-Length");
  headers.delete("ETag");

  for (const value of buildLinkHeaderValues(url)) {
    headers.append("Link", value);
  }

  return new Response(markdown, {
    status: response.status,
    statusText: response.statusText,
    headers,
  });
}
