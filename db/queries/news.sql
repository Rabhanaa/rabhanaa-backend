-- ------------------------------------------------------------------ admin

-- name: CreateNews :one
INSERT INTO news (
    title, summary, body_html, cover_image_url, source_url, source_name,
    source_text, source_images, ai_provider, ai_model, created_by_admin_id,
    notification_title, notification_body
) VALUES (
    @title, @summary, @body_html, @cover_image_url, @source_url, @source_name,
    @source_text, @source_images, @ai_provider, @ai_model, @created_by_admin_id,
    @notification_title, @notification_body
)
RETURNING *;

-- name: UpdateNews :one
UPDATE news SET
    title           = @title,
    summary         = @summary,
    body_html       = @body_html,
    cover_image_url = @cover_image_url,
    source_url      = @source_url,
    source_name     = @source_name,
    source_text     = @source_text,
    source_images   = @source_images,
    ai_provider     = @ai_provider,
    ai_model        = @ai_model,
    notification_title = @notification_title,
    notification_body  = @notification_body,
    updated_at      = NOW()
WHERE public_id = @public_id
RETURNING *;

-- name: GetNewsByPublicID :one
SELECT * FROM news WHERE public_id = $1;

-- name: AdminListNews :many
SELECT * FROM news
WHERE (@status::text = '' OR status = @status::text)
ORDER BY COALESCE(published_at, updated_at) DESC, id DESC
LIMIT @lim OFFSET @off;

-- name: AdminCountNews :one
SELECT COUNT(*) FROM news
WHERE (@status::text = '' OR status = @status::text);

-- name: DeleteNews :execrows
DELETE FROM news WHERE public_id = $1;

-- First publish stamps published_at; publishing again after an unpublish keeps
-- the original date so the story does not jump back to the top of the list.
-- name: PublishNews :one
UPDATE news SET
    status       = 'published',
    published_at = COALESCE(published_at, NOW()),
    updated_at   = NOW()
WHERE public_id = $1
RETURNING *;

-- name: UnpublishNews :one
UPDATE news SET status = 'draft', updated_at = NOW()
WHERE public_id = $1
RETURNING *;

-- Claims the notification for this story. Only one caller can win, so a double
-- click on "publish" cannot notify members twice.
-- name: ClaimNewsNotification :one
UPDATE news SET notified_at = NOW()
WHERE id = $1 AND notified_at IS NULL
RETURNING id;

-- name: SetNewsNotifiedCount :exec
UPDATE news SET notified_count = $2 WHERE id = $1;

-- name: DeleteNewsInterests :exec
DELETE FROM news_interests WHERE news_id = $1;

-- name: AddNewsInterest :exec
INSERT INTO news_interests (news_id, interest_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ListNewsInterests :many
SELECT ni.news_id, i.id, i.name_ar
FROM news_interests ni
JOIN interests i ON i.id = ni.interest_id
WHERE ni.news_id = ANY(@news_ids::int[])
ORDER BY i.name_ar;

-- Members to notify about a story: every active member, Pro or not — a
-- non-Pro member who taps it is shown the story's teaser and an upgrade
-- prompt. by_interest narrows it to members who share one of the story's
-- interests. Carriers and admins are not the audience.
-- name: ListNewsRecipients :many
SELECT u.id,
       EXISTS (
           SELECT 1 FROM user_subscriptions us
           WHERE us.user_id = u.id
             AND us.tier_name = 'pro'
             AND us.is_active = TRUE
             AND (us.expires_at IS NULL OR us.expires_at > NOW())
       ) AS is_pro
FROM users u
WHERE u.status = 'active'
  AND u.job_key <> 'shipping_company'
  AND COALESCE(u.role, '') <> 'admin'
  AND (
      NOT @by_interest::bool
      OR EXISTS (
          SELECT 1 FROM user_interests ui
          JOIN news_interests ni ON ni.interest_id = ui.interest_id
          WHERE ui.user_id = u.id AND ni.news_id = @news_id::int
      )
  )
ORDER BY u.id;

-- How many of the given members can actually receive a push right now.
-- name: CountUsersWithActiveDevice :one
SELECT COUNT(DISTINCT user_id) FROM device_tokens
WHERE is_active = TRUE AND user_id = ANY(@user_ids::int[]);

-- --------------------------------------------------------------- members

-- name: IsProUser :one
SELECT EXISTS (
    SELECT 1 FROM user_subscriptions
    WHERE user_id = $1
      AND tier_name = 'pro'
      AND is_active = TRUE
      AND (expires_at IS NULL OR expires_at > NOW())
) AS is_pro;

-- name: ListPublishedNews :many
SELECT n.public_id, n.title, n.summary, n.cover_image_url, n.published_at,
       n.published_at > COALESCE(
           (SELECT ns.last_seen_at FROM news_seen ns WHERE ns.user_id = @user_id),
           '-infinity'::timestamptz
       ) AS is_new
FROM news n
WHERE n.status = 'published'
ORDER BY n.published_at DESC, n.id DESC
LIMIT @lim OFFSET @off;

-- name: CountPublishedNews :one
SELECT COUNT(*) FROM news WHERE status = 'published';

-- name: GetPublishedNews :one
SELECT * FROM news WHERE public_id = $1 AND status = 'published';

-- name: CountUnreadNews :one
SELECT COUNT(*) FROM news n
WHERE n.status = 'published'
  AND n.published_at > COALESCE(
      (SELECT ns.last_seen_at FROM news_seen ns WHERE ns.user_id = $1),
      '-infinity'::timestamptz
  );

-- name: MarkNewsSeen :exec
INSERT INTO news_seen (user_id, last_seen_at) VALUES ($1, NOW())
ON CONFLICT (user_id) DO UPDATE SET last_seen_at = NOW();

-- ------------------------------------------------------------- analytics

-- name: InsertNewsDeliveries :exec
-- Two unnests in the select list are zipped row by row, so user_ids[i] gets
-- was_pro[i]. The arrays are always built together and are the same length.
INSERT INTO news_deliveries (news_id, user_id, was_pro)
SELECT @news_id::int, unnest(@user_ids::int[]), unnest(@was_pro::bool[])
ON CONFLICT (news_id, user_id) DO NOTHING;

-- name: MarkNewsDelivered :exec
UPDATE news_deliveries SET delivered = TRUE WHERE news_id = $1 AND user_id = $2;

-- Flags only ever turn on: a member who opened from the push once stays
-- counted as a push open however they come back.
-- name: RecordNewsView :exec
INSERT INTO news_views (news_id, user_id, from_push, read_as_pro, gated)
VALUES (@news_id, @user_id, @from_push, @read_as_pro, @gated)
ON CONFLICT (news_id, user_id) DO UPDATE SET
    from_push      = news_views.from_push OR EXCLUDED.from_push,
    read_as_pro    = news_views.read_as_pro OR EXCLUDED.read_as_pro,
    gated          = news_views.gated OR EXCLUDED.gated,
    view_count     = news_views.view_count + 1,
    last_viewed_at = NOW();

-- name: MarkNewsReadToEnd :execrows
UPDATE news_views SET read_to_end = TRUE
WHERE news_id = $1 AND user_id = $2 AND read_as_pro;

-- name: MarkNewsUpgradeClicked :execrows
UPDATE news_views SET upgrade_clicked = TRUE
WHERE news_id = $1 AND user_id = $2 AND gated;

-- name: NewsDeliveryStats :one
SELECT COUNT(*)                                   AS recipients,
       COUNT(*) FILTER (WHERE was_pro)            AS recipients_pro,
       COUNT(*) FILTER (WHERE delivered)          AS delivered,
       COUNT(*) FILTER (WHERE delivered AND was_pro) AS delivered_pro
FROM news_deliveries WHERE news_id = $1;

-- "Pro" below means read it and never hit the prompt; "free" means hit the
-- prompt, whether or not they upgraded afterwards (converted).
-- name: NewsViewStats :one
SELECT COUNT(*)                                                  AS viewers,
       COUNT(*) FILTER (WHERE read_as_pro AND NOT gated)         AS viewers_pro,
       COUNT(*) FILTER (WHERE gated)                             AS viewers_free,
       COUNT(*) FILTER (WHERE from_push)                         AS push_opens,
       COUNT(*) FILTER (WHERE from_push AND read_as_pro AND NOT gated) AS push_opens_pro,
       COUNT(*) FILTER (WHERE from_push AND gated)               AS push_opens_free,
       COUNT(*) FILTER (WHERE read_to_end)                       AS read_to_end,
       COUNT(*) FILTER (WHERE upgrade_clicked)                   AS upgrade_clicks,
       COUNT(*) FILTER (WHERE gated AND read_as_pro)             AS converted,
       COALESCE(SUM(view_count), 0)::bigint                      AS total_views
FROM news_views WHERE news_id = $1;

-- name: ListNewsViewers :many
SELECT u.public_id, u.name, u.phone, u.email,
       v.from_push, v.read_as_pro, v.gated, v.read_to_end, v.upgrade_clicked,
       v.view_count, v.first_viewed_at, v.last_viewed_at
FROM news_views v
JOIN users u ON u.id = v.user_id
WHERE v.news_id = @news_id
  AND (
      @filter::text = ''
      OR (@filter::text = 'pro' AND v.read_as_pro AND NOT v.gated)
      OR (@filter::text = 'free' AND v.gated)
      OR (@filter::text = 'push' AND v.from_push)
      OR (@filter::text = 'upgrade' AND v.upgrade_clicked)
  )
ORDER BY v.last_viewed_at DESC
LIMIT @lim OFFSET @off;

-- name: CountNewsViewers :one
SELECT COUNT(*) FROM news_views v
WHERE v.news_id = @news_id
  AND (
      @filter::text = ''
      OR (@filter::text = 'pro' AND v.read_as_pro AND NOT v.gated)
      OR (@filter::text = 'free' AND v.gated)
      OR (@filter::text = 'push' AND v.from_push)
      OR (@filter::text = 'upgrade' AND v.upgrade_clicked)
  );

-- name: CountNewsViewersByNews :many
SELECT news_id, COUNT(*) AS viewers FROM news_views
WHERE news_id = ANY(@news_ids::int[])
GROUP BY news_id;
