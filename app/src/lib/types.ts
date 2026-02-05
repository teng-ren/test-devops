export interface User {
  id: number
  username: string
  role: "user" | "admin" | "premium"
  created_at: string
}

export interface Message {
  id: string
  role: "user" | "assistant"
  content: string
  timestamp: Date
  model?: string
}

export interface AIModel {
  id: string
  name: string
  provider: string
  description: string
  icon: string
  maxTokens: number
}

export interface Conversation {
  id: string
  title: string
  messages: Message[]
  model: string
  createdAt: Date
  updatedAt: Date
}

// Prompt Manager types (matching backend API)
export type MessageRole = "user" | "assistant" | "system"
export type MessageStatus = "pending" | "streaming" | "completed" | "failed"

export interface Chat {
  id: string
  user_id: number
  title: string
  model: string
  created_at: string
  updated_at: string
}

export interface ChatMessage {
  id: string
  chat_id: string
  role: MessageRole
  content: string
  status: MessageStatus
  error_message?: string
  tokens_used?: number
  created_at: string
  updated_at: string
}

export interface ChatWithMessages {
  chat: Chat
  messages: ChatMessage[]
}

export interface ChatListResponse {
  chats: Chat[]
  page: number
  page_size: number
  total_count: number
}

export interface ModelsResponse {
  data: {
    models: string[]
  }
}
