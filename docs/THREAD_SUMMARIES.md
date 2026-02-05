# 线程摘要缓存（Thread Summaries）

本系统只在“多帖汇总”时使用摘要缓存，单帖分析不做摘要。

## 使用规则
- 单个帖子内分析：使用 `read_post`，不做摘要。
- 多帖汇总：使用 `get_thread_digest` 获取每个帖子的摘要或增量。
- 当返回 `needs_summary=true` 时：用 `read_post` 阅读该帖全量内容后生成摘要，并调用 `save_thread_summary` 保存。

## 工具输入与输出
### get_thread_digest
输入：
- `post_id` 线程根帖子 ID
- `max_new_comments` 新回复上限（默认 20，范围 1-30）

输出字段：
- `summary` 已缓存的线程摘要
- `summary_updated_at` 摘要更新时间（RFC3339）
- `needs_summary` 是否需要重新摘要
- `resummary_reason` 触发原因
- `new_comments` 摘要后的新增回复（增量输入格式）
- `truncated` 是否因新回复过多而截断
- `thread_long` 是否为长帖/长线程

增量输入格式 `new_comments[]`：
- `id` 回复 ID
- `parent_id` 父评论 ID（顶层回复时为根贴 ID）
- `depth` 回复层级（根贴下为 1，回复评论为 2+）
- `author_name` 作者名
- `content` 回复内容（可能被截断）
- `parent_excerpt` 被回复评论的摘录（仅当回复评论时）
- `parent_oversize` 被回复评论是否过长（为 true 时建议重摘要）

### save_thread_summary
输入：
- `post_id` 线程根帖子 ID
- `summary` 线程摘要内容

输出：
- `summary_updated_at`
- `comment_count`
- `last_comment_id`

## 触发重新摘要的条件
- `missing_summary_long_thread`：长帖/长线程且无摘要。
- `post_content_changed`：主帖内容被修改（内容哈希变化）。
- `too_many_new_comments`：新增回复超过 `max_new_comments`。
- `new_comment_too_long`：新增回复过长（默认 >1200 字符，会被截断）。
- `reply_to_large_parent`：回复了过长的父评论（默认 >180 字符）。

## 长帖/长线程判断
- 主帖内容长度 ≥ 2000
- 或线程总字符数 ≥ 6000
- 或回复数 ≥ 18

## 推荐汇总流程
1. 多帖汇总时逐帖调用 `get_thread_digest`。
2. 若 `needs_summary=true`，先 `read_post` 获取全量内容，再产出摘要并 `save_thread_summary`。
3. 汇总时优先使用每个帖子的摘要 + `new_comments` 增量。
