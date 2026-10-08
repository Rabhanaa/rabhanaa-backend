-- +goose Up

-- Photos found in the source article, already copied to our storage. Kept so
-- the editor can offer them again (as the cover, or inside the body) when a
-- draft is reopened, without fetching the source page a second time.
-- Shape: [{"url": "...", "alt": "...", "width": 1200, "height": 800}]
ALTER TABLE news ADD COLUMN source_images JSONB NOT NULL DEFAULT '[]'::jsonb;

-- +goose Down

ALTER TABLE news DROP COLUMN IF EXISTS source_images;
