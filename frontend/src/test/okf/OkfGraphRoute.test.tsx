import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import OkfGraphPage from '@/app/(dashboard)/okf/graph/page'
import { useQuery } from '@tanstack/react-query'

// Mock react-force-graph-2d
vi.mock('react-force-graph-2d', () => {
  return {
    default: ({ graphData, onNodeClick }: any) => {
      return (
        <div data-testid="mock-force-graph">
          <ul data-testid="graph-nodes">
            {graphData.nodes.map((node: any) => (
              <li key={node.id} onClick={() => onNodeClick && onNodeClick(node)}>
                {node.title}
              </li>
            ))}
          </ul>
        </div>
      )
    }
  }
})

// Mock useQuery from React Query
vi.mock('@tanstack/react-query', () => {
  return {
    useQuery: vi.fn(),
    useQueryClient: () => ({
      invalidateQueries: vi.fn()
    })
  }
})

describe('OkfGraphPage Route & Sidebar Navigation', () => {
  const mockGraphData = {
    nodes: [
      { id: 'nodeA.md', title: 'System Architecture', type: 'Concept', description: 'Overall system layout', tags: ['architecture', 'design'] },
      { id: 'nodeB.md', title: 'Database Setup', type: 'Feature', description: 'SurrealDB relational configuration', tags: ['db', 'setup'] },
      { id: 'nodeC.md', title: 'Auth Implementation', type: 'Task', description: 'JWT authentication layers', tags: ['security', 'auth'] }
    ],
    links: [
      { source: 'nodeA.md', target: 'nodeB.md' },
      { source: 'nodeB.md', target: 'nodeC.md' }
    ]
  }

  beforeEach(() => {
    vi.clearAllMocks()
    
    // Mock local storage
    const store: Record<string, string> = {
      'okf-workspace-path': '/test/workspace'
    }
    vi.stubGlobal('localStorage', {
      getItem: (key: string) => store[key] || null,
      setItem: (key: string, value: string) => { store[key] = value },
      clear: () => {}
    })
  })

  it('renders the graph page loading state and then successful graph layout', async () => {
    // Set useQuery to return loading state first
    vi.mocked(useQuery).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null
    } as any)

    const { rerender } = render(<OkfGraphPage />)

    expect(screen.getByText(/loading/i)).toBeInTheDocument()

    // Now return full data
    vi.mocked(useQuery).mockReturnValue({
      data: mockGraphData,
      isLoading: false,
      error: null
    } as any)

    rerender(<OkfGraphPage />)

    expect(screen.queryByText(/loading/i)).not.toBeInTheDocument()
    expect(screen.getByText('System Architecture')).toBeInTheDocument()
    expect(screen.getByPlaceholderText(/search nodes/i)).toBeInTheDocument()
    expect(screen.getByLabelText('Concept')).toBeInTheDocument()
    expect(screen.getByLabelText('Feature')).toBeInTheDocument()
  })

  it('displays node detailed metadata, inbound and outbound references in the sidebar explorer on click', async () => {
    vi.mocked(useQuery).mockReturnValue({
      data: mockGraphData,
      isLoading: false,
      error: null
    } as any)

    render(<OkfGraphPage />)

    // Initially, no node selected text should be present
    expect(screen.getByText(/select a node/i)).toBeInTheDocument()

    // Click on nodeB
    const nodeBElement = screen.getByText('Database Setup')
    fireEvent.click(nodeBElement)

    // Now details should be visible
    expect(screen.getByText('Database Setup')).toBeInTheDocument()
    expect(screen.getByText('SurrealDB relational configuration')).toBeInTheDocument()
    expect(screen.getByText('db')).toBeInTheDocument() // tag badge
    expect(screen.getByText('setup')).toBeInTheDocument() // tag badge

    // Inbound & Outbound lists
    // nodeB outbound is nodeC, inbound is nodeA
    expect(screen.getByText('Inbound References')).toBeInTheDocument()
    expect(screen.getByText('System Architecture')).toBeInTheDocument() // Inbound link

    expect(screen.getByText('Outbound References')).toBeInTheDocument()
    expect(screen.getByText('Auth Implementation')).toBeInTheDocument() // Outbound link
  })

  it('filters node lists dynamically based on search query', async () => {
    vi.mocked(useQuery).mockReturnValue({
      data: mockGraphData,
      isLoading: false,
      error: null
    } as any)

    render(<OkfGraphPage />)

    const searchInput = screen.getByPlaceholderText(/search nodes/i)
    
    // Search for "Auth"
    fireEvent.change(searchInput, { target: { value: 'Auth' } })

    // Node A and B should be hidden/absent from filtered results or list.
    // In our sidebar we can render a list of filtered nodes. Let's make sure the filtered node items list is shown.
    expect(screen.getByText('Auth Implementation')).toBeInTheDocument()
    expect(screen.queryByText('Database Setup')).not.toBeInTheDocument()
  })
})
