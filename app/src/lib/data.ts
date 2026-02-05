import type { AIModel } from "./types"

export const aiModels: AIModel[] = [
  {
    id: "gemma3",
    name: "Gemma 3",
    provider: "Google",
    description: "Lightweight, state-of-the-art open model from Google",
    icon: "🔵",
    maxTokens: 8192,
  },
  {
    id: "qwen3",
    name: "Qwen 3",
    provider: "Alibaba",
    description: "Advanced multilingual model with strong reasoning",
    icon: "🟣",
    maxTokens: 8192,
  },
]

// Helper to get model info by id
export const getModelById = (id: string): AIModel | undefined => {
  return aiModels.find(model => model.id === id)
}

// Helper to create AIModel from model id string
export const createModelFromId = (id: string): AIModel => {
  const existing = getModelById(id)
  if (existing) return existing

  // Fallback for unknown models
  return {
    id,
    name: id.charAt(0).toUpperCase() + id.slice(1),
    provider: "Unknown",
    description: "AI model",
    icon: "🤖",
    maxTokens: 4096
  }
}

