"use client"

import { useState, useEffect } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Alert, AlertDescription } from "@/components/ui/alert"
import {
  Bot,
  Edit,
  MoreHorizontal,
  Plus,
  Search,
  Trash2,
  Users,
  Shield,
} from "lucide-react"
import { cn } from "@/lib/utils"
import type { User } from "@/lib/types"
import { adminApi } from "@/lib/api"
import { Link, useNavigate } from "react-router-dom"

export default function AdminDashboard() {
  const navigate = useNavigate()
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [isUsersLoading, setIsUsersLoading] = useState(false)
  const [authError, setAuthError] = useState("")
  const [users, setUsers] = useState<User[]>([])
  const [searchQuery, setSearchQuery] = useState("")
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false)
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false)
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false)
  const [selectedUser, setSelectedUser] = useState<User | null>(null)
  const [apiError, setApiError] = useState("")
  const [formData, setFormData] = useState({
    username: "",
    password: "",
    role: "user" as "user" | "admin" | "premium",
  })

  useEffect(() => {
    const validateToken = async () => {
      const token = localStorage.getItem('token')
      
      if (!token) {
        setAuthError("Please login to access admin dashboard")
        setTimeout(() => navigate('/'), 2000)
        setIsLoading(false)
        return
      }

      try {
        const api_gateway = import.meta.env.VITE_API_URL;
        const response = await fetch(`${api_gateway}/auth/validate`, {
          method: "POST",
          headers: {
            'Content-Type': 'application/json'
          },
          body: JSON.stringify({ token })
        })

        const resp = await response.json()

        if (resp.valid === true) {
          if (resp.role === 'admin') {
            setIsAuthenticated(true)
          } else {
            setAuthError("Access denied. Admin privileges required")
            setTimeout(() => navigate('/user'), 2000)
            setIsLoading(false)
          }
        } else {
          setAuthError("Session expired. Please login again")
          setTimeout(() => navigate('/'), 2000)
          setIsLoading(false)
        }
      } catch {
        setAuthError("Authentication error. Please login again")
        setTimeout(() => navigate('/'), 2000)
      } finally {
        setIsLoading(false)
      }
    }

    validateToken()
  }, [navigate])

  useEffect(() => {
    if (isAuthenticated) {
      fetchUsers()
    }
  }, [isAuthenticated])

  const fetchUsers = async () => {
    setIsUsersLoading(true)
    try {
      const usersData = await adminApi.getUsers()
      setUsers(usersData)
    } catch (error) {
      setApiError("Failed to fetch users")
      alert("Failed to fetch users")
    } finally {
      setIsUsersLoading(false)
    }
  }

  const filteredUsers = users.filter(
    (user) =>
      user.username.toLowerCase().includes(searchQuery.toLowerCase()),
  )

  const handleCreateUser = async () => {
    try {
      const newUser = await adminApi.createUser(formData)
      setUsers((prev) => [...prev, newUser])
      setIsCreateDialogOpen(false)
      setFormData({ username: "", password: "", role: "user" })
      setApiError("")
    } catch (error) {
      setApiError("Failed to create user")
      alert("Failed to create user")
    }
  }

  const handleEditUser = async () => {
    if (!selectedUser) return
    try {
      const updatedUser = await adminApi.editUser(selectedUser.id, formData.role)
      setUsers((prev) => prev.map((user) => (user.id === selectedUser.id ? updatedUser : user)))
      setIsEditDialogOpen(false)
      setSelectedUser(null)
      setApiError("")
    } catch (error) {
      setApiError("Failed to update user")
      alert("Failed to update user")
    }
  }

  const handleDeleteUser = async () => {
    if (!selectedUser) return
    try {
      await adminApi.deleteUser(selectedUser.id)
      setUsers((prev) => prev.filter((user) => user.id !== selectedUser.id))
      setIsDeleteDialogOpen(false)
      setSelectedUser(null)
      setApiError("")
    } catch (error) {
      setApiError("Failed to delete user")
      alert("Failed to delete user")
    }
  }

  const openEditDialog = (user: User) => {
    setSelectedUser(user)
    setFormData({
      username: user.username,
      password: "",
      role: user.role,
    })
    setIsEditDialogOpen(true)
  }

  const openDeleteDialog = (user: User) => {
    setSelectedUser(user)
    setIsDeleteDialogOpen(true)
  }

  const formatDate = (dateString: string) => {
    const date = new Date(dateString)
    return date.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric'
    })
  }

  const stats = [
    {
      title: "Total Users",
      value: users.length,
      icon: Users,
      change: "+12%",
    }
  ]

  return (
    <div className="min-h-screen min-w-screen bg-background">
      {isLoading && (
        <div className="flex items-center justify-center min-h-screen">
          <div className="text-center">
            <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary mx-auto mb-4"></div>
            <p className="text-muted-foreground">Validating authentication...</p>
          </div>
        </div>
      )}

      {authError && !isLoading && (
        <div className="flex items-center justify-center min-h-screen p-4">
          <Alert variant="destructive" className="max-w-md">
            <AlertDescription>{authError}</AlertDescription>
          </Alert>
        </div>
      )}

      {!isLoading && !authError && isAuthenticated && (
        <>
      <header className="sticky top-0 z-50 border-b border-border bg-card/95 backdrop-blur supports-[backdrop-filter]:bg-card/60">
        <div className="flex h-16 items-center justify-between px-6">
          <div className="flex items-center gap-4">
            <Link to="/" className="flex items-center gap-2">
              <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-r from-violet-500 to-purple-600 hover:from-violet-600 hover:to-purple-700">
                <Bot className="h-5 w-5 text-primary-foreground" /> 
              </div>
              <span className="text-xl font-semibold text-black">AIDC</span>
            </Link>
            <Badge variant="outline" className="border-primary/50 text-primary">
              <Shield className="mr-1 h-3 w-3" />
              Admin
            </Badge>
          </div>
          <nav className="flex items-center gap-2">
            <Link to="/user">
              <Button variant="outline" className="text-white hover:bg-white/20 bg-gradient-to-r from-violet-500 to-purple-600 hover:from-violet-600 hover:to-purple-700">
                User Dashboard
              </Button>
            </Link>
          </nav>
        </div>
      </header>

      <main className="p-6">
        <div className="mx-auto max-w-7xl space-y-6">
          {apiError && (
            <Alert variant="destructive">
              <AlertDescription>{apiError}</AlertDescription>
            </Alert>
          )}

          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-3xl font-bold text-foreground">User Management</h1>
              <p className="text-muted-foreground">Manage users, permissions, and access controls</p>
            </div>
            <Button
              variant="outline"
              onClick={() => {
                setFormData({ username: "", password: "", role: "user" })
                setIsCreateDialogOpen(true)
              }}
              className="bg-primary text-primary-foreground hover:bg-primary/90 bg-gradient-to-r from-violet-500 to-purple-600 hover:from-violet-600 hover:to-purple-700"
            >
              <Plus className="mr-2 h-4 w-4" />
              <span className="text-white hover:text-black bg-gradient-to-r from-violet-500 to-purple-600 hover:from-violet-600 hover:to-purple-700">Add User</span>
            </Button>
          </div>

          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
            {stats.map((stat, i) => (
              <Card key={i} className="border-border bg-card">
                <CardHeader className="flex flex-row items-center justify-between pb-2">
                  <CardTitle className="text-sm font-medium text-muted-foreground">{stat.title}</CardTitle>
                  <stat.icon className="h-4 w-4 text-primary" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold text-card-foreground">{stat.value}</div>
                  <p className="text-xs text-primary">{stat.change} from last month</p>
                </CardContent>
              </Card>
            ))}
          </div>

          {/* Users Table */}
          <Card className="border-border bg-card">
            <CardHeader>
              <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <CardTitle className="text-card-foreground">All Users</CardTitle>
                <div className="relative w-full sm:w-72">
                  <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    placeholder="Search users..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    className="pl-9 border-border bg-background text-foreground placeholder:text-muted-foreground"
                  />
                </div>
              </div>
            </CardHeader>
            <CardContent>
              {isUsersLoading ? (
                <div className="flex items-center justify-center py-12">
                  <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
                </div>
              ) : (
                <>
                  <Table>
                    <TableHeader>
                      <TableRow className="border-border hover:bg-transparent">
                        <TableHead className="text-muted-foreground">Username</TableHead>
                        <TableHead className="text-muted-foreground">Role</TableHead>
                        <TableHead className="text-muted-foreground">Created At</TableHead>
                        <TableHead className="text-right text-muted-foreground">Actions</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {filteredUsers.map((user) => (
                        <TableRow key={user.id} className="border-border">
                          <TableCell className="font-medium text-foreground">{user.username}</TableCell>
                          <TableCell>
                            <Badge
                              variant={
                                user.role === "admin" ? "default" :
                                user.role === "premium" ? "secondary" : "outline"
                              }
                              className={cn(user.role === "admin" && "bg-primary text-primary-foreground")}
                            >
                              {user.role}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-muted-foreground">{formatDate(user.created_at)}</TableCell>
                          <TableCell className="text-right">
                            <DropdownMenu>
                              <DropdownMenuTrigger asChild>
                                <Button variant="ghost" size="icon" className="text-white bg-gradient-to-r from-violet-500 to-purple-600 hover:from-violet-600 hover:to-purple-700">
                                  <MoreHorizontal className="h-4 w-4" />
                                </Button>
                              </DropdownMenuTrigger>
                              <DropdownMenuContent align="end" className="bg-popover">
                                <DropdownMenuItem onClick={() => openEditDialog(user)} className="cursor-pointer">
                                  <Edit className="mr-2 h-4 w-4" />
                                  Edit
                                </DropdownMenuItem>
                                <DropdownMenuItem
                                  onClick={() => openDeleteDialog(user)}
                                  className="cursor-pointer text-destructive focus:text-destructive"
                                >
                                  <Trash2 className="mr-2 h-4 w-4" />
                                  Delete
                                </DropdownMenuItem>
                              </DropdownMenuContent>
                            </DropdownMenu>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                  {filteredUsers.length === 0 && !isUsersLoading && (
                    <div className="py-12 text-center">
                      <Users className="mx-auto mb-4 h-12 w-12 text-muted-foreground/50" />
                      <p className="text-muted-foreground">No users found</p>
                    </div>
                  )}
                </>
              )}
            </CardContent>
          </Card>
        </div>
      </main>

      <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
        <DialogContent className="bg-card border-border" showCloseButton={false}>
          <DialogHeader>
            <DialogTitle className="text-card-foreground">Create New User</DialogTitle>
            <DialogDescription className="text-muted-foreground">
              Add a new user to the platform with username, password, and role.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="username" className="text-foreground">
                Username
              </Label>
              <Input
                id="username"
                value={formData.username}
                onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                placeholder="Enter username"
                className="border-border bg-background text-foreground placeholder:text-muted-foreground"
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="password" className="text-foreground">
                Password
              </Label>
              <Input
                id="password"
                type="password"
                value={formData.password}
                onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                placeholder="Enter password"
                className="border-border bg-background text-foreground placeholder:text-muted-foreground"
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="role" className="text-foreground">
                Role
              </Label>
              <Select
                value={formData.role}
                onValueChange={(value: "user" | "admin" | "premium") => setFormData({ ...formData, role: value })}
              >
                <SelectTrigger className="text-white border-border bg-background bg-gradient-to-r from-violet-500 to-purple-600 hover:from-violet-600 hover:to-purple-700">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className="bg-popover">
                  <SelectItem value="user">User</SelectItem>
                  <SelectItem value="admin">Admin</SelectItem>
                  <SelectItem value="premium">Premium</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setIsCreateDialogOpen(false)}
              className="border-border text-white bg-gradient-to-r from-violet-500 to-purple-600 hover:from-violet-600 hover:to-purple-700"
            >
              Cancel
            </Button>
            <Button
                onClick={handleCreateUser} 
                variant="outline"
                className="text-primary-foreground bg-gradient-to-r from-violet-500 to-purple-600 hover:from-violet-600 hover:to-purple-700">
              Create User
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={isEditDialogOpen} onOpenChange={setIsEditDialogOpen}>
        <DialogContent className="bg-card border-border" showCloseButton={false}>
          <DialogHeader>
            <DialogTitle className="text-card-foreground">Edit User Role</DialogTitle>
            <DialogDescription className="text-muted-foreground">
              Update user role. Only the role can be changed.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label className="text-foreground">
                Username
              </Label>
              <Input
                value={formData.username}
                disabled
                className="border-border bg-muted text-muted-foreground"
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="edit-role" className="text-foreground">
                Role
              </Label>
              <Select
                value={formData.role}
                onValueChange={(value: "user" | "admin" | "premium") => setFormData({ ...formData, role: value })}
              >
                <SelectTrigger className="border-border bg-gradient-to-r from-violet-500 to-purple-600 hover:from-violet-600 hover:to-purple-700 text-white">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent className="bg-popover">
                  <SelectItem value="user">User</SelectItem>
                  <SelectItem value="admin">Admin</SelectItem>
                  <SelectItem value="premium">Premium</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setIsEditDialogOpen(false)}
              className="border-border bg-gradient-to-r from-violet-500 to-purple-600 hover:from-violet-600 hover:to-purple-700 text-white"
            >
              Cancel
            </Button>
            <Button onClick={handleEditUser} className="bg-gradient-to-r from-violet-500 to-purple-600 hover:from-violet-600 hover:to-purple-700" variant="outline">
              <span className="text-white">Save Changes</span>
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete User Dialog */}
      <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
        <DialogContent className="bg-card border-border">
          <DialogHeader>
            <DialogTitle className="text-card-foreground">Delete User</DialogTitle>
            <DialogDescription className="text-muted-foreground">
              Are you sure you want to delete user "{selectedUser?.username}"? This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setIsDeleteDialogOpen(false)}
              className="border-border text-foreground"
            >
              Cancel
            </Button>
            <Button 
              onClick={handleDeleteUser} 
              variant="destructive"
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              <span className="text-white">Delete User</span>
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
        </>
      )}
    </div>
  )
}
