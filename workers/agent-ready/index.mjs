import {
  buildMarkdownResponse,
  htmlToAgentMarkdown,
  isHtmlContentType,
  prefersMarkdown,
  withDiscoveryHeaders,
} from "./lib.mjs";

export default {
  async fetch(request) {
    const method = request.method.toUpperCase();
    const originRequest = new Request(request, { redirect: "manual" });
    const originResponse = await fetch(originRequest);

    if (method !== "GET" && method !== "HEAD") {
      return originResponse;
    }

    if (originResponse.status !== 200) {
      return originResponse;
    }

    const contentType = originResponse.headers.get("Content-Type") || "";
    if (!isHtmlContentType(contentType)) {
      return originResponse;
    }

    const url = new URL(request.url);
    const wantsMarkdown = method === "GET" && prefersMarkdown(request.headers.get("Accept"));

    if (!wantsMarkdown) {
      return withDiscoveryHeaders(originResponse, url);
    }

    const html = await originResponse.text();
    const markdown = htmlToAgentMarkdown(html, url);
    return buildMarkdownResponse(originResponse, url, markdown);
  },
};
