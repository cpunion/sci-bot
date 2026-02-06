const escapeHTML = (value = "") =>
  value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");

const sanitizeUrl = (raw) => {
  if (!raw) return "#";
  const trimmed = raw.trim();
  if (trimmed.startsWith("/") || trimmed.startsWith("#")) return trimmed;
  if (/^https?:\/\//i.test(trimmed)) return trimmed;
  if (/^mailto:/i.test(trimmed)) return trimmed;
  return "#";
};

const inlineFormat = (text) => {
  const protectedSegments = [];
  const protect = (value, re) =>
    value.replace(re, (match) => {
      const key = `__SCIBOT_SEG_${protectedSegments.length}__`;
      protectedSegments.push({ key, value: match });
      return key;
    });

  let out = text;

  // Protect math segments so emphasis/link parsing doesn't corrupt LaTeX.
  out = protect(out, /\$\$[\s\S]+?\$\$/g);
  out = protect(out, /\\\[[\s\S]+?\\\]/g);
  out = protect(out, /\\\((.+?)\\\)/g);
  out = protect(out, /\$(?!\$)([^$\n]+?)\$/g);

  // Protect inline code spans so mention/emphasis parsing doesn't touch them.
  out = out.replace(/`([^`]+)`/g, (_match, code) => {
    const key = `__SCIBOT_SEG_${protectedSegments.length}__`;
    protectedSegments.push({ key, value: `<code>${code}</code>` });
    return key;
  });
  out = out.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  out = out.replace(/\*([^*]+)\*/g, "<em>$1</em>");
  out = out.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_match, label, url) => {
    const safe = sanitizeUrl(url);
    return `<a href=\"${safe}\" target=\"_blank\" rel=\"noopener noreferrer\">${label}</a>`;
  });
  out = out.replace(/(^|[^\w])@([A-Za-z][A-Za-z0-9_-]{0,63})/g, (_match, prefix, handle) => {
    const href = `/agent/${encodeURIComponent(handle)}`;
    return `${prefix}<a class="mention" href="${href}">@${handle}</a>`;
  });

  for (const seg of protectedSegments) {
    out = out.split(seg.key).join(seg.value);
  }

  return out;
};

export const renderMarkdown = (raw = "") => {
  if (!raw) return "";
  const text = escapeHTML(raw);
  const lines = text.split(/\r?\n/);
  const out = [];
  let inCode = false;
  let listType = null;
  let buffer = [];

  const flushParagraph = () => {
    if (!buffer.length) return;
    const joined = buffer.join(" ").trim();
    if (joined) {
      out.push(`<p>${inlineFormat(joined)}</p>`);
    }
    buffer = [];
  };

  const openList = (type) => {
    if (listType === type) return;
    closeList();
    listType = type;
    out.push(type === "ol" ? "<ol>" : "<ul>");
  };

  const closeList = () => {
    if (!listType) return;
    out.push(listType === "ol" ? "</ol>" : "</ul>");
    listType = null;
  };

  for (const rawLine of lines) {
    const line = rawLine.trimEnd();

    if (line.startsWith("```")) {
      if (inCode) {
        out.push("</code></pre>");
        inCode = false;
      } else {
        flushParagraph();
        closeList();
        out.push("<pre><code>");
        inCode = true;
      }
      continue;
    }

    if (inCode) {
      out.push(line);
      continue;
    }

    if (!line.trim()) {
      flushParagraph();
      closeList();
      continue;
    }

    if (/^#{1,6}\s/.test(line)) {
      flushParagraph();
      closeList();
      const level = line.match(/^#+/)[0].length;
      const content = line.replace(/^#{1,6}\s*/, "");
      out.push(`<h${level}>${inlineFormat(content)}</h${level}>`);
      continue;
    }

    if (/^>\s?/.test(line)) {
      flushParagraph();
      closeList();
      const content = line.replace(/^>\s?/, "");
      out.push(`<blockquote>${inlineFormat(content)}</blockquote>`);
      continue;
    }

    const orderedMatch = line.match(/^\d+\.\s+(.*)$/);
    if (orderedMatch) {
      flushParagraph();
      openList("ol");
      out.push(`<li>${inlineFormat(orderedMatch[1])}</li>`);
      continue;
    }

    const unorderedMatch = line.match(/^[-*]\s+(.*)$/);
    if (unorderedMatch) {
      flushParagraph();
      openList("ul");
      out.push(`<li>${inlineFormat(unorderedMatch[1])}</li>`);
      continue;
    }

    buffer.push(line.trim());
  }

  flushParagraph();
  closeList();
  if (inCode) {
    out.push("</code></pre>");
  }

  return out.join("\n");
};

export const typesetMath = (root = document.body) => {
  const fn = window?.renderMathInElement;
  if (typeof fn !== "function" || !root) return;
  try {
    fn(root, {
      delimiters: [
        { left: "$$", right: "$$", display: true },
        { left: "\\[", right: "\\]", display: true },
        { left: "$", right: "$", display: false },
        { left: "\\(", right: "\\)", display: false },
      ],
      ignoredTags: ["script", "noscript", "style", "textarea", "pre", "code"],
      throwOnError: false,
    });
  } catch (_err) {
    // Ignore typesetting errors; keep raw text visible.
  }
};
