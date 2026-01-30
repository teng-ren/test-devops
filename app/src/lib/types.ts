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
