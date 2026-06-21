import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ExportImportNotebooks } from './ExportImportNotebooks'
import { useNotebooks } from '@/lib/hooks/use-notebooks'
import { notebooksApi } from '@/lib/api/notebooks'
import { useToast } from '@/lib/hooks/use-toast'
import { useQueryClient } from '@tanstack/react-query'

// Mock Select component to simplify testing without Radix complexity
vi.mock('@/components/ui/select', () => {
  return {
    Select: ({ children, value, onValueChange }: any) => (
      <select
        data-testid="mock-select"
        value={value}
        onChange={(e) => onValueChange(e.target.value)}
      >
        {children}
      </select>
    ),
    SelectTrigger: ({ children, id }: any) => <div id={id}>{children}</div>,
    SelectValue: ({ placeholder }: any) => <span>{placeholder}</span>,
    SelectContent: ({ children }: any) => <>{children}</>,
    SelectItem: ({ value, children, disabled }: any) => (
      <option value={value} disabled={disabled}>
        {children}
      </option>
    ),
  }
})

// Mock useNotebooks hook
vi.mock('@/lib/hooks/use-notebooks', () => ({
  useNotebooks: vi.fn(),
}))

// Mock notebooksApi
vi.mock('@/lib/api/notebooks', () => ({
  notebooksApi: {
    export: vi.fn(),
    importNew: vi.fn(),
    importMerge: vi.fn(),
  },
}))

// Mock useToast hook
const mockToast = vi.fn()
vi.mock('@/lib/hooks/use-toast', () => ({
  useToast: () => ({
    toast: mockToast,
  }),
}))

// Mock react-query
const mockInvalidateQueries = vi.fn()
vi.mock('@tanstack/react-query', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-query')>()
  return {
    ...actual,
    useQueryClient: () => ({
      invalidateQueries: mockInvalidateQueries,
    }),
  }
})

describe('ExportImportNotebooks', () => {
  const mockNotebooks = [
    { id: 'nb-1', name: 'Science Lab', description: 'Lab logs', archived: false },
    { id: 'nb-2', name: 'History Journal', description: 'Personal entries', archived: false },
  ]

  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(useNotebooks).mockReturnValue({
      data: mockNotebooks,
      isLoading: false,
      refetch: vi.fn(),
    } as any)

    // Mock createObjectURL, revokeObjectURL, and document element triggers
    window.URL.createObjectURL = vi.fn(() => 'blob:url')
    window.URL.revokeObjectURL = vi.fn()
  })

  it('renders export and import panels correctly', () => {
    render(<ExportImportNotebooks />)

    expect(screen.getByText('Export Obsidian Vault')).toBeInTheDocument()
    expect(screen.getByText('Import Obsidian Vault')).toBeInTheDocument()
    expect(screen.getByText('Choose Zipped Vault (.zip)')).toBeInTheDocument()
  })

  it('shows loading state when fetching notebooks', () => {
    vi.mocked(useNotebooks).mockReturnValue({
      data: undefined,
      isLoading: true,
      refetch: vi.fn(),
    } as any)

    render(<ExportImportNotebooks />)

    expect(screen.getByText('Loading notebooks...')).toBeInTheDocument()
  })

  it('handles notebook export successfully', async () => {
    const mockBlob = new Blob(['zip-content'], { type: 'application/zip' })
    vi.mocked(notebooksApi.export).mockResolvedValue(mockBlob)

    render(<ExportImportNotebooks />)

    // Select notebook
    const selectElements = screen.getAllByTestId('mock-select')
    fireEvent.change(selectElements[0], { target: { value: 'nb-1' } })

    // Click export button
    const exportButton = screen.getByText('Generate and Download Vault')
    fireEvent.click(exportButton)

    await waitFor(() => {
      expect(notebooksApi.export).toHaveBeenCalledWith('nb-1')
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'common.success',
          description: 'Notebook successfully exported as Obsidian vault ZIP',
        })
      )
    })
  })

  it('handles importing new notebook successfully', async () => {
    vi.mocked(notebooksApi.importNew).mockResolvedValue({
      id: 'nb-new',
      name: 'Imported Vault',
      description: '',
      archived: false,
    } as any)

    render(<ExportImportNotebooks />)

    // Select file
    const file = new File(['dummy-content'], 'vault.zip', { type: 'application/zip' })
    const fileInput = screen.getByLabelText('Choose Zipped Vault (.zip)')
    fireEvent.change(fileInput, { target: { files: [file] } })

    // Click import button
    const importButton = screen.getByText('Import as New Notebook')
    fireEvent.click(importButton)

    await waitFor(() => {
      expect(notebooksApi.importNew).toHaveBeenCalledWith(file)
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'common.success',
          description: 'Successfully imported as a new notebook: "Imported Vault"',
        })
      )
    })
  })

  it('handles merging import into an existing notebook successfully', async () => {
    vi.mocked(notebooksApi.importMerge).mockResolvedValue({ status: 'success' } as any)

    render(<ExportImportNotebooks />)

    // Toggle to "Merge into existing"
    const mergeRadio = screen.getByLabelText('Merge into existing')
    fireEvent.click(mergeRadio)

    // Select the target notebook for merge (which is the second select element that appears)
    await waitFor(() => {
      expect(screen.getByText('Target Notebook')).toBeInTheDocument()
    })

    const selectElements = screen.getAllByTestId('mock-select')
    // The second select is the merge destination select
    fireEvent.change(selectElements[1], { target: { value: 'nb-2' } })

    // Select file
    const file = new File(['dummy-content'], 'vault.zip', { type: 'application/zip' })
    const fileInput = screen.getByLabelText('Choose Zipped Vault (.zip)')
    fireEvent.change(fileInput, { target: { files: [file] } })

    // Click merge button
    const mergeButton = screen.getByText('Merge into Notebook')
    fireEvent.click(mergeButton)

    await waitFor(() => {
      expect(notebooksApi.importMerge).toHaveBeenCalledWith('nb-2', file)
      expect(mockToast).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'common.success',
          description: 'Successfully merged Obsidian vault into notebook',
        })
      )
    })
  })
})
