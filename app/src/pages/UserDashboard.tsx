"use client"

import { useState, useRef, useEffect, useCallback } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import {
  Bot,
  ChevronDown,
  MessageSquare,
  Plus,
  Send,
  LogOut,
  Sparkles,
  User,
  Trash2,
  PanelLeftClose,
  PanelLeft,
  AlertCircle,
  Loader2,
} from "lucide-react"
import { cn } from "@/lib/utils"
import type { Chat, ChatMessage, AIModel } from "@/lib/types"
import { aiModels, getModelById, createModelFromId } from "@/lib/data"
import { promptManagerApi } from "@/lib/api"
import { Link, useNavigate } from 'react-router-dom';

const decodeJWT = (token: string): { username?: string; role?: string } | null => {
  try {
    const payload = token.split('.')[1];
    const decoded = JSON.parse(atob(payload));
    return { username: decoded.username, role: decoded.role };
  } catch (err) {
    console.error('Failed to decode JWT:', err);
    return null;
  }
};

export default function DashboardPage() {
  const navigate = useNavigate()
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [username, setUsername] = useState("")
  const [selectedModel, setSelectedModel] = useState<AIModel>(aiModels[0])
  const [availableModels, setAvailableModels] = useState<AIModel[]>(aiModels)
  const [chats, setChats] = useState<Chat[]>([])
  const [activeChat, setActiveChat] = useState<Chat | null>(null)
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [input, setInput] = useState("")
  const [isTyping, setIsTyping] = useState(false)
  const [isSending, setIsSending] = useState(false)
  const [isLoadingMessages, setIsLoadingMessages] = useState(false)
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const pollingRef = useRef<number | null>(null)
  const isSendingRef = useRef(false)

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" })
  }

  const handleLogout = () => {
    localStorage.removeItem('token');
    window.location.reload();
  };

  useEffect(() => {
    scrollToBottom()
  }, [messages])

  // Fetch available models
  const fetchModels = useCallback(async () => {
    try {
      const modelIds = await promptManagerApi.getModels()
      const models = modelIds.map(id => createModelFromId(id))
      setAvailableModels(models)
      if (models.length > 0) {
        setSelectedModel(models[0])
      }
    } catch (err) {
      console.error('Failed to fetch models:', err)
      // Fall back to default models
      setAvailableModels(aiModels)
    }
  }, [])

  // Fetch user's chats
  const fetchChats = useCallback(async () => {
    try {
      const response = await promptManagerApi.getChats(1, 50)
      setChats(response.chats || [])
    } catch (err) {
      console.error('Failed to fetch chats:', err)
      setError('Failed to load conversations')
    }
  }, [])

  // Fetch messages for active chat
  const fetchMessages = useCallback(async (chatId: string, showLoading = true) => {
    try {
      if (showLoading) setIsLoadingMessages(true)
      const response = await promptManagerApi.getChat(chatId)
      const incomingMessages = response.messages || []
      setMessages(incomingMessages)
      setActiveChat(response.chat)

      // Check if there's a pending message that needs polling
      const hasPendingMessage = incomingMessages.some(
        (msg: ChatMessage) => msg.status === 'pending' || msg.status === 'streaming'
      )
      if (hasPendingMessage) {
        setIsTyping(true)
      } else {
        setIsTyping(false)
      }
    } catch (err) {
      console.error('Failed to fetch messages:', err)
      setError('Failed to load messages')
    } finally {
      if (showLoading) setIsLoadingMessages(false)
    }
  }, [])

  // Poll for message updates when there's a pending message
  const startPolling = useCallback((chatId: string) => {
    // Clear any existing polling
    if (pollingRef.current) {
      clearInterval(pollingRef.current)
    }

    pollingRef.current = setInterval(async () => {
      try {
        const response = await promptManagerApi.getChat(chatId)
        const incomingMessages = response.messages || []
        setMessages(incomingMessages)

        // Check if still has pending messages
        const hasPendingMessage = incomingMessages.some(
          (msg: ChatMessage) => msg.status === 'pending' || msg.status === 'streaming'
        )

        if (!hasPendingMessage) {
          setIsTyping(false)
          if (pollingRef.current) {
            clearInterval(pollingRef.current)
            pollingRef.current = null
          }
        }
      } catch (err) {
        console.error('Polling error:', err)
      }
    }, 2000) // Poll every 2 seconds
  }, [])

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (pollingRef.current) {
        clearInterval(pollingRef.current)
      }
    }
  }, [])

  // Validate token and load initial data
  useEffect(() => {
    const validateToken = async () => {
      const token = localStorage.getItem('token')

      if (!token) {
        navigate('/')
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
          setIsAuthenticated(true)
          const decoded = decodeJWT(token)
          if (decoded?.username) {
            setUsername(decoded.username)
          }
          // Load models and chats after authentication
          await fetchModels()
          await fetchChats()
        } else {
          navigate('/')
          setIsLoading(false)
        }
      } catch {
        navigate('/')
      } finally {
        setIsLoading(false)
      }
    }

    validateToken()
  }, [navigate, fetchModels, fetchChats])

  // Load messages when active chat changes
  useEffect(() => {
    if (activeChat) {
      fetchMessages(activeChat.id)
    } else {
      setMessages([])
    }
  }, [activeChat?.id, fetchMessages])

  const handleSendMessage = async () => {
    // Use ref for synchronous check to prevent double-sends
    if (!input.trim() || isSendingRef.current) return

    // Set ref immediately to block any concurrent calls
    isSendingRef.current = true
    setIsSending(true)

    // If no active chat, create one first
    let currentChat = activeChat
    if (!currentChat) {
      try {
        currentChat = await promptManagerApi.createChat(selectedModel.id)
        setActiveChat(currentChat)
        setChats(prev => [currentChat!, ...prev])
      } catch (err) {
        console.error('Failed to create chat:', err)
        setError('Failed to create conversation')
        isSendingRef.current = false
        setIsSending(false)
        return
      }
    }

    const messageContent = input
    setInput("")
    setError(null)

    try {
      // Send the message
      await promptManagerApi.sendMessage(currentChat.id, messageContent)

      // Fetch messages from server to get the sent message
      await fetchMessages(currentChat.id, false)

      // Start typing indicator and polling for response
      setIsTyping(true)
      startPolling(currentChat.id)

      // Refresh chats to get updated titles
      await fetchChats()
    } catch (err) {
      console.error('Failed to send message:', err)
      setError('Failed to send message')
    } finally {
      isSendingRef.current = false
      setIsSending(false)
    }
  }

  const handleNewChat = () => {
    // Stop any existing polling
    if (pollingRef.current) {
      clearInterval(pollingRef.current)
      pollingRef.current = null
    }
    // Clear state for new chat - chat will be created when first message is sent
    setActiveChat(null)
    setMessages([])
    setIsTyping(false)
    setError(null)
  }

  const handleSelectChat = (chat: Chat) => {
    // Stop any existing polling
    if (pollingRef.current) {
      clearInterval(pollingRef.current)
      pollingRef.current = null
    }
    // Clear messages immediately when switching chats
    setMessages([])
    setIsTyping(false)
    setActiveChat(chat)
    // Update selected model to match the chat's model
    const chatModel = getModelById(chat.model) || createModelFromId(chat.model)
    setSelectedModel(chatModel)
  }

  const handleDeleteChat = async (chatId: string, e: React.MouseEvent) => {
    e.stopPropagation()
    try {
      await promptManagerApi.deleteChat(chatId)
      setChats(prev => prev.filter(c => c.id !== chatId))
      if (activeChat?.id === chatId) {
        setActiveChat(null)
        setMessages([])
      }
    } catch (err) {
      console.error('Failed to delete chat:', err)
      setError('Failed to delete conversation')
    }
  }

  const formatTimestamp = (dateString: string) => {
    const date = new Date(dateString)
    return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
  }

  const getMessageStatusIcon = (status: ChatMessage['status']) => {
    switch (status) {
      case 'pending':
      case 'streaming':
        return <Loader2 className="h-3 w-3 animate-spin" />
      case 'failed':
        return <AlertCircle className="h-3 w-3 text-destructive" />
      default:
        return null
    }
  }

  return (
    <div className="flex h-screen w-full bg-background">
      {isLoading && (
        <div className="absolute inset-0 z-50 flex items-center justify-center bg-background">
          <div className="text-center">
            <Loader2 className="mx-auto mb-4 h-10 w-10 animate-spin text-primary" />
            <p className="text-muted-foreground">Loading...</p>
          </div>
        </div>
      )}

      {!isLoading && isAuthenticated && (
        <div className="flex h-full w-full">
          {/* Sidebar */}
          <aside
            className={cn(
              "flex flex-col border-r border-border bg-card transition-all duration-300",
              sidebarOpen ? "w-72" : "w-0 overflow-hidden"
            )}
          >
            {/* Logo */}
            <div className="flex h-14 items-center border-b border-border px-4">
              <Link to="/" className="flex items-center gap-2">
                <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary">
                  <Bot className="h-4 w-4 text-primary-foreground" />
                </div>
                <span className="font-semibold text-foreground">AIDC</span>
              </Link>
            </div>

            {/* New Chat Button */}
            <div className="p-3">
              <Button
                onClick={handleNewChat}
                className="w-full justify-start gap-2"
              >
                <Plus className="h-4 w-4" />
                New Chat
              </Button>
            </div>

            {/* Chat List */}
            <ScrollArea className="flex-1 px-3">
              <div className="space-y-1 pb-4">
                <p className="px-2 py-2 text-xs font-medium text-muted-foreground">
                  Recent Conversations
                </p>
                {chats.map((chat) => (
                  <div
                    key={chat.id}
                    className={cn(
                      "group flex cursor-pointer items-center justify-between rounded-md px-2 py-2 text-sm transition-colors",
                      activeChat?.id === chat.id
                        ? "bg-accent text-accent-foreground"
                        : "text-muted-foreground hover:bg-accent/50 hover:text-foreground"
                    )}
                    onClick={() => handleSelectChat(chat)}
                  >
                    <div className="flex items-center gap-2 truncate">
                      <MessageSquare className="h-4 w-4 shrink-0" />
                      <span className="truncate">{chat.title || "New Chat"}</span>
                    </div>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6 opacity-0 transition-opacity group-hover:opacity-100"
                      onClick={(e) => handleDeleteChat(chat.id, e)}
                    >
                      <Trash2 className="h-3 w-3" />
                    </Button>
                  </div>
                ))}
                {chats.length === 0 && (
                  <p className="px-2 py-4 text-center text-xs text-muted-foreground">
                    No conversations yet
                  </p>
                )}
              </div>
            </ScrollArea>

            {/* User Section */}
            <div className="border-t border-border p-3">
              <div className="flex items-center gap-3 rounded-md bg-secondary/50 p-2">
                <Avatar className="h-8 w-8">
                  <AvatarFallback className="bg-primary/10 text-primary text-sm">
                    {username ? username.charAt(0).toUpperCase() : "U"}
                  </AvatarFallback>
                </Avatar>
                <div className="flex-1 truncate">
                  <p className="text-sm font-medium text-foreground">{username || "User"}</p>
                </div>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8 text-muted-foreground hover:text-foreground"
                  onClick={handleLogout}
                  title="Logout"
                >
                  <LogOut className="h-4 w-4" />
                </Button>
              </div>
            </div>
          </aside>

          {/* Main Content */}
          <main className="flex flex-1 flex-col overflow-hidden">
            {/* Header */}
            <header className="flex h-14 items-center justify-between border-b border-border px-4">
              <div className="flex items-center gap-3">
                <Button
                  variant="ghost"
                  size="icon"
                  onClick={() => setSidebarOpen(!sidebarOpen)}
                  className="h-8 w-8"
                >
                  {sidebarOpen ? (
                    <PanelLeftClose className="h-4 w-4" />
                  ) : (
                    <PanelLeft className="h-4 w-4" />
                  )}
                </Button>

                {activeChat ? (
                  <div className="flex items-center gap-2 rounded-md border border-border bg-secondary/30 px-3 py-1.5">
                    <span className="text-base">{selectedModel.icon}</span>
                    <span className="text-sm font-medium text-foreground">
                      {selectedModel.name}
                    </span>
                  </div>
                ) : (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="outline" size="sm" className="gap-2">
                        <span className="text-base">{selectedModel.icon}</span>
                        <span className="font-medium">{selectedModel.name}</span>
                        <ChevronDown className="h-3 w-3 text-muted-foreground" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="start" className="w-64">
                      {availableModels.map((model) => (
                        <DropdownMenuItem
                          key={model.id}
                          onClick={() => setSelectedModel(model)}
                          className="cursor-pointer p-3"
                        >
                          <div className="flex items-start gap-3">
                            <span className="text-lg">{model.icon}</span>
                            <div>
                              <p className="font-medium">{model.name}</p>
                              <p className="text-xs text-muted-foreground">
                                {model.description}
                              </p>
                            </div>
                          </div>
                        </DropdownMenuItem>
                      ))}
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
              </div>

              <Link to="/admin">
                <Button variant="outline" size="sm">
                  Admin Panel
                </Button>
              </Link>
            </header>

            {/* Error Banner */}
            {error && (
              <div className="mx-4 mt-3 flex items-center gap-2 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-destructive">
                <AlertCircle className="h-4 w-4 shrink-0" />
                <span className="flex-1 text-sm">{error}</span>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-6 px-2 text-destructive hover:bg-destructive/20 hover:text-destructive"
                  onClick={() => setError(null)}
                >
                  Dismiss
                </Button>
              </div>
            )}

            {/* Messages Area */}
            <ScrollArea className="flex-1 min-h-0">
              <div className="p-6">
                {isLoadingMessages ? (
                  <div className="flex h-[50vh] flex-col items-center justify-center">
                    <Loader2 className="h-8 w-8 animate-spin text-primary" />
                    <p className="mt-3 text-sm text-muted-foreground">
                      Loading messages...
                    </p>
                  </div>
                ) : messages.length === 0 ? (
                  <div className="flex h-[50vh] flex-col items-center justify-center text-center">
                    <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-2xl bg-primary/10">
                      <Sparkles className="h-8 w-8 text-primary" />
                    </div>
                    <h2 className="mb-2 text-xl font-semibold text-foreground">
                      How can I help you today?
                    </h2>
                    <p className="mb-6 max-w-sm text-sm text-muted-foreground">
                      Start a conversation with {selectedModel.name}. Ask questions,
                      get creative assistance, or explore ideas together.
                    </p>
                    <div className="grid max-w-md gap-2 sm:grid-cols-2">
                      {[
                        "Explain quantum computing",
                        "Write a creative story",
                        "Help me debug code",
                        "Summarize an article",
                      ].map((prompt, i) => (
                        <Button
                          key={i}
                          variant="outline"
                          size="sm"
                          className="justify-start text-left"
                          onClick={() => setInput(prompt)}
                        >
                          {prompt}
                        </Button>
                      ))}
                    </div>
                  </div>
                ) : (
                  <div className="mx-auto max-w-3xl space-y-4">
                    {messages.map((message) => (
                      <div
                        key={message.id}
                        className={cn(
                          "flex gap-3",
                          message.role === "user" ? "justify-end" : "justify-start"
                        )}
                      >
                        {message.role === "assistant" && (
                          <Avatar className="h-8 w-8 shrink-0">
                            <AvatarFallback className="bg-primary text-primary-foreground">
                              <Bot className="h-4 w-4" />
                            </AvatarFallback>
                          </Avatar>
                        )}
                        <div
                          className={cn(
                            "max-w-[75%] rounded-xl px-4 py-2.5",
                            message.role === "user"
                              ? "bg-primary text-primary-foreground"
                              : "bg-secondary text-secondary-foreground",
                            message.status === "failed" && "border border-destructive/50"
                          )}
                        >
                          {message.status === "pending" || message.status === "streaming" ? (
                            <div className="flex items-center gap-2">
                              <Loader2 className="h-4 w-4 animate-spin" />
                              <span className="text-sm">Generating...</span>
                            </div>
                          ) : (
                            <>
                              <p className="text-sm leading-relaxed whitespace-pre-wrap">
                                {message.content}
                              </p>
                              {message.error_message && (
                                <p className="mt-2 text-xs text-destructive">
                                  {message.error_message}
                                </p>
                              )}
                            </>
                          )}
                          <div className="mt-1.5 flex items-center gap-2 text-xs opacity-60">
                            <span>{formatTimestamp(message.created_at)}</span>
                            {getMessageStatusIcon(message.status)}
                            {message.tokens_used && (
                              <span>({message.tokens_used} tokens)</span>
                            )}
                          </div>
                        </div>
                        {message.role === "user" && (
                          <Avatar className="h-8 w-8 shrink-0">
                            <AvatarFallback className="bg-secondary text-secondary-foreground">
                              <User className="h-4 w-4" />
                            </AvatarFallback>
                          </Avatar>
                        )}
                      </div>
                    ))}
                    {isTyping && !messages.some(m => m.status === "pending" || m.status === "streaming") && (
                      <div className="flex gap-3">
                        <Avatar className="h-8 w-8 shrink-0">
                          <AvatarFallback className="bg-primary text-primary-foreground">
                            <Bot className="h-4 w-4" />
                          </AvatarFallback>
                        </Avatar>
                        <div className="rounded-xl bg-secondary px-4 py-3">
                          <div className="flex gap-1">
                            <span className="h-2 w-2 animate-bounce rounded-full bg-muted-foreground [animation-delay:0ms]" />
                            <span className="h-2 w-2 animate-bounce rounded-full bg-muted-foreground [animation-delay:150ms]" />
                            <span className="h-2 w-2 animate-bounce rounded-full bg-muted-foreground [animation-delay:300ms]" />
                          </div>
                        </div>
                      </div>
                    )}
                    <div ref={messagesEndRef} />
                  </div>
                )}
              </div>
            </ScrollArea>

            {/* Input Area */}
            <div className="border-t border-border p-4">
              <div className="mx-auto max-w-3xl">
                <div className="flex gap-2">
                  <Input
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && !e.shiftKey) {
                        e.preventDefault()
                        handleSendMessage()
                      }
                    }}
                    placeholder={`Message ${selectedModel.name}...`}
                    disabled={isSending}
                    className="flex-1"
                  />
                  <Button
                    onClick={handleSendMessage}
                    disabled={!input.trim() || isSending}
                    size="icon"
                  >
                    {isSending ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <Send className="h-4 w-4" />
                    )}
                  </Button>
                </div>
                <p className="mt-2 text-center text-xs text-muted-foreground">
                  {selectedModel.name} may produce inaccurate information.
                </p>
              </div>
            </div>
          </main>
        </div>
      )}
    </div>
  )
}
