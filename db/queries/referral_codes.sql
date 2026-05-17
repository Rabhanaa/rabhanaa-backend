-- name: GetReferralCodeByUser :one
SELECT * FROM referral_codes WHERE user_id = $1;

-- name: GetReferralCodeByCode :one
SELECT * FROM referral_codes WHERE code = $1 AND is_active = TRUE;

-- name: CreateReferralCode :one
INSERT INTO referral_codes (user_id, code) VALUES ($1, $2) RETURNING *;

-- name: IncrementReferralUsage :exec
UPDATE referral_codes SET usage_count = usage_count + 1 WHERE id = $1;

-- name: CreateReferralUsage :exec
INSERT INTO referral_usages (referral_code_id, referred_user_id, discount_applied)
VALUES ($1, $2, $3);
