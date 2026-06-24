'use client'

import React, { useState, useEffect, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import apiClient from '@/lib/api/client'
import { VisualGraph, GraphNode, GraphLink } from '@/components/okf/VisualGraph'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Search,
  Folder,
  ArrowRight,
  ChevronRight,
  ChevronLeft,
  Settings,
  Share2,
  FileText,
  FileCode,
  Tag,
  BookOpen,
  Download,
  Copy,
  Check
} from 'lucide-react'
import { useTranslation } from '@/lib/hooks/use-translation'

export default function OkfGraphPage() {
  const { t } = useTranslation()
  const [workspacePath, setWorkspacePath] = useState('')
  const [activePath, setActivePath] = useState('')

  // Selected Node State
  const [selectedNode, setSelectedNode] = useState<GraphNode | null>(null)
  const [isSidebarOpen, setIsSidebarOpen] = useState(true)

  // Filters State
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedTypes, setSelectedTypes] = useState<Record<string, boolean>>({
    Concept: true,
    Feature: true,
    Task: true,
    Code: true,
    Issue: true
  })

  // Load initial workspace path from localStorage or default to backend-friendly current dir
  useEffect(() => {
    if (typeof window !== 'undefined') {
      const saved = localStorage.getItem('okf-workspace-path') || './'
      setWorkspacePath(saved)
      setActivePath(saved)
    }
  }, [])

  // Handle workspace path submission
  const handleUpdateWorkspace = (e: React.FormEvent) => {
    e.preventDefault()
    if (typeof window !== 'undefined') {
      localStorage.setItem('okf-workspace-path', workspacePath)
    }
    setActivePath(workspacePath)
    setSelectedNode(null)
  }

  // Query graph data from backend
  const { data: graphData, isLoading, error } = useQuery<{ nodes: GraphNode[]; links: GraphLink[] }>({
    queryKey: ['okf-graph', activePath],
    queryFn: async () => {
      const response = await apiClient.get('/okf/graph', { params: { path: activePath } })
      return response.data
    },
    enabled: !!activePath
  })

  // Safe defaults
  const nodes = graphData?.nodes || []
  const links = graphData?.links || []

  // Extract all available types for dynamic checkboxes
  const availableTypes = useMemo(() => {
    const types = new Set<string>()
    nodes.forEach(n => {
      if (n.type) types.add(n.type)
    })
    return Array.from(types)
  }, [nodes])

  // Initialize selected types when nodes load
  useEffect(() => {
    if (availableTypes.length > 0) {
      const initial: Record<string, boolean> = {}
      availableTypes.forEach(t => {
        initial[t] = true
      })
      setSelectedTypes(prev => {
        // Only override if we don't have types tracked yet
        const hasTypes = Object.keys(prev).some(k => availableTypes.includes(k))
        return hasTypes ? prev : initial
      })
    }
  }, [availableTypes])

  // Filter nodes based on Search Query and Type filters
  const filteredNodes = useMemo(() => {
    return nodes.filter(node => {
      // Type filter
      if (node.type && selectedTypes[node.type] === false) {
        return false
      }
      // Search filter
      const query = searchQuery.toLowerCase()
      const titleMatch = (node.title || '').toLowerCase().includes(query)
      const descriptionMatch = (node.description || '').toLowerCase().includes(query)
      const pathMatch = (node.id || '').toLowerCase().includes(query)
      const tagMatch = (node.tags || []).some(t => t.toLowerCase().includes(query))

      return titleMatch || descriptionMatch || pathMatch || tagMatch
    })
  }, [nodes, searchQuery, selectedTypes])

  // Map links to target the remaining filtered nodes list
  const filteredLinks = useMemo(() => {
    const activeNodeIds = new Set(filteredNodes.map(n => n.id))
    return links.filter(link => {
      const sourceId = typeof link.source === 'object' ? (link.source as any).id : link.source
      const targetId = typeof link.target === 'object' ? (link.target as any).id : link.target
      return activeNodeIds.has(sourceId) && activeNodeIds.has(targetId)
    })
  }, [filteredNodes, links])

  // Find Inbound & Outbound references for the selected node
  const { inboundRefs, outboundRefs } = useMemo(() => {
    const inbound: GraphNode[] = []
    const outbound: GraphNode[] = []

    if (!selectedNode) return { inboundRefs: inbound, outboundRefs: outbound }

    links.forEach(link => {
      const sourceId = typeof link.source === 'object' ? (link.source as any).id : link.source
      const targetId = typeof link.target === 'object' ? (link.target as any).id : link.target

      if (sourceId === selectedNode.id) {
        const targetNode = nodes.find(n => n.id === targetId)
        if (targetNode) outbound.push(targetNode)
      }
      if (targetId === selectedNode.id) {
        const sourceNode = nodes.find(n => n.id === sourceId)
        if (sourceNode) inbound.push(sourceNode)
      }
    })

    return { inboundRefs: inbound, outboundRefs: outbound }
  }, [selectedNode, nodes, links])

  const handleNodeSelect = (node: GraphNode) => {
    setSelectedNode(node)
    setIsSidebarOpen(true)
  }

  // Toggle Type Selection
  const toggleType = (type: string) => {
    setSelectedTypes(prev => ({
      ...prev,
      [type]: !prev[type]
    }))
  }

  // Copy Mermaid syntax to clipboard
  const [copiedMermaid, setCopiedMermaid] = useState(false)
  const handleExportMermaid = () => {
    let mermaid = 'graph TD\n'
    
    // Nodes styling definitions
    mermaid += '  classDef concept fill:#10b981,stroke:#09090b,stroke-width:2px,color:#fff;\n'
    mermaid += '  classDef feature fill:#3b82f6,stroke:#09090b,stroke-width:2px,color:#fff;\n'
    mermaid += '  classDef task fill:#8b5cf6,stroke:#09090b,stroke-width:2px,color:#fff;\n'
    mermaid += '  classDef code fill:#f59e0b,stroke:#09090b,stroke-width:2px,color:#fff;\n'
    mermaid += '  classDef defaultStyle fill:#71717a,stroke:#09090b,stroke-width:2px,color:#fff;\n\n'

    // Add nodes
    filteredNodes.forEach(node => {
      const idClean = node.id.replace(/[^a-zA-Z0-9]/g, '_')
      mermaid += `  ${idClean}["${node.title || node.id}"]\n`
      
      const typeLower = (node.type || '').toLowerCase()
      if (['concept', 'feature', 'task', 'code'].includes(typeLower)) {
        mermaid += `  class ${idClean} ${typeLower};\n`
      } else {
        mermaid += `  class ${idClean} defaultStyle;\n`
      }
    })

    mermaid += '\n'

    // Add connections
    filteredLinks.forEach(link => {
      const sourceId = typeof link.source === 'object' ? (link.source as any).id : link.source
      const targetId = typeof link.target === 'object' ? (link.target as any).id : link.target
      
      const sClean = sourceId.replace(/[^a-zA-Z0-9]/g, '_')
      const tClean = targetId.replace(/[^a-zA-Z0-9]/g, '_')
      mermaid += `  ${sClean} --> ${tClean}\n`
    })

    navigator.clipboard.writeText(mermaid)
    setCopiedMermaid(true)
    setTimeout(() => setCopiedMermaid(false), 2000)
  }

  // Download adjacency JSON
  const handleExportJson = () => {
    const payload = {
      nodes: filteredNodes,
      links: filteredLinks
    }
    const dataStr = 'data:text/json;charset=utf-8,' + encodeURIComponent(JSON.stringify(payload, null, 2))
    const downloadAnchor = document.createElement('a')
    downloadAnchor.setAttribute('href', dataStr)
    downloadAnchor.setAttribute('download', `okf_adjacency_matrix_${new Date().toISOString().split('T')[0]}.json`)
    document.body.appendChild(downloadAnchor)
    downloadAnchor.click()
    downloadAnchor.remove()
  }

  // Download nodes CSV
  const handleExportCsv = () => {
    let csv = 'File Path,Title,Type,Description,Tags\n'
    filteredNodes.forEach(n => {
      const tagsStr = (n.tags || []).join(';')
      const desc = (n.description || '').replace(/"/g, '""')
      const title = (n.title || '').replace(/"/g, '""')
      csv += `"${n.id}","${title}","${n.type || ''}","${desc}","${tagsStr}"\n`
    })

    const dataStr = 'data:text/csv;charset=utf-8,' + encodeURIComponent(csv)
    const downloadAnchor = document.createElement('a')
    downloadAnchor.setAttribute('href', dataStr)
    downloadAnchor.setAttribute('download', `okf_nodes_registry_${new Date().toISOString().split('T')[0]}.csv`)
    document.body.appendChild(downloadAnchor)
    downloadAnchor.click()
    downloadAnchor.remove()
  }

  return (
    <div className="flex flex-col h-[calc(100vh-4rem)] w-full text-zinc-100 bg-[#09090b]">
      {/* Dynamic workspace path selector */}
      <div className="border-b border-zinc-800 bg-zinc-950/40 p-4 backdrop-blur-md">
        <form onSubmit={handleUpdateWorkspace} className="flex flex-col md:flex-row items-stretch md:items-center gap-3 max-w-4xl">
          <div className="flex-1 flex items-center gap-2 rounded-lg border border-zinc-800 bg-zinc-900/40 px-3 py-1.5 focus-within:border-zinc-700 transition">
            <Folder className="h-4 w-4 text-zinc-500 shrink-0" />
            <input
              type="text"
              placeholder="Workspace Path (e.g. /path/to/notes)"
              value={workspacePath}
              onChange={(e) => setWorkspacePath(e.target.value)}
              className="w-full bg-transparent border-none text-sm text-zinc-300 placeholder:text-zinc-600 focus:outline-none focus:ring-0 py-0.5"
            />
          </div>
          <Button type="submit" variant="secondary" className="h-9 font-medium px-4 text-xs bg-zinc-800 hover:bg-zinc-700 text-zinc-200">
            Set Directory
          </Button>
        </form>
      </div>

      {/* Main Body Grid */}
      <div className="flex-1 flex overflow-hidden relative">
        {/* Left Side: filters panel and canvas */}
        <div className="flex-1 flex flex-col md:flex-row overflow-hidden relative">
          
          {/* Filters Sidebar */}
          <div className="w-full md:w-64 border-r border-zinc-800/80 bg-zinc-950/20 p-4 flex flex-col gap-5 shrink-0 overflow-y-auto">
            {/* Search */}
            <div className="space-y-2">
              <Label htmlFor="search-nodes" className="text-xs font-semibold uppercase tracking-wider text-zinc-500">Search</Label>
              <div className="relative">
                <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-zinc-500" />
                <Input
                  id="search-nodes"
                  placeholder="Search nodes, tags..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="bg-zinc-900/40 border-zinc-800 text-xs pl-9 focus-visible:ring-zinc-700 h-9"
                />
              </div>
            </div>

            {/* Type Checkboxes */}
            <div className="space-y-2.5">
              <Label className="text-xs font-semibold uppercase tracking-wider text-zinc-500">Types</Label>
              <div className="space-y-2">
                {availableTypes.length === 0 ? (
                  <span className="text-xs text-zinc-600 italic">No types found.</span>
                ) : (
                  availableTypes.map((type) => (
                    <div key={type} className="flex items-center space-x-2.5">
                      <Checkbox
                        id={`type-${type}`}
                        checked={selectedTypes[type] !== false}
                        onCheckedChange={() => toggleType(type)}
                        className="border-zinc-700"
                      />
                      <label
                        htmlFor={`type-${type}`}
                        className="text-xs font-medium text-zinc-300 leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70 cursor-pointer flex items-center gap-1.5"
                      >
                        <span
                          className="h-2 w-2 rounded-full"
                          style={{
                            backgroundColor:
                              type.toLowerCase() === 'concept' ? '#10b981' :
                              type.toLowerCase() === 'feature' ? '#3b82f6' :
                              type.toLowerCase() === 'task' ? '#8b5cf6' :
                              type.toLowerCase() === 'code' ? '#f59e0b' : '#71717a'
                          }}
                        />
                        {type}
                      </label>
                    </div>
                  ))
                )}
              </div>
            </div>

            {/* Filtered Nodes List (Quick Navigation) */}
            <div className="flex-1 flex flex-col min-h-[150px]">
              <Label className="text-xs font-semibold uppercase tracking-wider text-zinc-500 mb-2">Filtered Nodes ({filteredNodes.length})</Label>
              <ScrollArea className="flex-1 rounded-lg border border-zinc-800 bg-zinc-950/40 p-2">
                <div className="space-y-1">
                  {filteredNodes.length === 0 ? (
                    <div className="text-center py-4 text-xs text-zinc-600">No matches.</div>
                  ) : (
                    filteredNodes.map(node => (
                      <button
                        key={node.id}
                        onClick={() => handleNodeSelect(node)}
                        className={`w-full text-left px-2 py-1.5 rounded-md text-xs transition duration-200 truncate flex items-center justify-between ${
                          selectedNode?.id === node.id
                            ? 'bg-zinc-800 text-zinc-100 font-semibold border-l-2 border-violet-500 pl-1.5'
                            : 'text-zinc-400 hover:bg-zinc-900/60 hover:text-zinc-200'
                        }`}
                      >
                        <span className="truncate">{node.title || node.id}</span>
                        <span className="text-[9px] text-zinc-600 px-1 py-0.5 rounded bg-zinc-900 ml-1 font-mono">{node.type || 'Note'}</span>
                      </button>
                    ))
                  )}
                </div>
              </ScrollArea>
            </div>
          </div>

          {/* Center Canvas */}
          <div className="flex-1 h-full p-4 relative flex items-center justify-center bg-zinc-950/10">
            {isLoading ? (
              <div className="text-center py-10 space-y-3">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-zinc-400 mx-auto" />
                <p className="text-sm text-zinc-500 font-medium">Loading OKF Visual Graph...</p>
              </div>
            ) : error ? (
              <div className="text-center py-10 max-w-md space-y-3">
                <div className="text-red-400 text-2xl">⚠️</div>
                <p className="text-sm text-zinc-400 font-semibold">Failed to fetch Graph data</p>
                <p className="text-xs text-zinc-600 font-mono">{(error as any).message}</p>
              </div>
            ) : (
              <VisualGraph
                nodes={filteredNodes}
                links={filteredLinks}
                selectedNodeId={selectedNode?.id}
                onNodeSelect={handleNodeSelect}
              />
            )}
          </div>
        </div>

        {/* Right Side Collapsible Sidebar (Details panel) */}
        {isSidebarOpen && selectedNode ? (
          <div className="w-80 border-l border-zinc-800 bg-[#09090b] flex flex-col h-full shrink-0 relative shadow-2xl overflow-hidden z-20 transition-all duration-300">
            
            {/* Header */}
            <div className="p-4 border-b border-zinc-800 flex items-center justify-between bg-zinc-950/60">
              <div className="flex items-center gap-2">
                <BookOpen className="h-4 w-4 text-violet-400" />
                <span className="text-xs font-semibold uppercase tracking-wider text-zinc-400">Node Explorer</span>
              </div>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setIsSidebarOpen(false)}
                className="h-7 w-7 text-zinc-500 hover:text-zinc-200"
              >
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>

            {/* Node Content */}
            <ScrollArea className="flex-1 p-4">
              <div className="space-y-5">
                
                {/* Header info */}
                <div className="space-y-1.5">
                  <Badge className={`px-2 py-0.5 text-[10px] font-semibold tracking-wide border uppercase rounded-md ${
                    selectedNode.type?.toLowerCase() === 'concept' ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' :
                    selectedNode.type?.toLowerCase() === 'feature' ? 'bg-blue-500/10 text-blue-400 border-blue-500/20' :
                    selectedNode.type?.toLowerCase() === 'task' ? 'bg-violet-500/10 text-violet-400 border-violet-500/20' :
                    selectedNode.type?.toLowerCase() === 'code' ? 'bg-amber-500/10 text-amber-400 border-amber-500/20' :
                    'bg-zinc-800 text-zinc-400'
                  }`}>
                    {selectedNode.type || 'Undefined'}
                  </Badge>
                  <h3 className="text-lg font-bold text-zinc-100 tracking-tight leading-snug">{selectedNode.title || 'Untitled Note'}</h3>
                  <p className="text-[10px] font-mono text-zinc-500 truncate mt-0.5" title={selectedNode.id}>{selectedNode.id}</p>
                </div>

                {/* Description */}
                {selectedNode.description && (
                  <div className="space-y-1 bg-zinc-900/40 p-3 rounded-lg border border-zinc-800/80">
                    <span className="text-[10px] font-semibold text-zinc-500 uppercase tracking-wider">Description</span>
                    <p className="text-xs text-zinc-300 leading-relaxed font-sans">{selectedNode.description}</p>
                  </div>
                )}

                {/* Tags array */}
                {selectedNode.tags && selectedNode.tags.length > 0 && (
                  <div className="space-y-1.5">
                    <span className="text-[10px] font-semibold text-zinc-500 uppercase tracking-wider block">Tags</span>
                    <div className="flex flex-wrap gap-1">
                      {selectedNode.tags.map((tag) => (
                        <Badge key={tag} variant="secondary" className="bg-zinc-900 text-zinc-400 border-zinc-800 text-[10px] font-normal px-1.5 py-0.5 hover:bg-zinc-800">
                          <Tag className="h-2.5 w-2.5 mr-1 shrink-0 text-zinc-600" />
                          {tag}
                        </Badge>
                      ))}
                    </div>
                  </div>
                )}

                {/* Connections (Inbound & Outbound clickable lists) */}
                <div className="space-y-4 pt-1">
                  
                  {/* Inbound */}
                  <div className="space-y-1.5">
                    <span className="text-[10px] font-semibold text-zinc-500 uppercase tracking-wider block">Inbound References</span>
                    {inboundRefs.length === 0 ? (
                      <span className="text-xs text-zinc-600 italic block pl-1">No incoming references.</span>
                    ) : (
                      <div className="space-y-1">
                        {inboundRefs.map(ref => (
                          <button
                            key={ref.id}
                            onClick={() => handleNodeSelect(ref)}
                            className="w-full text-left px-2 py-1.5 rounded bg-zinc-950/80 border border-zinc-900 text-xs text-zinc-400 hover:text-zinc-200 hover:border-zinc-800 transition duration-150 flex items-center justify-between group"
                          >
                            <span className="truncate pr-1">{ref.title || ref.id}</span>
                            <ArrowRight className="h-3 w-3 shrink-0 opacity-0 group-hover:opacity-100 text-violet-400 transition" />
                          </button>
                        ))}
                      </div>
                    )}
                  </div>

                  {/* Outbound */}
                  <div className="space-y-1.5">
                    <span className="text-[10px] font-semibold text-zinc-500 uppercase tracking-wider block">Outbound References</span>
                    {outboundRefs.length === 0 ? (
                      <span className="text-xs text-zinc-600 italic block pl-1">No outgoing references.</span>
                    ) : (
                      <div className="space-y-1">
                        {outboundRefs.map(ref => (
                          <button
                            key={ref.id}
                            onClick={() => handleNodeSelect(ref)}
                            className="w-full text-left px-2 py-1.5 rounded bg-zinc-950/80 border border-zinc-900 text-xs text-zinc-400 hover:text-zinc-200 hover:border-zinc-800 transition duration-150 flex items-center justify-between group"
                          >
                            <span className="truncate pr-1">{ref.title || ref.id}</span>
                            <ArrowRight className="h-3 w-3 shrink-0 opacity-0 group-hover:opacity-100 text-violet-400 transition" />
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                </div>

                {/* Exporters / Actions section */}
                <div className="space-y-2 pt-3 border-t border-zinc-800/80">
                  <span className="text-[10px] font-semibold text-zinc-500 uppercase tracking-wider block">Export Active Canvas</span>
                  <div className="grid grid-cols-1 gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={handleExportMermaid}
                      className="w-full justify-start text-xs border-zinc-800 text-zinc-300 hover:bg-zinc-900 h-8"
                    >
                      {copiedMermaid ? (
                        <>
                          <Check className="h-3.5 w-3.5 mr-2 text-emerald-400" />
                          <span>Copied to clipboard!</span>
                        </>
                      ) : (
                        <>
                          <Share2 className="h-3.5 w-3.5 mr-2 text-violet-400" />
                          <span>Copy Mermaid Flowchart</span>
                        </>
                      )}
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={handleExportJson}
                      className="w-full justify-start text-xs border-zinc-800 text-zinc-300 hover:bg-zinc-900 h-8"
                    >
                      <Download className="h-3.5 w-3.5 mr-2 text-blue-400" />
                      <span>Download JSON Adjacency</span>
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={handleExportCsv}
                      className="w-full justify-start text-xs border-zinc-800 text-zinc-300 hover:bg-zinc-900 h-8"
                    >
                      <FileText className="h-3.5 w-3.5 mr-2 text-emerald-400" />
                      <span>Download Nodes CSV</span>
                    </Button>
                  </div>
                </div>

              </div>
            </ScrollArea>
          </div>
        ) : selectedNode ? (
          <Button
            variant="outline"
            size="icon"
            onClick={() => setIsSidebarOpen(true)}
            className="absolute right-4 top-4 h-9 w-9 bg-zinc-950 border-zinc-800 text-zinc-400 hover:text-zinc-200 z-10 hover:bg-zinc-900 shadow-xl"
          >
            <ChevronLeft className="h-5 w-5" />
          </Button>
        ) : (
          <div className="w-80 border-l border-zinc-800 bg-[#09090b] flex flex-col items-center justify-center p-6 text-center shrink-0 hidden md:flex">
            <Settings className="h-8 w-8 text-zinc-600 animate-pulse mb-3" />
            <h4 className="text-sm font-semibold text-zinc-300">Select a Node</h4>
            <p className="text-xs text-zinc-500 max-w-[200px] mt-1.5 leading-relaxed">
              Click on any node in the interactive graph canvas to explore its metadata details and relationship linkages.
            </p>
          </div>
        )}

      </div>
    </div>
  )
}
