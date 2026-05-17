-- +goose Up

-- Insert Jobs
INSERT INTO jobs (id, key, name_ar, name_en, is_active, created_at) VALUES
(1, 'trader', 'تاجر', 'Trader', true, NOW()),
(2, 'importer', 'مستورد', 'Importer', true, NOW()),
(3, 'processor', 'مصنع', 'Processor', true, NOW()),
(4, 'company', 'شركة', 'Company', true, NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert Interests
INSERT INTO interests (id, name_ar, name_en, is_active, created_at) VALUES
(1, 'خضروات', 'Vegetables', true, NOW()),
(2, 'فاكهة', 'Fruits', true, NOW()),
(3, 'حبوب', 'Grains', true, NOW()),
(4, 'لحوم', 'Meat', true, NOW()),
(5, 'دواجن', 'Poultry', true, NOW()),
(6, 'أسماك', 'Fish', true, NOW()),
(7, 'ألبان', 'Dairy', true, NOW()),
(8, 'توابل', 'Spices', true, NOW()),
(9, 'زيوت', 'Oils', true, NOW()),
(10, 'تمور', 'Dates', true, NOW()),
(11, 'مكسرات', 'Nuts', true, NOW()),
(12, 'عصائر', 'Juices', true, NOW())
ON CONFLICT (id) DO NOTHING;

-- Insert Regions
INSERT INTO regions (id, name_ar, name_en, is_active, created_at) VALUES
(1, 'القاهرة', 'Cairo', true, NOW()),
(2, 'الإسكندرية', 'Alexandria', true, NOW()),
(3, 'الجيزة', 'Giza', true, NOW()),
(4, 'القليوبية', 'Qalyubia', true, NOW())
ON CONFLICT (id) DO NOTHING;

-- +goose Down

DELETE FROM regions WHERE id IN (1, 2, 3, 4);
DELETE FROM interests WHERE id IN (1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12);
DELETE FROM jobs WHERE id IN (1, 2, 3);
