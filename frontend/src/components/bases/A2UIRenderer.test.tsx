import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { A2UIRenderer, A2UIResponse } from './A2UIRenderer'
import React from 'react'

describe('A2UIRenderer', () => {
  it('should render table format correctly', () => {
    const tableData: A2UIResponse = {
      type: 'table',
      columns: [
        { key: 'title', label: 'Title', type: 'string' },
        { key: 'status', label: 'Status', type: 'string' },
      ],
      rows: [
        { title: 'Note One', status: 'active', file_path: 'note:123' },
        { title: 'Note Two', status: 'completed', file_path: 'note:456' },
      ],
    }

    render(<A2UIRenderer data={tableData} />)

    expect(screen.getByText('Title')).toBeInTheDocument()
    expect(screen.getByText('Status')).toBeInTheDocument()
    expect(screen.getByText('Note One')).toBeInTheDocument()
    expect(screen.getByText('Note Two')).toBeInTheDocument()
    expect(screen.getByText('active')).toBeInTheDocument()
    expect(screen.getByText('completed')).toBeInTheDocument()
  })

  it('should trigger onNoteClick when note link is clicked', () => {
    const tableData: A2UIResponse = {
      type: 'table',
      columns: [
        { key: 'file_path', label: 'File Path', type: 'string' },
      ],
      rows: [
        { title: 'Note One', file_path: 'note:123' },
      ],
    }

    const onNoteClickMock = vi.fn()
    render(<A2UIRenderer data={tableData} onNoteClick={onNoteClickMock} />)

    const linkBtn = screen.getByText('Note One')
    fireEvent.click(linkBtn)

    expect(onNoteClickMock).toHaveBeenCalledWith('123')
  })

  it('should render card grid correctly', () => {
    const cardData: A2UIResponse = {
      type: 'card_grid',
      cards: [
        { title: 'Card One', description: 'Desc One', status: 'active', link: 'note:111' },
        { title: 'Card Two', description: 'Desc Two', status: 'pending', link: 'note:222' },
      ],
    }

    render(<A2UIRenderer data={cardData} />)

    expect(screen.getByText('Card One')).toBeInTheDocument()
    expect(screen.getByText('Desc One')).toBeInTheDocument()
    expect(screen.getByText('Card Two')).toBeInTheDocument()
    expect(screen.getByText('Desc Two')).toBeInTheDocument()
    expect(screen.getByText('active')).toBeInTheDocument()
    expect(screen.getByText('pending')).toBeInTheDocument()
  })

  it('should render lists correctly', () => {
    const listData: A2UIResponse = {
      type: 'list',
      items: [
        { title: 'Item One', description: 'Detail One', metadata: ['tag1', 'tag2'], link: 'note:333' },
      ],
    }

    render(<A2UIRenderer data={listData} />)

    expect(screen.getByText('Item One')).toBeInTheDocument()
    expect(screen.getByText('Detail One')).toBeInTheDocument()
    expect(screen.getByText('tag1')).toBeInTheDocument()
    expect(screen.getByText('tag2')).toBeInTheDocument()
  })
})
