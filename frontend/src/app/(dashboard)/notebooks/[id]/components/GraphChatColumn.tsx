'use client'

import { useState, useRef, useEffect } from 'react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { LoadingSpinner } from '@/components/common/LoadingSpinner'
import { Send, User, Bot, BookOpen, Fingerprint } from 'lucide-react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import axios from 'axios'

interface GraphChatColumnProps {
	notebookId: string
}

interface Message {
	role: 'user' | 'assistant'
	content: string
	metadata?: {
		mode: string
		sources?: string[]
		entities?: string[]
	}
}

export function GraphChatColumn({ notebookId }: GraphChatColumnProps) {
	const [messages, setMessages] = useState<Message[]>([
		{
			role: 'assistant',
			content: 'Welcome to GraphRAG Studio! Ask me anything about this notebook using the compiled Knowledge Graph.',
		},
	])
	const [input, setInput] = useState('')
	const [sending, setSending] = useState(false)
	const [mode, setMode] = useState<'local' | 'global' | 'hybrid'>('hybrid')

	const scrollRef = useRef<HTMLDivElement>(null)

	useEffect(() => {
		if (scrollRef.current) {
			scrollRef.current.scrollTop = scrollRef.current.scrollHeight
		}
	}, [messages, sending])

	const handleSend = async (e: React.FormEvent) => {
		e.preventDefault()
		if (!input.trim() || sending) return

		const userMsg = input.trim()
		setInput('')
		setMessages((prev) => [...prev, { role: 'user', content: userMsg }])
		setSending(true)

		try {
			const res = await axios.post(`/api/notebooks/${notebookId}/graph/query`, {
				query: userMsg,
				mode: mode,
			})

			setMessages((prev) => [
				...prev,
				{
					role: 'assistant',
					content: res.data.content,
					metadata: res.data.metadata,
				},
			])
		} catch (err) {
			console.error('Query failed:', err)
			setMessages((prev) => [
				...prev,
				{
					role: 'assistant',
					content: 'Error: Failed to retrieve answer from GraphRAG pipeline.',
				},
			])
		} finally {
			setSending(false)
		}
	}

	return (
		<Card className="h-full flex flex-col border border-slate-200 rounded-xl overflow-hidden shadow-sm bg-white">
			{/* Mode Select Header */}
			<div className="p-4 border-b bg-slate-50 flex items-center justify-between shrink-0">
				<span className="font-semibold text-sm">Query Mode</span>
				<Tabs value={mode} onValueChange={(val) => setMode(val as any)}>
					<TabsList className="grid w-[240px] grid-cols-3 h-8 p-0.5">
						<TabsTrigger value="local" className="text-xs py-1">Local</TabsTrigger>
						<TabsTrigger value="global" className="text-xs py-1">Global</TabsTrigger>
						<TabsTrigger value="hybrid" className="text-xs py-1">Hybrid</TabsTrigger>
					</TabsList>
				</Tabs>
			</div>

			{/* Chat Area */}
			<ScrollArea className="flex-1 p-4 bg-slate-50/30">
				<div ref={scrollRef} className="space-y-4 max-h-[460px] overflow-y-auto pr-2">
					{messages.map((msg, i) => {
						const isUser = msg.role === 'user'
						return (
							<div key={i} className={`flex gap-3 ${isUser ? 'justify-end' : 'justify-start'}`}>
								{!isUser && (
									<div className="w-8 h-8 bg-indigo-600 text-white rounded-lg flex items-center justify-center shrink-0 shadow-sm">
										<Bot className="h-4 w-4" />
									</div>
								)}

								<div className="flex flex-col gap-1 max-w-[80%]">
									<div
										className={`p-3.5 rounded-2xl text-sm ${
											isUser
												? 'bg-indigo-600 text-white rounded-tr-none'
												: 'bg-white border border-slate-100 text-slate-800 rounded-tl-none shadow-sm'
										}`}
									>
										<div className="prose prose-sm max-w-none prose-slate">
											<ReactMarkdown remarkPlugins={[remarkGfm]}>{msg.content}</ReactMarkdown>
										</div>
									</div>

									{/* Metadata badges (sources, entities) */}
									{!isUser && msg.metadata && (
										<div className="flex flex-wrap gap-1.5 mt-1.5">
											{msg.metadata.sources && msg.metadata.sources.length > 0 && (
												<div className="flex items-center gap-1 bg-slate-100 text-slate-600 text-[10px] px-2 py-0.5 rounded-full border border-slate-200">
													<BookOpen className="h-3 w-3" />
													<span>{msg.metadata.sources.length} sources</span>
												</div>
											)}
											{msg.metadata.entities && msg.metadata.entities.length > 0 && (
												<div className="flex items-center gap-1 bg-indigo-50 text-indigo-600 text-[10px] px-2 py-0.5 rounded-full border border-indigo-100">
													<Fingerprint className="h-3 w-3" />
													<span>{msg.metadata.entities.length} entities</span>
												</div>
											)}
										</div>
									)}
								</div>

								{isUser && (
									<div className="w-8 h-8 bg-slate-250 text-slate-700 rounded-lg flex items-center justify-center shrink-0 border border-slate-300">
										<User className="h-4 w-4" />
									</div>
								)}
							</div>
						)
					})}

					{sending && (
						<div className="flex gap-3 justify-start">
							<div className="w-8 h-8 bg-indigo-600 text-white rounded-lg flex items-center justify-center shrink-0">
								<Bot className="h-4 w-4" />
							</div>
							<div className="bg-white border border-slate-100 rounded-2xl rounded-tl-none p-4 shadow-sm flex items-center gap-2">
								<LoadingSpinner size="sm" className="text-indigo-600" />
								<span className="text-xs text-muted-foreground">Synthesizing answer...</span>
							</div>
						</div>
					)}
				</div>
			</ScrollArea>

			{/* Input Form */}
			<form onSubmit={handleSend} className="p-4 border-t bg-white shrink-0 flex gap-2">
				<Input
					value={input}
					onChange={(e) => setInput(e.target.value)}
					placeholder={`Query using ${mode} retrieval mode...`}
					disabled={sending}
					className="flex-1 rounded-xl focus-visible:ring-indigo-500"
				/>
				<Button
					type="submit"
					disabled={!input.trim() || sending}
					className="bg-indigo-600 hover:bg-indigo-700 text-white rounded-xl shadow"
				>
					<Send className="h-4 w-4" />
				</Button>
			</form>
		</Card>
	)
}
