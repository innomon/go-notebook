'use client'

import React, { useState, useEffect, useMemo, useRef } from 'react'
import ForceGraph2D from 'react-force-graph-2d'

export interface GraphNode {
  id: string
  title: string
  type: string
  description?: string
  tags?: string[]
}

export interface GraphLink {
  source: string
  target: string
}

interface VisualGraphProps {
  nodes: GraphNode[]
  links: GraphLink[]
  selectedNodeId?: string | null
  onNodeSelect?: (node: GraphNode) => void
}

// Map OKF types to Zinc theme colors
const TYPE_COLORS: Record<string, string> = {
  concept: '#10b981', // Emerald
  feature: '#3b82f6', // Blue
  task: '#8b5cf6',    // Violet
  code: '#f59e0b',    // Amber
  issue: '#ef4444',   // Red
  default: '#71717a'  // Slate
}

export function VisualGraph({ nodes, links, selectedNodeId, onNodeSelect }: VisualGraphProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const graphRef = useRef<any>(null)
  const [dimensions, setDimensions] = useState({ width: 600, height: 400 })
  const [hoverNode, setHoverNode] = useState<any | null>(null)

  // Track resizing of the parent container to fill the space beautifully
  useEffect(() => {
    if (!containerRef.current) return

    const handleResize = () => {
      if (containerRef.current) {
        setDimensions({
          width: containerRef.current.clientWidth,
          height: containerRef.current.clientHeight || 500
        })
      }
    }

    handleResize()
    window.addEventListener('resize', handleResize)
    
    let observer: ResizeObserver | null = null
    if (typeof ResizeObserver !== 'undefined') {
      observer = new ResizeObserver(handleResize)
      observer.observe(containerRef.current)
    }

    return () => {
      window.removeEventListener('resize', handleResize)
      if (observer) {
        observer.disconnect()
      }
    }
  }, [])

  // Map raw node objects to d3-force ready objects (making a deep copy to prevent mutation bugs)
  const graphData = useMemo(() => {
    const nodesCopy = nodes.map(n => {
      const typeLower = (n.type || '').toLowerCase()
      const color = TYPE_COLORS[typeLower] || TYPE_COLORS.default
      return {
        ...n,
        color,
        val: 1
      }
    })

    const linksCopy = links.map(l => ({
      source: typeof l.source === 'object' ? (l.source as any).id : l.source,
      target: typeof l.target === 'object' ? (l.target as any).id : l.target
    }))

    return {
      nodes: nodesCopy,
      links: linksCopy
    }
  }, [nodes, links])

  // Map immediate links to quickly query neighbors
  const adjacencyMap = useMemo(() => {
    const map = new Map<string, Set<string>>()
    nodes.forEach(n => map.set(n.id, new Set<string>()))
    links.forEach(l => {
      const s = typeof l.source === 'object' ? (l.source as any).id : l.source
      const t = typeof l.target === 'object' ? (l.target as any).id : l.target
      if (map.has(s)) map.get(s)!.add(t)
      if (map.has(t)) map.get(t)!.add(s)
    })
    return map
  }, [nodes, links])

  // Center and zoom fit on data changes
  useEffect(() => {
    if (graphRef.current && graphData.nodes.length > 0) {
      // Small timeout to allow container layouts to stabilize
      setTimeout(() => {
        graphRef.current.zoomToFit(400, 40)
      }, 100)
    }
  }, [graphData])

  // Render logic for custom styling of nodes (Circular nodes with outer ring highlights & text labels)
  const drawNode = (node: any, ctx: CanvasRenderingContext2D, globalScale: number) => {
    const isSelected = selectedNodeId === node.id
    const isHovered = hoverNode?.id === node.id
    const isNeighbor = hoverNode ? adjacencyMap.get(hoverNode.id)?.has(node.id) : false

    // Determine opacity/dimming
    let alpha = 1.0
    if (hoverNode && !isHovered && !isNeighbor) {
      alpha = 0.15
    } else if (selectedNodeId && !isSelected && !adjacencyMap.get(selectedNodeId)?.has(node.id)) {
      alpha = 0.3
    }

    ctx.save()
    ctx.globalAlpha = alpha

    const r = isSelected ? 8 : isHovered ? 7 : 5

    // Draw selection/hover outer glowing halo
    if (isSelected || isHovered) {
      ctx.beginPath()
      ctx.arc(node.x, node.y, r + 4, 0, 2 * Math.PI, false)
      ctx.fillStyle = isSelected ? 'rgba(139, 92, 246, 0.15)' : 'rgba(255, 255, 255, 0.08)'
      ctx.fill()
      ctx.lineWidth = 1.5 / globalScale
      ctx.strokeStyle = isSelected ? '#8b5cf6' : '#a1a1aa'
      ctx.stroke()
    }

    // Draw main circular node
    ctx.beginPath()
    ctx.arc(node.x, node.y, r, 0, 2 * Math.PI, false)
    ctx.fillStyle = node.color
    ctx.fill()

    // Add border to node
    ctx.lineWidth = 1.2 / globalScale
    ctx.strokeStyle = '#09090b'
    ctx.stroke()

    // Render node label (Only show labels above a certain scale or if selected/hovered to keep view clean)
    if (globalScale > 1.2 || isSelected || isHovered || isNeighbor) {
      const fontSize = Math.max(10 / globalScale, 9)
      ctx.font = `${isSelected ? 'bold' : 'normal'} ${fontSize}px sans-serif`
      ctx.textAlign = 'center'
      ctx.textBaseline = 'top'

      // Light backdrop text shadow for maximum readability
      ctx.fillStyle = '#09090b'
      ctx.fillText(node.title || node.id, node.x + 0.5, node.y + r + 4 + 0.5)

      ctx.fillStyle = isSelected ? '#ffffff' : isHovered ? '#f4f4f5' : '#a1a1aa'
      ctx.fillText(node.title || node.id, node.x, node.y + r + 4)
    }

    ctx.restore()
  }

  // Render logic for custom link styling (Directed paths with arrow indicators)
  const drawLink = (link: any, ctx: CanvasRenderingContext2D) => {
    const isHovered = hoverNode?.id === link.source.id || hoverNode?.id === link.target.id
    const isSelected = selectedNodeId === link.source.id || selectedNodeId === link.target.id

    let alpha = 0.25
    let width = 1.0

    if (hoverNode) {
      alpha = isHovered ? 0.8 : 0.05
      width = isHovered ? 1.8 : 1.0
    } else if (selectedNodeId) {
      alpha = isSelected ? 0.6 : 0.1
      width = isSelected ? 1.5 : 1.0
    }

    ctx.save()
    ctx.globalAlpha = alpha
    ctx.lineWidth = width
    ctx.strokeStyle = isHovered ? '#a1a1aa' : isSelected ? '#8b5cf6' : '#27272a'

    // Draw line
    ctx.beginPath()
    ctx.moveTo(link.source.x, link.source.y)
    ctx.lineTo(link.target.x, link.target.y)
    ctx.stroke()

    // Draw small directional arrowhead mid-line
    const arrowLength = 5
    const arrowWidth = 3
    const sX = link.source.x
    const sY = link.source.y
    const tX = link.target.x
    const tY = link.target.y

    // Find mid point
    const mX = (sX + tX) / 2
    const mY = (sY + tY) / 2

    // Direction vector
    const dx = tX - sX
    const dy = tY - sY
    const len = Math.sqrt(dx * dx + dy * dy)

    if (len > 0) {
      const ux = dx / len
      const uy = dy / len

      // Arrow path
      ctx.beginPath()
      ctx.moveTo(mX, mY)
      ctx.lineTo(
        mX - ux * arrowLength + uy * arrowWidth,
        mY - uy * arrowLength - ux * arrowWidth
      )
      ctx.lineTo(
        mX - ux * arrowLength - uy * arrowWidth,
        mY - uy * arrowLength + ux * arrowWidth
      )
      ctx.closePath()
      ctx.fillStyle = isHovered ? '#a1a1aa' : isSelected ? '#8b5cf6' : '#27272a'
      ctx.fill()
    }

    ctx.restore()
  }

  return (
    <div
      ref={containerRef}
      className="relative w-full h-full min-h-[400px] rounded-xl border border-zinc-800 bg-[#09090b] overflow-hidden shadow-inner flex items-center justify-center"
    >
      {graphData.nodes.length === 0 ? (
        <div className="text-zinc-500 text-sm font-medium flex flex-col items-center gap-2">
          <span>No connected nodes found in this directory.</span>
          <span className="text-xs text-zinc-600">Ensure your workspace path contains OKF markdown files.</span>
        </div>
      ) : (
        <ForceGraph2D
          ref={graphRef}
          graphData={graphData}
          width={dimensions.width}
          height={dimensions.height}
          backgroundColor="#09090b"
          nodeRelVal={5}
          nodeCanvasObject={drawNode}
          linkCanvasObject={drawLink}
          onNodeClick={(node) => onNodeSelect && onNodeSelect(node as any)}
          onNodeHover={(node) => setHoverNode(node)}
          onNodeDragEnd={(node) => {
            // Keep dragged nodes locked in their physics coordinates
            node.fx = node.x
            node.fy = node.y
          }}
          enableNodeDrag={true}
          enablePanInteraction={true}
          enableZoomInteraction={true}
          cooldownTicks={120}
        />
      )}
    </div>
  )
}
