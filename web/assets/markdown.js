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
  let out = text;
  out = out.replace(/`([^`]+)`/g, "<code>$1</code>");
  out = out.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  out = out.replace(/\*([^*]+)\*/g, "<em>$1</em>");
  out = out.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_match, label, url) => {
    const safe = sanitizeUrl(url);
    return `<a href=\"${safe}\" target=\"_blank\" rel=\"noopener noreferrer\">${label}</a>`;
  });
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
