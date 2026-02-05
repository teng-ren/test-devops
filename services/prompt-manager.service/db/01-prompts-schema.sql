CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Chats table (like ChatGPT conversations)
CREATE TABLE chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id INTEGER NOT NULL,
    title VARCHAR(255),  -- Auto-generated from first message or user-defined
    model VARCHAR(50) NOT NULL DEFAULT 'gemma3',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TYPE message_role AS ENUM ('user', 'assistant', 'system');
CREATE TYPE message_status AS ENUM ('pending', 'streaming', 'completed', 'failed');

-- Messages table (individual messages in a chat)
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    role message_role NOT NULL,
    content TEXT NOT NULL,
    status message_status NOT NULL DEFAULT 'completed',
    error_message TEXT,
    tokens_used INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_chats_user_id ON chats(user_id);
CREATE INDEX idx_chats_updated ON chats(user_id, updated_at DESC);
CREATE INDEX idx_messages_chat_id ON messages(chat_id);
CREATE INDEX idx_messages_created ON messages(chat_id, created_at);
