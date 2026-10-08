package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"rabhana/db/sqlc"
	"rabhana/news/model"
	"rabhana/pkg/errs"
	settingsSvc "rabhana/settings/service"
)

// fakePush delivers to everyone except the members in fail, and records who
// was sent what.
type fakePush struct {
	mu       sync.Mutex
	disabled bool
	fail     map[int32]bool
	sent     map[int32]string
	title    string
}

func (f *fakePush) PushEnabled() bool { return !f.disabled }

func (f *fakePush) SendPushOnly(_ context.Context, userID int32, title, body string, data map[string]string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent[userID] = data["news_id"]
	f.title = title + " | " + body
	return !f.fail[userID]
}

// Runs against a real, freshly migrated database: NEWS_TEST_DATABASE_URL.
// Skipped without one, so `go test ./...` stays self-contained.
func TestNotifyAllAndAnalytics(t *testing.T) {
	url := os.Getenv("NEWS_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("NEWS_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	// Cleanups run last-registered first, so the members are deleted before
	// the pool closes.
	t.Cleanup(pool.Close)
	q := sqlc.New(pool)

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	tag := uuid.NewString()[:8]
	user := func(name string, pro bool, status, jobKey, role string) int32 {
		t.Helper()
		var id int32
		err := pool.QueryRow(ctx, `INSERT INTO users (email, name, status, job_key, role) VALUES ($1, $2, $3, $4, NULLIF($5, '')) RETURNING id`,
			fmt.Sprintf("%s-%s@analytics.test", name, tag), name, status, jobKey, role).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		if pro {
			exec(`INSERT INTO user_subscriptions (user_id, tier_name, is_active, is_primary) VALUES ($1, 'pro', TRUE, TRUE)`, id)
		}
		return id
	}
	// The audience counts must be exactly this test's members, and this test
	// must never touch real data: it only runs on a database without members.
	var others int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE status = 'active'`).Scan(&others); err != nil {
		t.Fatal(err)
	}
	if others > 0 {
		t.Skip("needs a freshly migrated database with no active members")
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE email LIKE '%@analytics.test'`)
	})

	pro1 := user("pro1", true, "active", "trader", "")
	pro2 := user("pro2", true, "active", "trader", "")
	free1 := user("free1", false, "active", "trader", "")
	free2 := user("free2", false, "active", "retailer", "")
	user("carrier", false, "active", "shipping_company", "")
	admin := user("admin", true, "active", "trader", "admin")
	user("pending", false, "pending_review", "trader", "")

	settings := settingsSvc.NewService(q, nil)
	if err := settings.Set(ctx, settingsSvc.KeyNewsNotifyMode, settingsSvc.NewsNotifyAll, 0); err != nil {
		t.Fatal(err)
	}
	push := &fakePush{fail: map[int32]bool{pro2: true}, sent: map[int32]string{}}
	svc := NewService(q, pool, settings, push, nil, "")

	n, err := svc.Create(ctx, model.SaveRequest{
		Title: "خبر", Summary: "ملخص", BodyHTML: "<p>نص الخبر</p>",
		NotificationTitle: "🐔 عنوان الإشعار", NotificationBody: "نص الإشعار 📈",
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.MustParse(n.PublicID)

	aud, err := svc.Audience(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if aud.Recipients != 4 || aud.RecipientsPro != 2 || aud.RecipientsFree != 2 {
		t.Fatalf("audience = %+v, want 4 members (2 Pro, 2 not); carrier, admin and pending excluded", aud)
	}

	pub, err := svc.Publish(ctx, id, true)
	if err != nil || pub.NotifiedCount != 4 {
		t.Fatalf("publish = %+v, %v", pub, err)
	}
	// The fan-out runs in the background; wait for the deliveries to land.
	waitFor(t, func() bool {
		a, _ := svc.Analytics(ctx, id)
		return a != nil && a.Delivered == 3
	})
	if push.title != "🐔 عنوان الإشعار | نص الإشعار 📈" {
		t.Errorf("push used %q, want the notification text with its emoji", push.title)
	}

	// pro1 taps the push and reads to the end; free1 taps it, sees the prompt
	// and taps subscribe; free2 opens it from the app twice.
	if a, _, err := svc.Get(ctx, id, pro1, true); err != nil || a == nil {
		t.Fatalf("pro read: %v", err)
	}
	if err := svc.MarkReadToEnd(ctx, id, pro1); err != nil {
		t.Fatal(err)
	}
	_, teaser, err := svc.Get(ctx, id, free1, true)
	if !errors.Is(err, errs.ErrProRequired) || teaser == nil || teaser.Title != "خبر" {
		t.Fatalf("free read: teaser %+v, err %v", teaser, err)
	}
	if err := svc.UpgradeClicked(ctx, id, free1); err != nil {
		t.Fatal(err)
	}
	svc.Get(ctx, id, free2, false)
	svc.Get(ctx, id, free2, false)
	// A non-Pro member cannot mark a story read, and a Pro one cannot mark an
	// upgrade click.
	svc.MarkReadToEnd(ctx, id, free2)
	svc.UpgradeClicked(ctx, id, pro1)

	// free1 then upgrades and reads it: counted as converted.
	exec(`INSERT INTO user_subscriptions (user_id, tier_name, is_active, is_primary) VALUES ($1, 'pro', TRUE, TRUE)`, free1)
	if a, _, err := svc.Get(ctx, id, free1, false); err != nil || a == nil {
		t.Fatalf("converted read: %v", err)
	}

	a, err := svc.Analytics(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	want := model.NewsAnalytics{
		NotifiedAt: a.NotifiedAt,
		Recipients: 4, RecipientsPro: 2, RecipientsFree: 2,
		Delivered: 3, DeliveredPro: 1, DeliveredFree: 2,
		Viewers: 3, ViewersPro: 1, ViewersFree: 2,
		PushOpens: 2, PushOpensPro: 1, PushOpensFree: 1,
		ReadToEnd: 1, UpgradeClicks: 1, Converted: 1, TotalViews: 5,
	}
	if *a != want {
		t.Errorf("analytics\n got %+v\nwant %+v", *a, want)
	}

	for filter, wantN := range map[string]int64{"": 3, "pro": 1, "free": 2, "push": 2, "upgrade": 1} {
		list, err := svc.Viewers(ctx, id, filter, 1, 20)
		if err != nil || list.Total != wantN || int64(len(list.Viewers)) != wantN {
			t.Errorf("viewers(%q) = %d (%d rows), %v; want %d", filter, list.Total, len(list.Viewers), err, wantN)
		}
	}

	// --- test notification: only the admin's devices, nothing recorded
	before, _ := svc.Analytics(ctx, id)
	push.disabled = true
	if _, err := svc.SendTestNotification(ctx, id, admin, model.TestNotificationRequest{}); !errors.Is(err, errs.ErrPushUnavailable) {
		t.Errorf("push off: got %v, want ErrPushUnavailable", err)
	}
	push.disabled = false
	if _, err := svc.SendTestNotification(ctx, id, admin, model.TestNotificationRequest{}); !errors.Is(err, errs.ErrNoTestDevice) {
		t.Errorf("no device: got %v, want ErrNoTestDevice", err)
	}
	exec(`INSERT INTO device_tokens (user_id, token, platform) VALUES ($1, $2, 'web')`, admin, "admin-token-"+tag)
	push.sent = map[int32]string{}
	res, err := svc.SendTestNotification(ctx, id, admin, model.TestNotificationRequest{NotificationTitle: "✨ نص لم يُحفظ بعد"})
	if err != nil || res.Devices != 1 {
		t.Fatalf("test push = %+v, %v", res, err)
	}
	if len(push.sent) != 1 || push.sent[admin] == "" {
		t.Errorf("test push went to %v, want only the admin", push.sent)
	}
	if push.title != "✨ نص لم يُحفظ بعد | نص الإشعار 📈" {
		t.Errorf("test push said %q, want the unsaved title with the saved body", push.title)
	}
	push.fail[admin] = true
	if _, err := svc.SendTestNotification(ctx, id, admin, model.TestNotificationRequest{}); !errors.Is(err, errs.ErrTestPushFailed) {
		t.Errorf("rejected: got %v, want ErrTestPushFailed", err)
	}
	after, _ := svc.Analytics(ctx, id)
	before.NotifiedAt, after.NotifiedAt = nil, nil // pointers; the counts are what matter
	if *after != *before {
		t.Errorf("test pushes changed the analytics:\n before %+v\n after  %+v", *before, *after)
	}

	// An admin previews any story — a draft too — without being counted.
	draft, err := svc.Create(ctx, model.SaveRequest{Title: "مسودة", BodyHTML: "<p>نص</p>"}, admin)
	if err != nil {
		t.Fatal(err)
	}
	if p, err := svc.Preview(ctx, uuid.MustParse(draft.PublicID)); err != nil || p.Title != "مسودة" {
		t.Errorf("preview draft = %+v, %v", p, err)
	}

	again, err := svc.Publish(ctx, id, true)
	if err != nil || again.NotifiedCount != 0 || !again.AlreadyNotified {
		t.Errorf("second publish = %+v, %v", again, err)
	}
	if got, _ := svc.AdminGet(ctx, id); got.Viewers != 3 || got.NotificationTitle != "🐔 عنوان الإشعار" {
		t.Errorf("admin view: viewers %d, notification %q", got.Viewers, got.NotificationTitle)
	}
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if ok() {
			return
		}
		sleep()
	}
	t.Fatal("timed out waiting for the push fan-out")
}

func sleep() { time.Sleep(50 * time.Millisecond) }
