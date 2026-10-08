-- Takes one of a member's demo-notification slots for today, if one is free:
-- fewer than daily_max sent today, and the last one at least min_gap_minutes
-- ago. Returns the member's id when the slot was taken, no row when it was not.
-- A new Cairo day starts the count again.
-- name: ClaimSeedNotification :one
INSERT INTO seed_notification_quota AS q (user_id, day, sent_today, last_sent_at)
VALUES (@user_id, @day, 1, NOW())
ON CONFLICT (user_id) DO UPDATE SET
    sent_today   = CASE WHEN q.day = EXCLUDED.day THEN q.sent_today + 1 ELSE 1 END,
    day          = EXCLUDED.day,
    last_sent_at = NOW()
WHERE q.day <> EXCLUDED.day
   OR (q.sent_today < @daily_max::int
       AND q.last_sent_at <= NOW() - make_interval(mins => @min_gap_minutes::int))
RETURNING q.user_id;
