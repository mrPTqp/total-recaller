-- +goose Up
-- Создаем таблицу пользователей
CREATE TABLE
    users (
        id SERIAL PRIMARY KEY,
        telegram_id BIGINT UNIQUE NOT NULL,
        username VARCHAR(255),
        first_name VARCHAR(255),
        last_name VARCHAR(255),
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );

-- Создаем индекс для таблицы пользователей
CREATE INDEX idx_telegram_id ON users (telegram_id);

-- Создаем таблицу встреч
CREATE TABLE
    meetings (
        id SERIAL PRIMARY KEY,
        telegram_id BIGINT NOT NULL REFERENCES users (telegram_id) ON DELETE CASCADE,
        audio_url TEXT,
        full_text TEXT,
        summary TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        -- Полнотекстовый индекс
        full_text_tsvector TSVECTOR GENERATED ALWAYS AS (to_tsvector ('russian', COALESCE(full_text, ''))) STORED
    );

-- Создаем индексы для таблицы встреч
CREATE INDEX idx_telegram_id_created ON meetings (telegram_id, created_at);
CREATE INDEX idx_full_text_gin ON meetings USING GIN (full_text_tsvector);

-- +goose Down
DROP INDEX IF EXISTS idx_full_text_gin;
DROP INDEX IF EXISTS idx_telegram_id_created;
DROP INDEX IF EXISTS idx_telegram_id;
DROP TABLE IF EXISTS meetings;
DROP TABLE IF EXISTS users;
