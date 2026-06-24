'use client'

import React, { useState, useEffect, useRef } from 'react'
import { CheckCircle2, AlertTriangle, FileCode, LayoutGrid } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

interface PropertiesEditorProps {
  initialYaml: string
  onSave: (yaml: string, parsedMetadata: Record<string, any>) => void
  errors?: string[]
}

// Robust lightweight YAML parser for key-value fields and string arrays
function parseYaml(yaml: string): Record<string, any> {
  const result: Record<string, any> = {}
  const lines = yaml.split('\n')
  let currentKey: string | null = null
  let currentArray: string[] = []

  for (let line of lines) {
    const rawLine = line
    line = line.trim()
    if (!line || line.startsWith('#')) continue

    // Handle array list items
    if (line.startsWith('-')) {
      if (currentKey) {
        let val = line.substring(1).trim()
        // Strip quotes if any
        if ((val.startsWith('"') && val.endsWith('"')) || (val.startsWith("'") && val.endsWith("'"))) {
          val = val.substring(1, val.length - 1)
        }
        currentArray.push(val)
      }
      continue
    }

    // If we transition to a new key, commit the previous array if active
    if (currentKey && currentArray.length > 0) {
      result[currentKey] = currentArray
      currentArray = []
      currentKey = null
    }

    const colonIndex = line.indexOf(':')
    if (colonIndex !== -1) {
      const key = line.substring(0, colonIndex).trim()
      let val = line.substring(colonIndex + 1).trim()
      
      // Strip quotes if any
      if ((val.startsWith('"') && val.endsWith('"')) || (val.startsWith("'") && val.endsWith("'"))) {
        val = val.substring(1, val.length - 1)
      }

      if (val === '') {
        currentKey = key
      } else {
        result[key] = val
        currentKey = null
      }
    }
  }

  // Commit final array if we finished the file
  if (currentKey && currentArray.length > 0) {
    result[currentKey] = currentArray
  }

  return result
}

// Serializes a Record to a YAML frontmatter string, preserving other keys if requested
function serializeYaml(data: Record<string, any>): string {
  let yaml = ''
  
  // Print standard keys first in predefined order
  const standardKeys = ['type', 'title', 'description', 'resource', 'tags', 'timestamp', 'properties']
  const printed = new Set<string>()

  const printKey = (key: string, val: any) => {
    if (val === undefined || val === null) return
    printed.add(key)
    if (Array.isArray(val)) {
      yaml += `${key}:\n`
      for (const item of val) {
        yaml += `  - ${item}\n`
      }
    } else if (typeof val === 'object') {
      yaml += `${key}:\n`
      // Basic flat key-value printing for custom properties
      for (const [k, v] of Object.entries(val)) {
        yaml += `  ${k}: ${v}\n`
      }
    } else {
      yaml += `${key}: ${val}\n`
    }
  }

  for (const key of standardKeys) {
    if (key in data) {
      printKey(key, data[key])
    }
  }

  for (const [key, val] of Object.entries(data)) {
    if (!printed.has(key)) {
      printKey(key, val)
    }
  }

  return yaml.trim()
}

export function PropertiesEditor({ initialYaml, onSave, errors = [] }: PropertiesEditorProps) {
  const [mode, setMode] = useState<'form' | 'yaml'>('form')
  const [yamlContent, setYamlContent] = useState(initialYaml)

  // Keep track of non-form fields that were in the original YAML (e.g. resource, custom properties)
  const [allMetadata, setAllMetadata] = useState<Record<string, any>>({})

  // Form State
  const [title, setTitle] = useState('')
  const [type, setType] = useState('')
  const [description, setDescription] = useState('')
  const [tagsInput, setTagsInput] = useState('')

  // Sync state with initialYaml when it changes from props
  useEffect(() => {
    setYamlContent(initialYaml)
    try {
      const parsed = parseYaml(initialYaml)
      setAllMetadata(parsed)
      setTitle(parsed.title || '')
      setType(parsed.type || '')
      setDescription(parsed.description || '')
      setTagsInput(Array.isArray(parsed.tags) ? parsed.tags.join(', ') : '')
    } catch (e) {
      // Silence parsing errors on malformed yaml
    }
  }, [initialYaml])

  const handleFormBlur = () => {
    const tags = tagsInput
      .split(',')
      .map(t => t.trim())
      .filter(t => t.length > 0)

    const updatedMetadata = {
      ...allMetadata,
      title,
      type,
      description,
      tags
    }

    const newYaml = serializeYaml(updatedMetadata)
    setYamlContent(newYaml)
    setAllMetadata(updatedMetadata)
    onSave(newYaml, updatedMetadata)
  }

  const handleYamlBlur = () => {
    try {
      const parsed = parseYaml(yamlContent)
      setAllMetadata(parsed)
      setTitle(parsed.title || '')
      setType(parsed.type || '')
      setDescription(parsed.description || '')
      setTagsInput(Array.isArray(parsed.tags) ? parsed.tags.join(', ') : '')
      onSave(yamlContent, parsed)
    } catch (e) {
      // Even if invalid, still trigger save with the raw text so we can get validation feedback from backend
      onSave(yamlContent, {})
    }
  }

  const isValid = errors.length === 0

  return (
    <div className="w-full rounded-xl border border-zinc-800 bg-zinc-950/60 p-5 shadow-2xl backdrop-blur-md transition-all duration-300">
      {/* Header with status badge and toggles */}
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-zinc-800/80 pb-4 mb-4">
        <div className="flex items-center gap-3">
          <span className="text-sm font-semibold tracking-wide text-zinc-300">Properties</span>
          {isValid ? (
            <Badge variant="secondary" className="flex items-center gap-1.5 bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 px-2 py-0.5 text-xs font-medium">
              <CheckCircle2 className="h-3 w-3" />
              Valid OKF
            </Badge>
          ) : (
            <Badge variant="destructive" className="flex items-center gap-1.5 bg-amber-500/10 text-amber-400 border border-amber-500/20 px-2 py-0.5 text-xs font-medium">
              <AlertTriangle className="h-3 w-3" />
              Invalid OKF
            </Badge>
          )}
        </div>

        <div className="flex items-center gap-1.5 rounded-lg border border-zinc-800 bg-zinc-900/50 p-1">
          <Button
            size="sm"
            variant={mode === 'form' ? 'secondary' : 'ghost'}
            className={`flex items-center gap-1.5 px-3 py-1 text-xs h-7 rounded-md ${
              mode === 'form' ? 'bg-zinc-800 text-zinc-100 font-medium' : 'text-zinc-400 hover:text-zinc-200'
            }`}
            onClick={() => setMode('form')}
          >
            <LayoutGrid className="h-3.5 w-3.5" />
            Form Mode
          </Button>
          <Button
            size="sm"
            variant={mode === 'yaml' ? 'secondary' : 'ghost'}
            className={`flex items-center gap-1.5 px-3 py-1 text-xs h-7 rounded-md ${
              mode === 'yaml' ? 'bg-zinc-800 text-zinc-100 font-medium' : 'text-zinc-400 hover:text-zinc-200'
            }`}
            onClick={() => setMode('yaml')}
          >
            <FileCode className="h-3.5 w-3.5" />
            YAML Mode
          </Button>
        </div>
      </div>

      {/* Validation Errors Panel */}
      {!isValid && (
        <div className="mb-4 rounded-lg border border-amber-500/20 bg-amber-500/5 p-3 text-xs text-amber-400">
          <div className="flex items-center gap-2 font-semibold mb-1">
            <AlertTriangle className="h-4 w-4 shrink-0" />
            <span>Compliance Errors</span>
          </div>
          <ul className="list-disc pl-5 space-y-1 mt-1 text-amber-300/90 font-mono leading-relaxed">
            {errors.map((err, idx) => (
              <li key={idx}>{err}</li>
            ))}
          </ul>
        </div>
      )}

      {/* Editors */}
      {mode === 'form' ? (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="space-y-1.5">
            <Label htmlFor="okf-title" className="text-xs font-semibold text-zinc-400">Title</Label>
            <Input
              id="okf-title"
              value={title}
              onChange={e => setTitle(e.target.value)}
              onBlur={handleFormBlur}
              placeholder="e.g. System Design Document"
              className="bg-zinc-900/40 border-zinc-800 text-zinc-100 placeholder:text-zinc-600 focus-visible:ring-zinc-700 h-9"
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="okf-type" className="text-xs font-semibold text-zinc-400">Type</Label>
            <Input
              id="okf-type"
              value={type}
              onChange={e => setType(e.target.value)}
              onBlur={handleFormBlur}
              placeholder="e.g. Concept, Feature, Task, Code"
              className="bg-zinc-900/40 border-zinc-800 text-zinc-100 placeholder:text-zinc-600 focus-visible:ring-zinc-700 h-9"
            />
          </div>

          <div className="space-y-1.5 md:col-span-2">
            <Label htmlFor="okf-description" className="text-xs font-semibold text-zinc-400">Description</Label>
            <Textarea
              id="okf-description"
              value={description}
              onChange={e => setDescription(e.target.value)}
              onBlur={handleFormBlur}
              placeholder="Provide a concise description of the purpose of this note..."
              className="bg-zinc-900/40 border-zinc-800 text-zinc-100 placeholder:text-zinc-600 focus-visible:ring-zinc-700 min-h-[60px] max-h-[120px]"
            />
          </div>

          <div className="space-y-1.5 md:col-span-2">
            <Label htmlFor="okf-tags" className="text-xs font-semibold text-zinc-400">Tags</Label>
            <Input
              id="okf-tags"
              value={tagsInput}
              onChange={e => setTagsInput(e.target.value)}
              onBlur={handleFormBlur}
              placeholder="comma-separated tags: e.g. graphrag, database, setup"
              className="bg-zinc-900/40 border-zinc-800 text-zinc-100 placeholder:text-zinc-600 focus-visible:ring-zinc-700 h-9"
            />
          </div>
        </div>
      ) : (
        <div className="space-y-2">
          <Label htmlFor="okf-raw-yaml" className="text-xs font-semibold text-zinc-400">Raw YAML Editor</Label>
          <textarea
            id="okf-raw-yaml"
            placeholder="Raw YAML content..."
            value={yamlContent}
            onChange={e => setYamlContent(e.target.value)}
            onBlur={handleYamlBlur}
            className="w-full font-mono text-sm leading-relaxed p-4 rounded-lg bg-zinc-950/80 border border-zinc-800 text-zinc-300 focus:outline-none focus:border-zinc-700 min-h-[180px] max-h-[300px]"
          />
        </div>
      )}
    </div>
  )
}
