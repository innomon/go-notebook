'use client'

import React, { useState } from 'react'
import { AppShell } from '@/components/layout/AppShell'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Label } from '@/components/ui/label'
import { useTranslation } from '@/lib/hooks/use-translation'
import { useNotebooks } from '@/lib/hooks/use-notebooks'
import { useBasesPlugins, useUpdatePluginPermissions, useEvaluateBase } from '@/lib/hooks/use-bases'
import { A2UIRenderer, A2UIResponse } from '@/components/bases/A2UIRenderer'
import { NoteEditorDialog } from '@/app/(dashboard)/notebooks/components/NoteEditorDialog'
import { LayoutGrid, ShieldAlert, Play, RefreshCw, Layers } from 'lucide-react'
// @ts-ignore
import jsyaml from 'js-yaml'

const DEFAULT_CONFIG_YAML = `view_type: table
filters:
  - property: status
    operator: eq
    value: active
formulas:
  age_days: calculate_days_since`

export default function BasesPage() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState('evaluate')
  const [selectedNotebookId, setSelectedNotebookId] = useState<string>('all')
  const [configYaml, setConfigYaml] = useState(DEFAULT_CONFIG_YAML)
  const [evalResult, setEvalResult] = useState<A2UIResponse | null>(null)
  const [selectedNoteId, setSelectedNoteId] = useState<string | null>(null)

  // Fetch notebooks and plugins
  const { data: notebooks } = useNotebooks()
  const { data: plugins, isLoading: pluginsLoading, refetch: refetchPlugins } = useBasesPlugins()

  // Mutations
  const updatePermissions = useUpdatePluginPermissions()
  const evaluateBase = useEvaluateBase()

  const handleEvaluate = async () => {
    try {
      // Parse YAML config client-side to JSON before sending
      const parsedConfig = jsyaml.load(configYaml)
      const notebookParam = selectedNotebookId === 'all' ? '' : selectedNotebookId

      const result = await evaluateBase.mutateAsync({
        notebookId: notebookParam,
        config: parsedConfig,
      })
      setEvalResult(result)
    } catch (err) {
      console.error('Evaluation failed:', err)
    }
  }

  const handleTogglePermission = (pluginName: string, field: 'read_other_notes' | 'access_env', checked: boolean) => {
    const existingPlugin = plugins?.find((p) => p.name === pluginName)
    if (!existingPlugin) return

    updatePermissions.mutate({
      name: pluginName,
      read_other_notes: field === 'read_other_notes' ? checked : existingPlugin.read_other_notes,
      access_env: field === 'access_env' ? checked : existingPlugin.access_env,
    })
  }

  return (
    <AppShell>
      <div className="flex-1 overflow-y-auto">
        <div className="p-6 space-y-6 max-w-6xl mx-auto">
          {/* Header */}
          <div className="flex flex-col gap-2">
            <h1 className="text-3xl font-extrabold tracking-tight bg-gradient-to-r from-primary to-violet-500 bg-clip-text text-transparent">
              Obsidian Bases
            </h1>
            <p className="text-muted-foreground text-sm max-w-2xl">
              Compile notes dynamically into structured database views using WASM guest extensions and permission-restricted host sandboxes.
            </p>
          </div>

          {/* Controls Panel */}
          <div className="flex flex-col sm:flex-row gap-4 items-end bg-card p-4 rounded-lg border shadow-sm">
            <div className="flex-1 space-y-1.5 w-full">
              <Label htmlFor="notebook-select" className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Select Notes Context
              </Label>
              <Select value={selectedNotebookId} onValueChange={setSelectedNotebookId}>
                <SelectTrigger id="notebook-select" className="w-full">
                  <SelectValue placeholder="All Notebooks" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Notebooks</SelectItem>
                  {notebooks?.map((nb) => (
                    <SelectItem key={nb.id} value={nb.id}>
                      {nb.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <Button
              className="w-full sm:w-auto bg-gradient-to-r from-primary to-indigo-600 text-white font-medium hover:opacity-90 transition-opacity"
              disabled={evaluateBase.isPending}
              onClick={handleEvaluate}
            >
              {evaluateBase.isPending ? (
                <>
                  <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                  Evaluating...
                </>
              ) : (
                <>
                  <Play className="h-4 w-4 mr-2 fill-current" />
                  Evaluate Base
                </>
              )}
            </Button>
          </div>

          {/* Tabbed Workspace */}
          <Tabs value={activeTab} onValueChange={setActiveTab} className="space-y-6">
            <TabsList className="bg-muted p-1 border rounded-lg max-w-md">
              <TabsTrigger value="evaluate" className="flex items-center gap-2">
                <LayoutGrid className="h-4 w-4" />
                Evaluate View
              </TabsTrigger>
              <TabsTrigger value="permissions" className="flex items-center gap-2">
                <ShieldAlert className="h-4 w-4" />
                Permissions & Plugins
              </TabsTrigger>
            </TabsList>

            {/* Evaluate Tab */}
            <TabsContent value="evaluate" className="space-y-6">
              <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 items-start">
                {/* Configuration Editor Card */}
                <Card className="lg:col-span-1 border shadow-sm">
                  <CardHeader className="pb-3 border-b">
                    <CardTitle className="text-sm font-semibold flex items-center gap-2">
                      <Layers className="h-4 w-4 text-primary" />
                      Base Configuration (.base)
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="pt-4">
                    <textarea
                      className="w-full h-72 p-3 font-mono text-xs border rounded-md bg-muted/40 focus:bg-background focus:outline-none focus:ring-1 focus:ring-primary resize-y"
                      value={configYaml}
                      onChange={(e) => setConfigYaml(e.target.value)}
                      placeholder="Enter base YAML config..."
                    />
                  </CardContent>
                </Card>

                {/* Render Result Card */}
                <div className="lg:col-span-2 space-y-4">
                  <h3 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground px-1">
                    Structured UI Output (A2UI Renderer)
                  </h3>
                  {evalResult ? (
                    <A2UIRenderer data={evalResult} onNoteClick={setSelectedNoteId} />
                  ) : (
                    <div className="flex flex-col items-center justify-center p-12 border border-dashed rounded-lg bg-card text-center text-muted-foreground min-h-[300px]">
                      <LayoutGrid className="h-10 w-10 text-muted-foreground/50 mb-3" />
                      <p className="font-medium text-sm">No evaluated view output yet.</p>
                      <p className="text-xs text-muted-foreground max-w-xs mt-1">
                        Select a notebook context, verify the YAML config, and click &quot;Evaluate Base&quot; to compile notes.
                      </p>
                    </div>
                  )}
                </div>
              </div>
            </TabsContent>

            {/* Permissions Tab */}
            <TabsContent value="permissions" className="space-y-6">
              <Card className="border shadow-sm">
                <CardHeader className="border-b pb-4 flex flex-row items-center justify-between">
                  <div>
                    <CardTitle className="text-lg">Plugin Access Control</CardTitle>
                    <CardDescription className="text-xs mt-1">
                      Manage sandbox permissions for compiled WASM guest libraries.
                    </CardDescription>
                  </div>
                  <Button variant="outline" size="icon" onClick={() => refetchPlugins()}>
                    <RefreshCw className="h-4 w-4" />
                  </Button>
                </CardHeader>
                <CardContent className="divide-y p-0">
                  {pluginsLoading ? (
                    <div className="p-8 text-center text-muted-foreground text-sm">
                      <RefreshCw className="h-5 w-5 animate-spin mx-auto mb-2 text-primary" />
                      Loading plugin configurations...
                    </div>
                  ) : plugins && plugins.length > 0 ? (
                    plugins.map((plugin) => (
                      <div key={plugin.name} className="flex flex-col md:flex-row md:items-center justify-between p-4 md:p-6 gap-4">
                        <div className="space-y-1">
                          <div className="flex items-center gap-2">
                            <span className="font-bold text-sm text-foreground">{plugin.name}</span>
                            <Badge variant="secondary" className="text-[10px] font-normal font-mono px-1.5">
                              active
                            </Badge>
                          </div>
                          <p className="text-xs text-muted-foreground">
                            Compiled WASM Reactor guest extension.
                          </p>
                        </div>
                        <div className="flex flex-row gap-6 items-center shrink-0">
                          {/* Read Other Notes Toggle */}
                          <div className="flex items-center gap-2 bg-muted/40 p-2.5 rounded border">
                            <Checkbox
                              id={`read-${plugin.name}`}
                              checked={plugin.read_other_notes}
                              onCheckedChange={(checked) =>
                                handleTogglePermission(plugin.name, 'read_other_notes', !!checked)
                              }
                            />
                            <Label htmlFor={`read-${plugin.name}`} className="text-xs font-semibold cursor-pointer select-none">
                              Read Notes
                            </Label>
                          </div>

                          {/* Access Env Toggle */}
                          <div className="flex items-center gap-2 bg-muted/40 p-2.5 rounded border">
                            <Checkbox
                              id={`env-${plugin.name}`}
                              checked={plugin.access_env}
                              onCheckedChange={(checked) =>
                                handleTogglePermission(plugin.name, 'access_env', !!checked)
                              }
                            />
                            <Label htmlFor={`env-${plugin.name}`} className="text-xs font-semibold cursor-pointer select-none">
                              Access Env
                            </Label>
                          </div>
                        </div>
                      </div>
                    ))
                  ) : (
                    <div className="p-8 text-center text-muted-foreground text-sm">
                      No compiled plugins found in extensions directory.
                    </div>
                  )}
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>

          {/* Modal Editor Dialog for Note Open */}
          {selectedNoteId && (
            <NoteEditorDialog
              open={!!selectedNoteId}
              onOpenChange={(open) => !open && setSelectedNoteId(null)}
              notebookId={selectedNotebookId === 'all' ? '' : selectedNotebookId}
              note={{ id: selectedNoteId, title: '', content: '' }}
            />
          )}
        </div>
      </div>
    </AppShell>
  )
}
