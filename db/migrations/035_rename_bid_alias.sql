-- +goose Up
-- Bidders are shown under an anonymised label generated at bid time and stored
-- on the row. Renaming the generator only affects new bids, so rewrite the
-- existing ones to match ("مزاد1" -> "صفقة1").
-- supplier_fake_name ("مورد1") is deliberately untouched — "مورد" means
-- supplier and is unrelated to the auction/deal rename.
UPDATE sell_bids
SET bidder_fake_name = REPLACE(bidder_fake_name, 'مزاد', 'صفقة')
WHERE bidder_fake_name LIKE 'مزاد%';

-- Notification title/body are also snapshotted at send time, so already
-- delivered notifications would keep showing the old wording. Only the display
-- text is rewritten; event_type and the JSON data payload drive navigation and
-- must not change.
UPDATE notifications
SET title = REPLACE(REPLACE(title, 'انتهى المزاد', 'انتهت الصفقة'), 'مزاد جديد', 'صفقة جديدة'),
    body  = REPLACE(
              REPLACE(
                REPLACE(
                  REPLACE(body, 'انتهى المزاد', 'انتهت الصفقة'),
                'على مزادك', 'على صفقتك'),
              'إلغاء المزاد', 'إلغاء الصفقة'),
            'على المزاد', 'على الصفقة')
WHERE title LIKE '%مزاد%' OR body LIKE '%مزاد%';

-- +goose Down
UPDATE sell_bids
SET bidder_fake_name = REPLACE(bidder_fake_name, 'صفقة', 'مزاد')
WHERE bidder_fake_name LIKE 'صفقة%';

UPDATE notifications
SET title = REPLACE(REPLACE(title, 'انتهت الصفقة', 'انتهى المزاد'), 'صفقة جديدة', 'مزاد جديد'),
    body  = REPLACE(
              REPLACE(
                REPLACE(
                  REPLACE(body, 'انتهت الصفقة', 'انتهى المزاد'),
                'على صفقتك', 'على مزادك'),
              'إلغاء الصفقة', 'إلغاء المزاد'),
            'على الصفقة', 'على المزاد')
WHERE title LIKE '%صفقة%' OR body LIKE '%صفقة%';
