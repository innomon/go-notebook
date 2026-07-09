import React from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { FileText, ArrowUpRight } from 'lucide-react'

export interface A2UIResponse {
  type: 'table' | 'card_grid' | 'list'
  columns?: Array<{ key: string; label: string; type: string }>
  rows?: Array<Record<string, any>>
  cards?: Array<{
    title: string
    description?: string
    status?: string
    link?: string
    [key: string]: any
  }>
  items?: Array<{
    title: string
    description?: string
    metadata?: string[]
    link?: string
    [key: string]: any
  }>
}

interface A2UIRendererProps {
  data: A2UIResponse
  onNoteClick?: (noteId: string) => void
}

export function A2UIRenderer({ data, onNoteClick }: A2UIRendererProps) {
  if (!data) return null

  const handleLink = (link?: string) => {
    if (!link) return
    if (link.startsWith('note:')) {
      const noteId = link.replace('note:', '')
      onNoteClick?.(noteId)
    } else if (link.startsWith('http://') || link.startsWith('https://')) {
      window.open(link, '_blank', 'noopener,noreferrer')
    }
  }

  const renderCell = (key: string, value: any, row: Record<string, any>) => {
    if (value === null || value === undefined) return <span className="text-muted-foreground">-</span>

    // If it's a file path/link pointing to a note
    if (key === 'file_path' && typeof value === 'string' && value.startsWith('note:')) {
      return (
        <Button
          variant="link"
          className="h-auto p-0 flex items-center gap-1 text-primary hover:underline font-medium text-left"
          onClick={() => handleLink(value)}
        >
          <FileText className="h-3.5 w-3.5 shrink-0" />
          <span className="truncate">{row.title || value.replace('note:', '')}</span>
        </Button>
      )
    }

    // Format boolean
    if (typeof value === 'boolean') {
      return (
        <Badge variant={value ? 'default' : 'secondary'}>
          {value ? 'true' : 'false'}
        </Badge>
      )
    }

    // Format other values
    return String(value)
  }

  if (data.type === 'table' && data.columns && data.rows) {
    return (
      <div className="border rounded-md overflow-hidden bg-card text-card-foreground shadow-sm">
        <div className="overflow-x-auto">
          <table className="w-full text-sm text-left border-collapse">
            <thead>
              <tr className="border-b bg-muted/50 font-medium text-muted-foreground">
                {data.columns.map((col) => (
                  <th key={col.key} className="p-3 font-semibold select-none">
                    {col.label}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y">
              {data.rows.map((row, idx) => (
                <tr key={idx} className="hover:bg-muted/30 transition-colors">
                  {data.columns!.map((col) => (
                    <td key={col.key} className="p-3 max-w-xs truncate">
                      {renderCell(col.key, row[col.key], row)}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    )
  }

  if (data.type === 'card_grid' && data.cards) {
    return (
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {data.cards.map((card, idx) => (
          <Card 
            key={idx} 
            className={`flex flex-col h-full ${card.link ? 'cursor-pointer hover:border-primary/50 transition-all hover:shadow-md' : ''}`}
            onClick={() => card.link && handleLink(card.link)}
          >
            <CardHeader className="pb-2">
              <div className="flex items-start justify-between gap-2">
                <CardTitle className="text-md font-bold leading-tight line-clamp-1">{card.title}</CardTitle>
                {card.status && (
                  <Badge variant="outline" className="text-xs uppercase shrink-0">
                    {card.status}
                  </Badge>
                )}
              </div>
              {card.description && (
                <CardDescription className="line-clamp-2 mt-1">{card.description}</CardDescription>
              )}
            </CardHeader>
            <CardContent className="mt-auto flex justify-between items-center text-xs text-muted-foreground pt-4">
              {card.link && (
                <span className="flex items-center gap-1 text-primary hover:underline font-medium ml-auto">
                  View Note <ArrowUpRight className="h-3 w-3" />
                </span>
              )}
            </CardContent>
          </Card>
        ))}
      </div>
    )
  }

  if (data.type === 'list' && data.items) {
    return (
      <div className="space-y-4">
        {data.items.map((item, idx) => (
          <div
            key={idx}
            className={`p-4 border rounded-md bg-card hover:bg-muted/20 transition-colors flex items-center justify-between gap-4 ${
              item.link ? 'cursor-pointer hover:border-primary/30' : ''
            }`}
            onClick={() => item.link && handleLink(item.link)}
          >
            <div className="space-y-1">
              <div className="font-semibold text-sm flex items-center gap-2">
                {item.title}
                {item.metadata?.map((meta, mIdx) => (
                  <Badge key={mIdx} variant="secondary" className="text-[10px] py-0 px-1.5 font-normal">
                    {meta}
                  </Badge>
                ))}
              </div>
              {item.description && (
                <p className="text-xs text-muted-foreground line-clamp-1">{item.description}</p>
              )}
            </div>
            {item.link && (
              <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-primary">
                <ArrowUpRight className="h-4 w-4" />
              </Button>
            )}
          </div>
        ))}
      </div>
    )
  }

  return (
    <div className="p-4 text-sm text-muted-foreground text-center border border-dashed rounded-md bg-muted/10">
      No data available to render.
    </div>
  )
}
