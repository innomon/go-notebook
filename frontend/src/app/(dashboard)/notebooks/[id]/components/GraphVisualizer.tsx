'use client'

import { useState, useEffect, useRef } from 'react'
import dynamic from 'next/dynamic'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { ScrollArea } from '@/components/ui/scroll-area'
import { LoadingSpinner } from '@/components/common/LoadingSpinner'
import { RefreshCw, Network, Info, CheckCircle2, AlertTriangle, Layers } from 'lucide-react'
import axios from 'axios'

// Load ForceGraph2D dynamically to bypass SSR issues
const ForceGraph2D = dynamic(() => import('react-force-graph-2d'), { ssr: false })

interface GraphVisualizerProps {
	notebookId: string
}

interface Connection {
	source: string
	target: string
	weight: number
}

interface Community {
	id: number
	summary: string
	top_entities: string[]
	size: number
	num_chunks: number
}

interface GraphData {
	connections: Connection[]
	communities: Community[]
	top_nodes: string[]
}

// Custom community color palette
const COLORS = [
	'#6366f1', // Indigo
	'#ec4899', // Pink
	'#10b981', // Emerald
	'#f59e0b', // Amber
	'#3b82f6', // Blue
	'#8b5cf6', // Violet
	'#ef4444', // Red
	'#14b8a6', // Teal
	'#f97316', // Orange
	'#06b6d4', // Cyan
]

export function GraphVisualizer({ notebookId }: GraphVisualizerProps) {
	const [graphData, setGraphData] = useState<GraphData>({ connections: [], communities: [], top_nodes: [] })
	const [loading, setLoading] = useState(true)
	const [rebuilding, setRebuilding] = useState(false)
	const [jobStatus, setJobStatus] = useState<string>('')
	const [selectedCommunity, setSelectedCommunity] = useState<number | null>(null)
	const [maxNodes, setMaxNodes] = useState(30)

	const containerRef = useRef<HTMLDivElement>(null)
	const [dimensions, setDimensions] = useState({ width: 500, height: 400 })

	// Fetch graph data
	const fetchGraph = async () => {
		setLoading(true)
		try {
			const res = await axios.get(`/api/notebooks/${notebookId}/graph?max_nodes=${maxNodes}`)
			setGraphData(res.data)
		} catch (err) {
			console.error('Failed to fetch graph data:', err)
		} finally {
			setLoading(false)
		}
	}

	useEffect(() => {
		fetchGraph()
	}, [notebookId, maxNodes])

	// Handle dimensions resize
	useEffect(() => {
		const handleResize = () => {
			if (containerRef.current) {
				setDimensions({
					width: containerRef.current.clientWidth,
					height: containerRef.current.clientHeight || 450,
				})
			}
		}

		handleResize()
		window.addEventListener('resize', handleResize)
		return () => window.removeEventListener('resize', handleResize)
	}, [loading, rebuilding])

	// Trigger background graph build job
	const handleBuildGraph = async () => {
		setRebuilding(true)
		setJobStatus('Submitting rebuild task...')
		try {
			const res = await axios.post(`/api/notebooks/${notebookId}/graph/build`)
			const jobId = res.data.job_id

			// Poll background job status
			const interval = setInterval(async () => {
				try {
					const jobRes = await axios.get(`/api/config`) // A safe endpoint, let's query surreal job status directly
					// Alternatively, we can query commands status in SurrealDB:
					const statusRes = await axios.get(`/api/notebooks/${notebookId}/graph`) // If build completes, graph data is updated
					if (statusRes.data.top_nodes.length > 0) {
						clearInterval(interval)
						setRebuilding(false)
						setJobStatus('')
						fetchGraph()
					}
				} catch (e) {
					// Keep polling
				}
			}, 4000)

			// Timeout fallback after 3 minutes
			setTimeout(() => {
				clearInterval(interval)
				setRebuilding(false)
				setJobStatus('')
				fetchGraph()
			}, 180000)

		} catch (err) {
			console.error('Failed to trigger graph build:', err)
			setRebuilding(false)
			setJobStatus('Build failed.')
		}
	}

	// Prepare force-directed graph input format
	const uniqueNodes = new Set<string>()
	const links = graphData.connections.map((c) => {
		uniqueNodes.add(c.source)
		uniqueNodes.add(c.target)
		return {
			source: c.source,
			target: c.target,
			val: c.weight,
		}
	})

	// Match entities with their community cluster ID for color coding
	const entityCommunityMap = new Map<string, number>()
	graphData.communities.forEach((comm) => {
		comm.top_entities.forEach((ent) => {
			entityCommunityMap.set(ent.toLowerCase(), comm.id)
		})
	})

	const nodes = Array.from(uniqueNodes).map((nodeId) => {
		const commId = entityCommunityMap.get(nodeId.toLowerCase()) ?? -1
		return {
			id: nodeId,
			group: commId,
			color: commId !== -1 ? COLORS[commId % COLORS.length] : '#94a3b8',
		}
	})

	const forceData = { nodes, links }

	// Filter highlighted items if community is selected
	const isHighlightedNode = (node: any) => {
		if (selectedCommunity === null) return true
		const comm = graphData.communities.find((c) => c.id === selectedCommunity)
		if (!comm) return true
		return comm.top_entities.some((e) => e.toLowerCase() === node.id.toLowerCase())
	}

	const isHighlightedLink = (link: any) => {
		if (selectedCommunity === null) return true
		const comm = graphData.communities.find((c) => c.id === selectedCommunity)
		if (!comm) return true
		const src = typeof link.source === 'object' ? link.source.id : link.source
		const tgt = typeof link.target === 'object' ? link.target.id : link.target
		return (
			comm.top_entities.some((e) => e.toLowerCase() === src.toLowerCase()) &&
			comm.top_entities.some((e) => e.toLowerCase() === tgt.toLowerCase())
		)
	}

	return (
		<div className="flex flex-col h-full gap-4">
			{/* Build Status & Controls Banner */}
			<div className="flex items-center justify-between bg-white border rounded-xl p-4 shadow-sm shrink-0">
				<div className="flex items-center gap-3">
					<div className="p-2.5 bg-indigo-50 text-indigo-600 rounded-lg">
						<Network className="h-5 w-5" />
					</div>
					<div>
						<h3 className="font-semibold text-sm">Knowledge Graph Pipeline</h3>
						<p className="text-xs text-muted-foreground">
							{graphData.top_nodes.length > 0
								? `Graph contains ${graphData.top_nodes.length} nodes & ${graphData.connections.length} relations divided into ${graphData.communities.length} clusters.`
								: 'No Knowledge Graph constructed yet for this notebook.'}
						</p>
					</div>
				</div>

				<div className="flex items-center gap-2">
					<Button
						variant="outline"
						size="sm"
						onClick={fetchGraph}
						disabled={loading || rebuilding}
					>
						<RefreshCw className={`h-4 w-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
						Refresh
					</Button>
					<Button
						variant="default"
						size="sm"
						onClick={handleBuildGraph}
						disabled={rebuilding}
						className="bg-indigo-600 hover:bg-indigo-700 text-white shadow"
					>
						<RefreshCw className={`h-4 w-4 mr-2 ${rebuilding ? 'animate-spin' : ''}`} />
						{rebuilding ? 'Building Graph...' : 'Build Graph'}
					</Button>
				</div>
			</div>

			{rebuilding && (
				<Alert className="bg-indigo-50/50 border-indigo-100 shrink-0">
					<LoadingSpinner size="sm" className="text-indigo-600 mr-3 inline-block" />
					<AlertTitle className="text-indigo-950 font-medium">Rebuilding Knowledge Graph</AlertTitle>
					<AlertDescription className="text-indigo-750 text-xs mt-1">
						The background worker is currently chunking files, running LLM entity extraction, and resolving communities. This takes a few moments.
					</AlertDescription>
				</Alert>
			)}

			{/* Main Split Layout */}
			<div className="flex flex-1 min-h-0 gap-6">
				{/* 2D Interactive Force Graph Card */}
				<Card ref={containerRef} className="flex-1 min-w-0 h-[480px] bg-slate-950 relative overflow-hidden border-slate-800 rounded-xl shadow">
					{loading ? (
						<div className="absolute inset-0 flex items-center justify-center bg-slate-950/80 z-10">
							<LoadingSpinner size="lg" className="text-indigo-500" />
						</div>
					) : forceData.nodes.length === 0 ? (
						<div className="absolute inset-0 flex flex-col items-center justify-center text-slate-400 gap-3">
							<Info className="h-10 w-10 text-slate-600" />
							<p className="text-sm">Click &quot;Build Graph&quot; to compile your knowledge graph.</p>
						</div>
					) : (
						<ForceGraph2D
							graphData={forceData}
							width={dimensions.width}
							height={dimensions.height}
							nodeLabel="id"
							nodeColor={(node: any) =>
								isHighlightedNode(node) ? node.color : '#334155'
							}
							nodeVal={(node: any) => {
								// Node size proportional to degree count
								const weight = graphData.connections.reduce((acc, c) => {
									if (c.source === node.id || c.target === node.id) {
										return acc + c.weight
									}
									return acc
								}, 1)
								return Math.max(4, Math.min(12, weight * 2))
							}}
							linkColor={(link: any) =>
								isHighlightedLink(link) ? '#475569' : '#1e293b'
							}
							linkWidth={(link: any) =>
								isHighlightedLink(link) ? Math.min(5, link.val) : 0.5
							}
							backgroundColor="#090d16"
							cooldownTicks={100}
						/>
					)}
				</Card>

				{/* Communities List Panel */}
				<Card className="w-80 border rounded-xl overflow-hidden flex flex-col shadow-sm bg-white shrink-0">
					<div className="p-4 border-b bg-slate-50 flex items-center gap-2 shrink-0">
						<Layers className="h-4 w-4 text-slate-500" />
						<span className="font-semibold text-sm">Thematic Clusters</span>
					</div>

					<ScrollArea className="flex-1 p-4">
						{graphData.communities.length === 0 ? (
							<div className="text-center text-xs text-muted-foreground py-8">
								Build the graph to identify thematic communities.
							</div>
						) : (
							<div className="space-y-3">
								<Button
									variant={selectedCommunity === null ? 'default' : 'outline'}
									size="sm"
									className="w-full text-xs justify-start"
									onClick={() => setSelectedCommunity(null)}
								>
									Show All Entities
								</Button>

								{graphData.communities.map((comm) => {
									const isSelected = selectedCommunity === comm.id
									return (
										<div
											key={comm.id}
											onClick={() => setSelectedCommunity(isSelected ? null : comm.id)}
											className={`p-3 border rounded-xl cursor-pointer transition-all ${
												isSelected
													? 'bg-indigo-50 border-indigo-200 shadow-sm'
													: 'hover:bg-slate-50 border-slate-100'
											}`}
										>
											<div className="flex items-center justify-between mb-1.5">
												<div className="flex items-center gap-2">
													<div
														className="w-2.5 h-2.5 rounded-full"
														style={{ backgroundColor: COLORS[comm.id % COLORS.length] }}
													/>
													<span className="font-semibold text-xs text-slate-800">
														Cluster {comm.id + 1}
													</span>
												</div>
												<Badge variant="secondary" className="text-[10px] px-1.5 py-0.5">
													{comm.size} entities
												</Badge>
											</div>
											<p className="text-[11px] leading-relaxed text-slate-600 line-clamp-3">
												{comm.summary}
											</p>
										</div>
									)
								})}
							</div>
						)}
					</ScrollArea>
				</Card>
			</div>
		</div>
	)
}
