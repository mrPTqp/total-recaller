-- +goose Up
-- Добавляем поле embedding для хранения векторных представлений текста
ALTER TABLE meetings ADD COLUMN IF NOT EXISTS embedding vector(1024);

-- Создаем индекс для векторного поиска (если установлено расширение pgvector)
CREATE INDEX IF NOT EXISTS idx_meetings_embedding ON meetings USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);

-- +goose Down
-- Удаляем поле embedding
ALTER TABLE meetings DROP COLUMN IF EXISTS embedding;

-- Удаляем индекс для векторного поиска
DROP INDEX IF EXISTS idx_meetings_embedding;
