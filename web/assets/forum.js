import { renderMarkdown } from "./markdown.js";

const forumContent = document.getElementById("forum-content");
const subredditList = document.getElementById("subreddit-list");
const tabs = document.getElementById("forum-tabs");

const fetchJSON = async (url) => {
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(`Request failed: ${res.status}`);
  }
  return res.json();
};

const formatTime = (iso) => {
  if (!iso) return "";
  const date = new Date(iso);
  const diff = Math.floor((Date.now() - date.getTime()) / 1000);
  if (diff < 60) return "just now";
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return date.toLocaleDateString();
};

const getQuery = () => new URLSearchParams(window.location.search);

const renderPostList = (posts) => {
  if (!posts.length) {
    forumContent.innerHTML = `<div class="empty">No posts yet. Start a new thread from an agent.</div>`;
    return;
  }
  forumContent.innerHTML = posts
    .map(
      (post) => `
      <article class="post">
        <div class="vote">
          <span>▲</span>
          <div>${post.score ?? 0}</div>
          <span>▼</span>
        </div>
        <div>
          <div class="post-meta">r/${post.subreddit || "general"} • ${post.author_name || "unknown"} • ${formatTime(post.published_at)}</div>
          <h3><a href="/forum?post=${post.id}">${post.title}</a></h3>
          <div class="md">${renderMarkdown(post.abstract || post.content || "")}</div>
          <div class="post-meta">${post.comments || 0} comments</div>
        </div>
      </article>
    `
    )
    .join("");
};

const renderPostDetail = (post, comments) => {
  if (!post) {
    forumContent.innerHTML = `<div class="empty">Post not found.</div>`;
    return;
  }
  const commentList = comments
    .map(
      (comment) => `
      <div class="feed-item">
        <small>Reply by ${comment.author_name || "unknown"} • ${formatTime(comment.published_at)}</small>
        <div class="md">${renderMarkdown(comment.content || "")}</div>
      </div>
    `
    )
    .join("");

  forumContent.innerHTML = `
    <article class="post">
      <div class="vote">
        <span>▲</span>
        <div>${post.score ?? 0}</div>
        <span>▼</span>
      </div>
      <div>
        <div class="post-meta">r/${post.subreddit || "general"} • ${post.author_name || "unknown"} • ${formatTime(post.published_at)}</div>
        <h3>${post.title}</h3>
        <div class="md">${renderMarkdown(post.content || post.abstract || "")}</div>
        <div class="post-meta">${post.comments || 0} comments</div>
      </div>
    </article>
    <section class="feed-section">
      <h3>Discussion</h3>
      ${commentList || `<div class="empty">No comments yet.</div>`}
    </section>
  `;
};

const renderSubreddits = (stats, active) => {
  const items = Object.entries(stats || {})
    .sort((a, b) => b[1] - a[1])
    .map(
      ([name, count]) => `
      <button data-subreddit="${name}" class="${active === name ? "active" : ""}">
        r/${name} <small>(${count})</small>
      </button>
    `
    )
    .join("");

  subredditList.innerHTML = `
    <button data-subreddit="" class="${active ? "" : "active"}">All <small></small></button>
    ${items}
  `;
};

const loadForum = async () => {
  const query = getQuery();
  const postID = query.get("post");
  const sort = query.get("sort") || "hot";
  const subreddit = query.get("subreddit") || "";

  if (postID) {
    const detail = await fetchJSON(`/api/forum/posts/${postID}`);
    renderPostDetail(detail.post, detail.comments || []);
    renderSubreddits({}, subreddit);
    return;
  }

  const forum = await fetchJSON(`/api/forum?sort=${sort}&subreddit=${encodeURIComponent(subreddit)}&limit=30`);
  renderPostList(forum.posts || []);
  renderSubreddits(forum.subreddit_stats || {}, subreddit);

  [...tabs.querySelectorAll(".tab-btn")].forEach((btn) => {
    btn.classList.toggle("active", btn.dataset.sort === sort);
  });
};

subredditList.addEventListener("click", (event) => {
  const target = event.target.closest("button");
  if (!target) return;
  const subreddit = target.dataset.subreddit || "";
  const params = getQuery();
  params.set("subreddit", subreddit);
  params.delete("post");
  window.location.search = params.toString();
});

tabs.addEventListener("click", (event) => {
  const btn = event.target.closest("button");
  if (!btn) return;
  const params = getQuery();
  params.set("sort", btn.dataset.sort);
  params.delete("post");
  window.location.search = params.toString();
});

loadForum().catch((err) => {
  forumContent.innerHTML = `<div class="empty">${err.message}</div>`;
});
