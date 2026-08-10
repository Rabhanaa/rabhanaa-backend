-- +goose Up

-- Only 4 of Egypt's 27 governorates were seeded, so most merchants had no
-- accurate region to pick. Ids 1-4 already exist and are referenced by users
-- and posts, so the rest continue from 5.
INSERT INTO regions (id, name_ar, name_en, is_active) VALUES
    (5,  'الشرقية',      'Sharqia',        true),
    (6,  'الدقهلية',     'Dakahlia',       true),
    (7,  'البحيرة',      'Beheira',        true),
    (8,  'المنيا',       'Minya',          true),
    (9,  'سوهاج',        'Sohag',          true),
    (10, 'أسيوط',        'Asyut',          true),
    (11, 'الغربية',      'Gharbia',        true),
    (12, 'كفر الشيخ',    'Kafr El Sheikh', true),
    (13, 'الفيوم',       'Faiyum',         true),
    (14, 'المنوفية',     'Monufia',        true),
    (15, 'قنا',          'Qena',           true),
    (16, 'بني سويف',     'Beni Suef',      true),
    (17, 'أسوان',        'Aswan',          true),
    (18, 'دمياط',        'Damietta',       true),
    (19, 'الإسماعيلية',  'Ismailia',       true),
    (20, 'الأقصر',       'Luxor',          true),
    (21, 'بورسعيد',      'Port Said',      true),
    (22, 'السويس',       'Suez',           true),
    (23, 'مطروح',        'Matrouh',        true),
    (24, 'شمال سيناء',   'North Sinai',    true),
    (25, 'جنوب سيناء',   'South Sinai',    true),
    (26, 'الوادي الجديد','New Valley',     true),
    (27, 'البحر الأحمر', 'Red Sea',        true)
ON CONFLICT (id) DO NOTHING;

SELECT setval(pg_get_serial_sequence('regions', 'id'), (SELECT MAX(id) FROM regions));

-- +goose Down

-- Same guard as the merchant-type rollback: leave behind any governorate that
-- users or posts already reference rather than aborting or orphaning rows.
DELETE FROM regions
WHERE id BETWEEN 5 AND 27
  AND id NOT IN (SELECT region_id FROM users WHERE region_id IS NOT NULL)
  AND id NOT IN (SELECT region_id FROM sell_auctions)
  AND id NOT IN (SELECT region_id FROM buy_requests);

SELECT setval(pg_get_serial_sequence('regions', 'id'), (SELECT MAX(id) FROM regions));
