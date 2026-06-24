'use client'

import { Controller, useForm, useWatch } from 'react-hook-form'
import { useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { useCreateNote, useUpdateNote, useNote } from '@/lib/hooks/use-notes'
import { QUERY_KEYS } from '@/lib/api/query-client'
import { MarkdownEditor } from '@/components/ui/markdown-editor'
import { InlineEdit } from '@/components/common/InlineEdit'
import { cn } from "@/lib/utils";
import { useTranslation } from '@/lib/hooks/use-translation'
import { PropertiesEditor } from '@/components/okf/PropertiesEditor'

const createNoteSchema = z.object({
  title: z.string().optional(),
  content: z.string().min(1, 'Content is required'),
})

type CreateNoteFormData = z.infer<typeof createNoteSchema>

interface NoteEditorDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  notebookId: string
  note?: { id: string; title: string | null; content: string | null }
}

export function NoteEditorDialog({ open, onOpenChange, notebookId, note }: NoteEditorDialogProps) {
  const { t } = useTranslation()
  const createNote = useCreateNote()
  const updateNote = useUpdateNote()
  const queryClient = useQueryClient()
  const isEditing = Boolean(note)

  // Ensure note ID has 'note:' prefix for API calls
  const noteIdWithPrefix = note?.id
    ? (note.id.includes(':') ? note.id : `note:${note.id}`)
    : ''

  const { data: fetchedNote, isLoading: noteLoading } = useNote(noteIdWithPrefix, { enabled: open && !!note?.id })
  const isSaving = isEditing ? updateNote.isPending : createNote.isPending
  const {
    handleSubmit,
    control,
    formState: { errors },
    reset,
    setValue,
  } = useForm<CreateNoteFormData>({
    resolver: zodResolver(createNoteSchema),
    defaultValues: {
      title: '',
      content: '',
    },
  })
  const watchTitle = useWatch({ control, name: 'title' })
  const [isEditorFullscreen, setIsEditorFullscreen] = useState(false)

  useEffect(() => {
    if (!open) {
      reset({ title: '', content: '' })
      return
    }

    const source = fetchedNote ?? note
    const title = source?.title ?? ''
    const content = source?.content ?? ''

    reset({ title, content })
  }, [open, note, fetchedNote, reset])

  useEffect(() => {
    if (!open) return

    const observer = new MutationObserver(() => {
      setIsEditorFullscreen(!!document.querySelector('.w-md-editor-fullscreen'))
    })
    observer.observe(document.body, { subtree: true, attributes: true, attributeFilter: ['class'] })
    return () => observer.disconnect()
  }, [open])

  const onSubmit = async (data: CreateNoteFormData) => {
    if (note) {
      await updateNote.mutateAsync({
        id: noteIdWithPrefix,
        data: {
          title: data.title || undefined,
          content: data.content,
        },
      })
      // Only invalidate notebook-specific queries if we have a notebookId
      if (notebookId) {
        queryClient.invalidateQueries({ queryKey: QUERY_KEYS.notes(notebookId) })
      }
    } else {
      // Creating a note requires a notebookId
      if (!notebookId) {
        console.error('Cannot create note without notebook_id')
        return
      }
      await createNote.mutateAsync({
        title: data.title || undefined,
        content: data.content,
        note_type: 'human',
        notebook_id: notebookId,
      })
    }
    reset()
    onOpenChange(false)
  }

  const handleClose = () => {
    reset()
    setIsEditorFullscreen(false)
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className={cn(
          "sm:max-w-3xl w-full max-h-[90vh] overflow-hidden p-0",
          isEditorFullscreen && "!max-w-screen !max-h-screen border-none w-screen h-screen"
      )}>
        <DialogTitle className="sr-only">
          {isEditing ? t('sources.editNote') : t('sources.createNote')}
        </DialogTitle>
        <form onSubmit={handleSubmit(onSubmit)} className="flex h-full flex-col min-w-0">
          {isEditing && noteLoading ? (
            <div className="flex-1 flex items-center justify-center py-10">
              <span className="text-sm text-muted-foreground">{t('common.loading')}</span>
            </div>
          ) : (
            <>
              <div className="border-b px-6 py-4">
                <InlineEdit
                  id="note-title"
                  name="title"
                  value={watchTitle ?? ''}
                  onSave={(value) => setValue('title', value || '')}
                  placeholder={t('sources.addTitle')}
                  emptyText={t('sources.untitledNote')}
                  className="text-xl font-semibold"
                  inputClassName="text-xl font-semibold"
                />
              </div>

              <div className={cn(
                  "flex-1 overflow-y-auto space-y-4",
                  !isEditorFullscreen && "px-6 py-4")
              }>
                {/* OKF Properties Panel */}
                <Controller
                  control={control}
                  name="content"
                  render={({ field }) => {
                    const contentValue = field.value || ''
                    // Split content into frontmatter and body
                    const normalized = contentValue.trimStart()
                    let frontmatter = ''
                    let body = contentValue
                    let hasFrontmatter = false

                    if (normalized.startsWith('---')) {
                      const secondDividerIndex = normalized.indexOf('---', 3)
                      if (secondDividerIndex !== -1) {
                        frontmatter = normalized.substring(3, secondDividerIndex).trim()
                        body = normalized.substring(secondDividerIndex + 3).trimStart()
                        hasFrontmatter = true
                      }
                    }

                    // Local frontmatter validation check
                    const validationErrors: string[] = []
                    if (hasFrontmatter) {
                      const lines = frontmatter.split('\n').map(l => l.trim())
                      let typeVal = ''
                      let titleVal = ''
                      let descVal = ''
                      for (const line of lines) {
                        if (line.startsWith('type:')) typeVal = line.substring(5).trim()
                        if (line.startsWith('title:')) titleVal = line.substring(6).trim()
                        if (line.startsWith('description:')) descVal = line.substring(12).trim()
                      }
                      if (!typeVal) validationErrors.push('missing mandatory OKF field: "type"')
                      if (!titleVal) validationErrors.push('missing mandatory OKF field: "title"')
                      if (!descVal) validationErrors.push('missing mandatory OKF field: "description"')
                    }

                    const handlePropertiesSave = (newYaml: string, parsedMetadata: Record<string, any>) => {
                      const newContent = `---\n${newYaml.trim()}\n---\n\n${body}`
                      field.onChange(newContent)
                      if (parsedMetadata.title) {
                        setValue('title', parsedMetadata.title)
                      }
                    }

                    const handleAddProperties = () => {
                      const initialMetadataYaml = `type: Concept\ntitle: ${watchTitle || 'Untitled Note'}\ndescription: `
                      const newContent = `---\n${initialMetadataYaml}\n---\n\n${contentValue}`
                      field.onChange(newContent)
                    }

                    const handleRemoveProperties = () => {
                      field.onChange(body)
                    }

                    return (
                      <div className="space-y-4">
                        {hasFrontmatter ? (
                          <div className="space-y-2">
                            <PropertiesEditor
                              initialYaml={frontmatter}
                              onSave={handlePropertiesSave}
                              errors={validationErrors}
                            />
                            <div className="flex justify-end">
                              <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                className="text-xs text-zinc-500 hover:text-red-400"
                                onClick={handleRemoveProperties}
                              >
                                Remove OKF Properties
                              </Button>
                            </div>
                          </div>
                        ) : (
                          <div className="rounded-xl border border-dashed border-zinc-800 bg-zinc-950/20 p-4 flex flex-col sm:flex-row items-center justify-between gap-3">
                            <div className="text-left">
                              <h4 className="text-sm font-semibold text-zinc-300">Open Knowledge Format</h4>
                              <p className="text-xs text-zinc-500 mt-0.5">Attach structured OKF frontmatter metadata tags to this research note.</p>
                            </div>
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              className="text-xs border-zinc-700 text-zinc-300 hover:bg-zinc-900"
                              onClick={handleAddProperties}
                            >
                              Add OKF Metadata
                            </Button>
                          </div>
                        )}

                        <MarkdownEditor
                          key={note?.id ?? 'new'}
                          textareaId="note-content"
                          value={body}
                          onChange={(newBody) => {
                            const newContent = hasFrontmatter
                              ? `---\n${frontmatter}\n---\n\n${newBody}`
                              : newBody
                            field.onChange(newContent)
                          }}
                          height={hasFrontmatter ? 240 : 380}
                          placeholder={t('sources.writeNotePlaceholder')}
                          className={cn(
                              "w-full h-full min-h-[240px] overflow-hidden [&_.w-md-editor]:!static [&_.w-md-editor]:!w-full [&_.w-md-editor]:!h-full [&_.w-md-editor-content]:overflow-y-auto",
                              !isEditorFullscreen && "rounded-md border"
                          )}
                        />
                      </div>
                    )
                  }}
                />
                {errors.content && (
                  <p className="text-sm text-red-600 mt-1">{errors.content.message}</p>
                )}
              </div>
            </>
          )}

          <div className="border-t px-6 py-4 flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={handleClose}>
              {t('common.cancel')}
            </Button>
            <Button
              type="submit"
              disabled={isSaving || (isEditing && noteLoading)}
            >
              {isSaving
                ? isEditing ? `${t('common.saving')}...` : `${t('common.creating')}...`
                : isEditing
                  ? t('sources.saveNote')
                  : t('sources.createNoteBtn')}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
