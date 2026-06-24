import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { VisualGraph } from '@/components/okf/VisualGraph'

// Mock react-force-graph-2d to render a test-inspectable container instead of the actual canvas engine
vi.mock('react-force-graph-2d', () => {
  return {
    default: ({ graphData, onNodeClick }: any) => {
      return (
        <div data-testid="mock-force-graph">
          <ul data-testid="nodes-list">
            {graphData.nodes.map((node: any) => (
              <li
                key={node.id}
                data-testid={`node-${node.id}`}
                onClick={() => onNodeClick && onNodeClick(node)}
              >
                {node.title} ({node.type})
              </li>
            ))}
          </ul>
          <ul data-testid="links-list">
            {graphData.links.map((link: any, idx: number) => (
              <li key={idx} data-testid={`link-${link.source}-${link.target}`}>
                {link.source} -{'>'} {link.target}
              </li>
            ))}
          </ul>
        </div>
      )
    }
  }
})

describe('VisualGraph', () => {
  const sampleNodes = [
    { id: '1', title: 'System Arch', type: 'Concept', description: 'System design' },
    { id: '2', title: 'User DB', type: 'Feature', description: 'Database setup' },
    { id: '3', title: 'Auth Service', type: 'Task', description: 'Auth setup' }
  ]

  const sampleLinks = [
    { source: '1', target: '2' },
    { source: '2', target: '3' }
  ]

  it('renders the graph canvas wrapper with nodes and links', () => {
    render(
      <VisualGraph
        nodes={sampleNodes}
        links={sampleLinks}
        onNodeSelect={vi.fn()}
      />
    )

    expect(screen.getByTestId('mock-force-graph')).toBeInTheDocument()
    expect(screen.getByTestId('nodes-list')).toBeInTheDocument()
    expect(screen.getByTestId('node-1')).toHaveTextContent('System Arch (Concept)')
    expect(screen.getByTestId('node-2')).toHaveTextContent('User DB (Feature)')
    expect(screen.getByTestId('node-3')).toHaveTextContent('Auth Service (Task)')
  })

  it('triggers onNodeSelect callback when a node is clicked', () => {
    const onNodeSelectMock = vi.fn()
    render(
      <VisualGraph
        nodes={sampleNodes}
        links={sampleLinks}
        onNodeSelect={onNodeSelectMock}
      />
    )

    const node1 = screen.getByTestId('node-1')
    fireEvent.click(node1)

    expect(onNodeSelectMock).toHaveBeenCalledTimes(1)
    expect(onNodeSelectMock).toHaveBeenCalledWith(sampleNodes[0])
  })
})
