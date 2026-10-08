package service

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"rabhana/db/sqlc"
	notifModel "rabhana/notification/model"
)

type recordingSender struct {
	mu   sync.Mutex
	sent []map[string]string
}

func (r *recordingSender) SendToUser(_ context.Context, _ int32, _, _ string, data map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, data)
}

func (r *recordingSender) Send(context.Context, int32, notifModel.EventType, map[string]string) {}

func (r *recordingSender) take() []map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.sent
	r.sent = nil
	return out
}

// Demo posts: one at a time, and only a few notifications a day per member,
// in daytime, hours apart; never "ending soon" reminders. Runs against a
// freshly migrated database: CRON_TEST_DATABASE_URL.
func TestSeedPostsAndTheirNotifications(t *testing.T) {
	url := os.Getenv("CRON_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("CRON_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var others int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&others); err != nil {
		t.Fatal(err)
	}
	if others > 0 {
		t.Skip("needs a freshly migrated database with no users")
	}
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM sell_auctions`)
		pool.Exec(c, `DELETE FROM buy_requests`)
		pool.Exec(c, `DELETE FROM users`)
	})
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	count := func(sql string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, sql).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	q := sqlc.New(pool)

	// --- the seeder posts one listing per slot, alternating sell and buy
	seeder := NewSeedService(q, 24, "https://example.com/x.png")
	seeder.SeedAuctions(ctx)
	seeder.SeedAuctions(ctx) // not due yet
	if s, b := count(`SELECT COUNT(*) FROM sell_auctions`), count(`SELECT COUNT(*) FROM buy_requests`); s != 1 || b != 0 {
		t.Fatalf("first slot posted %d sell, %d buy; want exactly one sell", s, b)
	}
	gap := seeder.nextSeedAt.Sub(time.Now())
	if gap < seedMinGap-time.Second || gap > seedMaxGap {
		t.Errorf("next demo post in %v, want between %v and %v", gap, seedMinGap, seedMaxGap)
	}
	seeder.nextSeedAt = time.Time{}
	seeder.SeedAuctions(ctx)
	if b := count(`SELECT COUNT(*) FROM buy_requests`); b != 1 {
		t.Fatalf("second slot posted %d buy requests; want one", b)
	}
	exec(`DELETE FROM sell_auctions`)
	exec(`DELETE FROM buy_requests`)

	// --- notifications
	var seed, owner, member int32
	pool.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, seedUserEmail).Scan(&seed)
	for _, u := range []struct {
		email string
		id    *int32
	}{{"owner@cron.test", &owner}, {"member@cron.test", &member}} {
		if err := pool.QueryRow(ctx, `INSERT INTO users (email, name, status, job_key) VALUES ($1, $1, 'active', 'trader') RETURNING id`, u.email).Scan(u.id); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO user_interests (user_id, interest_id) VALUES ($1, $2)`, member, seedMeatInterestID)
	var region int32
	pool.QueryRow(ctx, `SELECT id FROM regions ORDER BY id LIMIT 1`).Scan(&region)
	post := func(ownerID int32, endsIn time.Duration) {
		exec(`INSERT INTO buy_requests (owner_id, region_id, interest_id, title, image_url, unit, quantity, end_time)
			VALUES ($1, $2, $3, 'طلب', 'x.png', 'kg', 10, $4)`, ownerID, region, seedMeatInterestID, time.Now().Add(endsIn))
	}

	cairo, _ := time.LoadLocation(seedNotifyTimezone)
	clock := time.Date(2026, 10, 9, 10, 0, 0, 0, cairo) // 10 AM
	sender := &recordingSender{}
	cron := &CronService{queries: q, notificationSender: sender, now: func() time.Time { return clock }}
	announced := func() int {
		cron.processNewListings(ctx)
		n := 0
		for _, d := range sender.take() {
			if d["type"] == "new_buy_request" {
				n++
			}
		}
		return n
	}

	post(seed, 24*time.Hour)
	if n := announced(); n != 1 {
		t.Fatalf("10 AM, first demo post: %d notifications, want 1", n)
	}
	post(seed, 24*time.Hour)
	if n := announced(); n != 0 {
		t.Errorf("demo post right after another: %d notifications, want 0 (too soon)", n)
	}
	post(owner, 24*time.Hour)
	if n := announced(); n != 1 {
		t.Errorf("real post: %d notifications, want 1 — real posts are never limited", n)
	}
	exec(`UPDATE seed_notification_quota SET last_sent_at = NOW() - interval '181 minutes'`)
	post(seed, 24*time.Hour)
	if n := announced(); n != 1 {
		t.Errorf("demo post three hours later: %d notifications, want 1", n)
	}
	exec(`UPDATE seed_notification_quota SET sent_today = 5, last_sent_at = NOW() - interval '181 minutes'`)
	post(seed, 24*time.Hour)
	if n := announced(); n != 0 {
		t.Errorf("sixth demo notification of the day: sent, want blocked")
	}
	clock = clock.Add(24 * time.Hour) // next day, 10 AM
	post(seed, 24*time.Hour)
	if n := announced(); n != 1 {
		t.Errorf("next morning: %d notifications, want 1 (daily count resets)", n)
	}
	exec(`UPDATE seed_notification_quota SET last_sent_at = NOW() - interval '181 minutes'`)
	for _, hour := range []int{23, 3, 8} {
		clock = time.Date(2026, 10, 10, hour, 30, 0, 0, cairo)
		post(seed, 24*time.Hour)
		if n := announced(); n != 0 {
			t.Errorf("%d:30 Cairo: %d demo notifications, want 0 (night)", hour, n)
		}
	}
	if left := count(`SELECT COUNT(*) FROM buy_requests WHERE notified_at IS NULL`); left != 0 {
		t.Errorf("%d posts left unmarked; they would be scanned again every minute", left)
	}

	// --- "ending soon" reminders: real posts only
	clock = time.Date(2026, 10, 10, 12, 0, 0, 0, cairo)
	post(seed, 15*time.Minute)
	post(owner, 15*time.Minute)
	cron.processMotivationalMessages(ctx)
	reminders := 0
	for _, d := range sender.take() {
		if d["type"] == "request_motivation" {
			reminders++
		}
	}
	if reminders != 1 {
		t.Errorf("%d reminders sent, want 1 (the real post only)", reminders)
	}
	if left := count(`SELECT COUNT(*) FROM buy_requests WHERE end_time < NOW() + interval '30 minutes' AND last_motivation_sent_at IS NULL`); left != 0 {
		t.Errorf("%d ending posts left unmarked", left)
	}
}
