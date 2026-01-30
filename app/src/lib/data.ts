import type { User, AIModel, Conversation } from "./types"

export const aiModels: AIModel[] = [
  {
    id: "gpt-4",
    name: "GPT-4 Turbo",
    provider: "OpenAI",
    description: "Most capable GPT-4 model for complex tasks",
    icon: "🟢",
    maxTokens: 128000,
  },
  {
    id: "claude-3",
    name: "Claude 3 Opus",
    provider: "Anthropic",
    description: "Excellent for nuanced analysis and creative writing",
    icon: "🟠",
    maxTokens: 200000,
  },
  {
    id: "gemini-pro",
    name: "Gemini Pro",
    provider: "Google",
    description: "Multimodal AI with strong reasoning capabilities",
    icon: "🔵",
    maxTokens: 32000,
  },
  {
    id: "llama-3",
    name: "Llama 3 70B",
    provider: "Meta",
    description: "Open-source powerhouse for diverse applications",
    icon: "🟣",
    maxTokens: 8192,
  },
]

export const initialUsers: User[] = [
  {
    id: "1",
    name: "Alex Johnson",
    email: "alex@company.com",
    role: "admin",
    status: "active",
    createdAt: "2024-01-15",
    lastActive: "2024-01-20",
  },
  {
    id: "2",
    name: "Sarah Chen",
    email: "sarah@company.com",
    role: "user",
    status: "active",
    createdAt: "2024-01-10",
    lastActive: "2024-01-19",
  },
  {
    id: "3",
    name: "Mike Williams",
    email: "mike@company.com",
    role: "user",
    status: "inactive",
    createdAt: "2024-01-05",
    lastActive: "2024-01-12",
  },
  {
    id: "4",
    name: "Emily Davis",
    email: "emily@company.com",
    role: "user",
    status: "active",
    createdAt: "2024-01-08",
    lastActive: "2024-01-20",
  },
  {
    id: "5",
    name: "James Brown",
    email: "james@company.com",
    role: "user",
    status: "suspended",
    createdAt: "2023-12-20",
    lastActive: "2024-01-02",
  },
]

export const sampleConversations: Conversation[] = [
  {
    id: "1",
    title: "Code Review Help",
    model: "gpt-4",
    messages: [],
    createdAt: new Date("2024-01-19"),
    updatedAt: new Date("2024-01-19"),
  },
  {
    id: "2",
    title: "Marketing Strategy",
    model: "claude-3",
    messages: [],
    createdAt: new Date("2024-01-18"),
    updatedAt: new Date("2024-01-18"),
  },
  {
    id: "3",
    title: "Data Analysis",
    model: "gemini-pro",
    messages: [],
    createdAt: new Date("2024-01-17"),
    updatedAt: new Date("2024-01-17"),
  },
]
