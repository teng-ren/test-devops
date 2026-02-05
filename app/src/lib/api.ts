import type { User, Chat, ChatMessage, ChatWithMessages, ChatListResponse, ModelsResponse } from "./types"

const API_URL = import.meta.env.VITE_API_URL

// Helper to get auth headers
const getAuthHeaders = () => {
  const token = localStorage.getItem('token')
  return {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${token}`
  }
}

export const adminApi = {
  getUsers: async (): Promise<User[]> => {
    const token = localStorage.getItem('token')
    const response = await fetch(`${API_URL}/admin/users`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
      }
    })

    if (!response.ok) {
      throw new Error('Failed to fetch users')
    }

    return response.json()
  },

  createUser: async (userData: { username: string; password: string; role: string }): Promise<User> => {
    const token = localStorage.getItem('token')
    const response = await fetch(`${API_URL}/admin/users`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
      },
      body: JSON.stringify(userData)
    })

    if (!response.ok) {
      throw new Error('Failed to create user')
    }

    return response.json()
  },

  editUser: async (userId: number, role: string): Promise<User> => {
    const token = localStorage.getItem('token')
    const response = await fetch(`${API_URL}/admin/users/${userId}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
      },
      body: JSON.stringify({ role })
    })

    if (!response.ok) {
      throw new Error('Failed to update user')
    }

    return response.json()
  },

  deleteUser: async (userId: number): Promise<void> => {
    const token = localStorage.getItem('token')
    const response = await fetch(`${API_URL}/admin/users/${userId}`, {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${token}`
      }
    })

    if (!response.ok) {
      throw new Error('Failed to delete user')
    }
  }
}

export const promptManagerApi = {
  // Get available models
  getModels: async (): Promise<string[]> => {
    const response = await fetch(`${API_URL}/models`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json'
      }
    })

    if (!response.ok) {
      throw new Error('Failed to fetch models')
    }

    const data: ModelsResponse = await response.json()
    return data.data.models
  },

  // Create a new chat
  createChat: async (model: string): Promise<Chat> => {
    const response = await fetch(`${API_URL}/chats`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ model })
    })

    if (!response.ok) {
      throw new Error('Failed to create chat')
    }

    const data = await response.json()
    return data.data.chat
  },

  // List chats with pagination
  getChats: async (page: number = 1, pageSize: number = 20): Promise<ChatListResponse> => {
    const response = await fetch(`${API_URL}/chats?page=${page}&page_size=${pageSize}`, {
      method: 'GET',
      headers: getAuthHeaders()
    })

    if (!response.ok) {
      throw new Error('Failed to fetch chats')
    }

    return response.json()
  },

  // Get a single chat with messages
  getChat: async (chatId: string): Promise<ChatWithMessages> => {
    const response = await fetch(`${API_URL}/chats/${chatId}`, {
      method: 'GET',
      headers: getAuthHeaders()
    })

    if (!response.ok) {
      throw new Error('Failed to fetch chat')
    }

    return response.json()
  },

  // Delete a chat
  deleteChat: async (chatId: string): Promise<void> => {
    const response = await fetch(`${API_URL}/chats/${chatId}`, {
      method: 'DELETE',
      headers: getAuthHeaders()
    })

    if (!response.ok) {
      throw new Error('Failed to delete chat')
    }
  },

  // Send a message to a chat
  sendMessage: async (chatId: string, content: string): Promise<ChatMessage> => {
    const response = await fetch(`${API_URL}/chats/${chatId}/messages`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ content })
    })

    if (!response.ok) {
      throw new Error('Failed to send message')
    }

    const data = await response.json()
    return data.data.message
  }
}