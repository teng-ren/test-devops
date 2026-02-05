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
  Loader2,
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
    } catch {
      setApiError("Failed to fetch users")
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
    } catch {
      setApiError("Failed to create user")
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
    } catch {
      setApiError("Failed to update user")
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
    } catch {
      setApiError("Failed to delete user")
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

  return (
    <div className="min-h-screen w-full bg-background">
      {isLoading && (
        <div className="flex min-h-screen items-center justify-center">
          <div className="text-center">
            <Loader2 className="mx-auto mb-4 h-10 w-10 animate-spin text-primary" />
            <p className="text-muted-foreground">Validating authentication...</p>
          </div>
        </div>
      )}

      {authError && !isLoading && (
        <div className="flex min-h-screen items-center justify-center p-4">
          <Alert variant="destructive" className="max-w-md">
            <AlertDescription>{authError}</AlertDescription>
          </Alert>
        </div>
      )}

      {!isLoading && !authError && isAuthenticated && (
        <>
          {/* Header */}
          <header className="sticky top-0 z-50 border-b border-border bg-card">
            <div className="flex h-14 items-center justify-between px-4">
              <div className="flex items-center gap-3">
                <Link to="/" className="flex items-center gap-2">
                  <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary">
                    <Bot className="h-4 w-4 text-primary-foreground" />
                  </div>
                  <span className="font-semibold text-foreground">AIDC</span>
                </Link>
                <Badge variant="outline" className="border-primary/50 text-primary">
                  <Shield className="mr-1 h-3 w-3" />
                  Admin
                </Badge>
              </div>
              <Link to="/user">
                <Button variant="outline" size="sm">
                  User Dashboard
                </Button>
              </Link>
            </div>
          </header>

          {/* Main Content */}
          <main className="p-6">
            <div className="mx-auto max-w-6xl space-y-6">
              {apiError && (
                <Alert variant="destructive">
                  <AlertDescription>{apiError}</AlertDescription>
                </Alert>
              )}

              {/* Page Header */}
              <div className="flex items-center justify-between">
                <div>
                  <h1 className="text-2xl font-bold text-foreground">User Management</h1>
                  <p className="text-sm text-muted-foreground">
                    Manage users, permissions, and access controls
                  </p>
                </div>
                <Button
                  onClick={() => {
                    setFormData({ username: "", password: "", role: "user" })
                    setIsCreateDialogOpen(true)
                  }}
                >
                  <Plus className="mr-2 h-4 w-4" />
                  Add User
                </Button>
              </div>

              {/* Stats Card */}
              <Card>
                <CardHeader className="flex flex-row items-center justify-between pb-2">
                  <CardTitle className="text-sm font-medium text-muted-foreground">
                    Total Users
                  </CardTitle>
                  <Users className="h-4 w-4 text-primary" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold text-foreground">{users.length}</div>
                </CardContent>
              </Card>

              {/* Users Table */}
              <Card>
                <CardHeader>
                  <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                    <CardTitle>All Users</CardTitle>
                    <div className="relative w-full sm:w-64">
                      <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                      <Input
                        placeholder="Search users..."
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="pl-9"
                      />
                    </div>
                  </div>
                </CardHeader>
                <CardContent>
                  {isUsersLoading ? (
                    <div className="flex items-center justify-center py-12">
                      <Loader2 className="h-8 w-8 animate-spin text-primary" />
                    </div>
                  ) : (
                    <>
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>Username</TableHead>
                            <TableHead>Role</TableHead>
                            <TableHead>Created At</TableHead>
                            <TableHead className="text-right">Actions</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {filteredUsers.map((user) => (
                            <TableRow key={user.id}>
                              <TableCell className="font-medium">{user.username}</TableCell>
                              <TableCell>
                                <Badge
                                  variant={user.role === "admin" ? "default" : "secondary"}
                                  className={cn(user.role === "admin" && "bg-primary")}
                                >
                                  {user.role}
                                </Badge>
                              </TableCell>
                              <TableCell className="text-muted-foreground">
                                {formatDate(user.created_at)}
                              </TableCell>
                              <TableCell className="text-right">
                                <DropdownMenu>
                                  <DropdownMenuTrigger asChild>
                                    <Button variant="ghost" size="icon" className="h-8 w-8">
                                      <MoreHorizontal className="h-4 w-4" />
                                    </Button>
                                  </DropdownMenuTrigger>
                                  <DropdownMenuContent align="end">
                                    <DropdownMenuItem
                                      onClick={() => openEditDialog(user)}
                                      className="cursor-pointer"
                                    >
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
                      {filteredUsers.length === 0 && (
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

          {/* Create User Dialog */}
          <Dialog open={isCreateDialogOpen} onOpenChange={setIsCreateDialogOpen}>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Create New User</DialogTitle>
                <DialogDescription>
                  Add a new user to the platform with username, password, and role.
                </DialogDescription>
              </DialogHeader>
              <div className="grid gap-4 py-4">
                <div className="grid gap-2">
                  <Label htmlFor="username">Username</Label>
                  <Input
                    id="username"
                    value={formData.username}
                    onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                    placeholder="Enter username"
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="password">Password</Label>
                  <Input
                    id="password"
                    type="password"
                    value={formData.password}
                    onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                    placeholder="Enter password"
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="role">Role</Label>
                  <Select
                    value={formData.role}
                    onValueChange={(value: "user" | "admin" | "premium") =>
                      setFormData({ ...formData, role: value })
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="user">User</SelectItem>
                      <SelectItem value="admin">Admin</SelectItem>
                      <SelectItem value="premium">Premium</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setIsCreateDialogOpen(false)}>
                  Cancel
                </Button>
                <Button onClick={handleCreateUser}>Create User</Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>

          {/* Edit User Dialog */}
          <Dialog open={isEditDialogOpen} onOpenChange={setIsEditDialogOpen}>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Edit User Role</DialogTitle>
                <DialogDescription>
                  Update user role. Only the role can be changed.
                </DialogDescription>
              </DialogHeader>
              <div className="grid gap-4 py-4">
                <div className="grid gap-2">
                  <Label>Username</Label>
                  <Input value={formData.username} disabled className="bg-muted" />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="edit-role">Role</Label>
                  <Select
                    value={formData.role}
                    onValueChange={(value: "user" | "admin" | "premium") =>
                      setFormData({ ...formData, role: value })
                    }
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="user">User</SelectItem>
                      <SelectItem value="admin">Admin</SelectItem>
                      <SelectItem value="premium">Premium</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setIsEditDialogOpen(false)}>
                  Cancel
                </Button>
                <Button onClick={handleEditUser}>Save Changes</Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>

          {/* Delete User Dialog */}
          <Dialog open={isDeleteDialogOpen} onOpenChange={setIsDeleteDialogOpen}>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Delete User</DialogTitle>
                <DialogDescription>
                  Are you sure you want to delete user "{selectedUser?.username}"? This action
                  cannot be undone.
                </DialogDescription>
              </DialogHeader>
              <DialogFooter>
                <Button variant="outline" onClick={() => setIsDeleteDialogOpen(false)}>
                  Cancel
                </Button>
                <Button variant="destructive" onClick={handleDeleteUser}>
                  Delete User
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </>
      )}
    </div>
  )
}
