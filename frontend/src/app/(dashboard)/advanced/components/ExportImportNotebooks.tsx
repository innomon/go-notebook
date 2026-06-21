'use client'

import { useState } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useNotebooks } from '@/lib/hooks/use-notebooks'
import { notebooksApi } from '@/lib/api/notebooks'
import { useToast } from '@/lib/hooks/use-toast'
import { useTranslation } from '@/lib/hooks/use-translation'
import { useQueryClient } from '@tanstack/react-query'
import { QUERY_KEYS } from '@/lib/api/query-client'
import { Download, Upload, FileArchive, RefreshCw, CheckCircle2, AlertCircle } from 'lucide-react'

export function ExportImportNotebooks() {
  const { t } = useTranslation()
  const { toast } = useToast()
  const queryClient = useQueryClient()
  
  // Fetch non-archived notebooks
  const { data: notebooks, isLoading, refetch } = useNotebooks(false)

  // State
  const [exportNotebookId, setExportNotebookId] = useState<string>('')
  const [importMode, setImportMode] = useState<'new' | 'merge'>('new')
  const [mergeNotebookId, setMergeNotebookId] = useState<string>('')
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  
  // Progress & loading states
  const [isExporting, setIsExporting] = useState(false)
  const [isImporting, setIsImporting] = useState(false)

  const handleExport = async () => {
    if (!exportNotebookId) {
      toast({
        title: t('common.error'),
        description: 'Please select a notebook to export',
        variant: 'destructive',
      })
      return
    }

    const selectedNb = notebooks?.find(n => n.id === exportNotebookId)
    const nbName = selectedNb ? selectedNb.name : 'notebook'

    setIsExporting(true)
    try {
      const blob = await notebooksApi.export(exportNotebookId)
      const url = window.URL.createObjectURL(new Blob([blob]))
      const link = document.createElement('a')
      link.href = url
      link.setAttribute('download', `${nbName.replace(/\s+/g, '_')}_export.zip`)
      document.body.appendChild(link)
      link.click()
      link.parentNode?.removeChild(link)
      
      toast({
        title: t('common.success'),
        description: 'Notebook successfully exported as Obsidian vault ZIP',
      })
    } catch (error: any) {
      console.error('Export failed:', error)
      toast({
        title: t('common.error'),
        description: 'Failed to export notebook: ' + (error.message || 'Unknown error'),
        variant: 'destructive',
      })
    } finally {
      setIsExporting(false)
    }
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      setSelectedFile(e.target.files[0])
    }
  }

  const handleImport = async () => {
    if (!selectedFile) {
      toast({
        title: t('common.error'),
        description: 'Please select a zipped vault file to import',
        variant: 'destructive',
      })
      return
    }

    if (importMode === 'merge' && !mergeNotebookId) {
      toast({
        title: t('common.error'),
        description: 'Please select a target notebook to merge into',
        variant: 'destructive',
      })
      return
    }

    setIsImporting(true)
    try {
      if (importMode === 'new') {
        const newNb = await notebooksApi.importNew(selectedFile)
        toast({
          title: t('common.success'),
          description: `Successfully imported as a new notebook: "${newNb.name}"`,
        })
      } else {
        await notebooksApi.importMerge(mergeNotebookId, selectedFile)
        toast({
          title: t('common.success'),
          description: 'Successfully merged Obsidian vault into notebook',
        })
      }
      
      // Invalidate queries to refresh lists
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.notebooks })
      queryClient.invalidateQueries({ queryKey: ['sources'] })
      queryClient.invalidateQueries({ queryKey: ['notes'] })
      
      setSelectedFile(null)
      // Reset file input
      const fileInput = document.getElementById('vault-file') as HTMLInputElement
      if (fileInput) fileInput.value = ''
    } catch (error: any) {
      console.error('Import failed:', error)
      toast({
        title: t('common.error'),
        description: 'Failed to import vault: ' + (error.response?.data?.detail || error.message || 'Unknown error'),
        variant: 'destructive',
      })
    } finally {
      setIsImporting(false)
    }
  }

  return (
    <div className="grid gap-6 md:grid-cols-2">
      {/* Export Card */}
      <Card className="flex flex-col">
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="text-xl flex items-center gap-2">
                <Download className="h-5 w-5 text-indigo-600" />
                Export Obsidian Vault
              </CardTitle>
              <CardDescription className="mt-1">
                Compile and download a notebook's documents, notes, entities, and connections in an Obsidian-friendly format.
              </CardDescription>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => refetch()}
              title="Refresh notebook list"
            >
              <RefreshCw className="h-4 w-4" />
            </Button>
          </div>
        </CardHeader>
        
        <CardContent className="space-y-4 flex-1 flex flex-col justify-between">
          <div className="space-y-3">
            <Label htmlFor="export-notebook-select">Select Notebook</Label>
            {isLoading ? (
              <div className="text-sm text-muted-foreground">Loading notebooks...</div>
            ) : (
              <Select value={exportNotebookId} onValueChange={setExportNotebookId}>
                <SelectTrigger id="export-notebook-select">
                  <SelectValue placeholder="Choose a notebook to export" />
                </SelectTrigger>
                <SelectContent>
                  {notebooks && notebooks.length > 0 ? (
                    notebooks.map(nb => (
                      <SelectItem key={nb.id} value={nb.id}>
                        {nb.name}
                      </SelectItem>
                    ))
                  ) : (
                    <SelectItem value="none" disabled>No notebooks available</SelectItem>
                  )}
                </SelectContent>
              </Select>
            )}
          </div>
          
          <div className="pt-4">
            <Button
              onClick={handleExport}
              disabled={isExporting || !exportNotebookId}
              className="w-full bg-indigo-600 hover:bg-indigo-700 text-white flex items-center justify-center gap-2"
            >
              {isExporting ? 'Exporting...' : (
                <>
                  <FileArchive className="h-4 w-4" />
                  Generate and Download Vault
                </>
              )}
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Import Card */}
      <Card className="flex flex-col">
        <CardHeader>
          <CardTitle className="text-xl flex items-center gap-2">
            <Upload className="h-5 w-5 text-indigo-600" />
            Import Obsidian Vault
          </CardTitle>
          <CardDescription className="mt-1">
            Upload a zipped Obsidian vault. Markdown notes and sources will be parsed, and graph structures will be restored.
          </CardDescription>
        </CardHeader>
        
        <CardContent className="space-y-4 flex-1 flex flex-col justify-between">
          <div className="space-y-4">
            {/* Mode selection toggle */}
            <div className="space-y-2">
              <Label>Import Destination</Label>
              <div className="flex gap-4">
                <label className="flex items-center gap-2 text-sm cursor-pointer font-medium">
                  <input
                    type="radio"
                    name="importMode"
                    checked={importMode === 'new'}
                    onChange={() => setImportMode('new')}
                    className="text-indigo-600 focus:ring-indigo-500"
                  />
                  Create new notebook
                </label>
                <label className="flex items-center gap-2 text-sm cursor-pointer font-medium">
                  <input
                    type="radio"
                    name="importMode"
                    checked={importMode === 'merge'}
                    onChange={() => setImportMode('merge')}
                    className="text-indigo-600 focus:ring-indigo-500"
                  />
                  Merge into existing
                </label>
              </div>
            </div>

            {/* Merge destination dropdown if selected */}
            {importMode === 'merge' && (
              <div className="space-y-2 animate-in fade-in slide-in-from-top-1 duration-200">
                <Label htmlFor="merge-notebook-select">Target Notebook</Label>
                {isLoading ? (
                  <div className="text-sm text-muted-foreground">Loading notebooks...</div>
                ) : (
                  <Select value={mergeNotebookId} onValueChange={setMergeNotebookId}>
                    <SelectTrigger id="merge-notebook-select">
                      <SelectValue placeholder="Choose notebook to merge into" />
                    </SelectTrigger>
                    <SelectContent>
                      {notebooks && notebooks.length > 0 ? (
                        notebooks.map(nb => (
                          <SelectItem key={nb.id} value={nb.id}>
                            {nb.name}
                          </SelectItem>
                        ))
                      ) : (
                        <SelectItem value="none" disabled>No notebooks available</SelectItem>
                      )}
                    </SelectContent>
                  </Select>
                )}
              </div>
            )}

            {/* File selection */}
            <div className="space-y-2">
              <Label htmlFor="vault-file">Choose Zipped Vault (.zip)</Label>
              <Input
                id="vault-file"
                type="file"
                accept=".zip"
                onChange={handleFileChange}
                className="cursor-pointer"
              />
            </div>
          </div>

          <div className="pt-4">
            <Button
              onClick={handleImport}
              disabled={isImporting || !selectedFile || (importMode === 'merge' && !mergeNotebookId)}
              className="w-full bg-indigo-600 hover:bg-indigo-700 text-white flex items-center justify-center gap-2"
            >
              {isImporting ? 'Importing...' : (
                <>
                  <Upload className="h-4 w-4" />
                  {importMode === 'new' ? 'Import as New Notebook' : 'Merge into Notebook'}
                </>
              )}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
