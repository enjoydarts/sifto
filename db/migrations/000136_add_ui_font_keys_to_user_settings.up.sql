ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS ui_font_sans_key TEXT NOT NULL DEFAULT 'sawarabi-gothic',
  ADD COLUMN IF NOT EXISTS ui_font_serif_key TEXT NOT NULL DEFAULT 'sawarabi-mincho';
