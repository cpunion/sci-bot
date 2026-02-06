// Static data access helpers.
//
// The frontend reads simulation outputs directly from `./data/...` so the site
// can be hosted as pure static files (GitHub Pages, S3, etc.) without a server API.

const normalizePath = (relativePath) => {
  return String(relativePath || "")
    .trim()
    .replace(/^\/+/, "")
    .replace(/\\/g, "/");
};

// In-memory fetch cache (per page load). This is important because:
// - The Go dev server sets `Cache-Control: no-store` on `/data/*` so browsers
//   won't reuse HTTP cache entries.
// - Some pages hydrate events by reading per-agent daily JSONL, which can
//   otherwise trigger duplicate requests.
const _textCache = new Map(); // key: normalized relativePath -> Promise<string>

export const fetchText = async (relativePath) => {
  const key = normalizePath(relativePath);
  if (_textCache.has(key)) return _textCache.get(key);

  const p = (async () => {
    const url = new URL(`../data/${key}`, import.meta.url);
    const res = await fetch(url);
    if (!res.ok) {
      throw new Error(`Request failed: ${res.status}`);
    }
    return res.text();
  })();

  _textCache.set(key, p);
  try {
    return await p;
  } catch (err) {
    _textCache.delete(key);
    throw err;
  }
};

export const fetchJSON = async (relativePath) => {
  const text = await fetchText(relativePath);
  try {
    return JSON.parse(text);
  } catch (err) {
    throw new Error(`Invalid JSON: ${relativePath}`);
  }
};

export const fetchJSONL = async (relativePath) => {
  const text = await fetchText(relativePath);
  const lines = String(text).split(/\r?\n/);
  const out = [];
  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    try {
      out.push(JSON.parse(trimmed));
    } catch (_err) {
      // ignore partial lines / corrupt entries
    }
  }
  return out;
};

let _manifest = null;
export const loadManifest = async () => {
  if (_manifest) return _manifest;
  try {
    _manifest = await fetchJSON("site.json");
    return _manifest;
  } catch (_err) {
    _manifest = {
      logs: [],
      forum_path: "forum/forum.json",
      journal_path: "journal/journal.json",
      agents_path: "agents/agents.json",
      feed_index_path: "feed/index.json",
    };
    return _manifest;
  }
};

let _agents = null;
export const loadAgents = async () => {
  if (_agents) return _agents;
  const manifest = await loadManifest();
  const path = manifest?.agents_path || "agents/agents.json";
  const data = await fetchJSON(path);
  _agents = data?.agents || [];
  return _agents;
};

export const resolveAgentID = (rawHandle, agents = []) => {
  const key = String(rawHandle || "").trim().replace(/^@+/, "");
  if (!key) return "";
  const byID = agents.find((a) => String(a.id || "").toLowerCase() === key.toLowerCase());
  if (byID?.id) return byID.id;
  const matches = agents.filter((a) => String(a.name || "").toLowerCase() === key.toLowerCase());
  if (matches.length === 1 && matches[0]?.id) return matches[0].id;
  // Fallback: allow passing the raw value (helps with pre-migration data).
  return key;
};

export const agentProfileURL = (handleOrID) => {
  const h = String(handleOrID || "").trim();
  if (!h) return "./agent.html";
  return `./agent.html?id=${encodeURIComponent(h)}`;
};

export const forumPostURL = (postID) => `./forum.html?post=${encodeURIComponent(postID)}`;

export const forumCommentURL = (rootPostID, commentID) =>
  `./forum.html?post=${encodeURIComponent(rootPostID)}#${encodeURIComponent(commentID)}`;

export const paperURL = (paperID) => `./paper.html?id=${encodeURIComponent(paperID)}`;
