import type { User } from "./types"

const API_URL = import.meta.env.VITE_API_URL

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