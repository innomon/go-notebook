import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { PropertiesEditor } from '@/components/okf/PropertiesEditor'

describe('PropertiesEditor', () => {
  const defaultYaml = `type: Concept
title: Test Title
description: This is a test description
tags:
  - tag1
  - tag2`

  it('renders in Form Mode by default and displays all fields', () => {
    render(
      <PropertiesEditor
        initialYaml={defaultYaml}
        onSave={vi.fn()}
      />
    )

    // Form fields should be visible
    expect(screen.getByLabelText(/title/i)).toHaveValue('Test Title')
    expect(screen.getByLabelText(/type/i)).toHaveValue('Concept')
    expect(screen.getByLabelText(/description/i)).toHaveValue('This is a test description')
    expect(screen.getByLabelText(/tags/i)).toHaveValue('tag1, tag2')
  })

  it('toggles between Form Mode and YAML Mode', () => {
    render(
      <PropertiesEditor
        initialYaml={defaultYaml}
        onSave={vi.fn()}
      />
    )

    // Initially in Form Mode
    expect(screen.queryByPlaceholderText(/raw yaml/i)).not.toBeInTheDocument()

    // Click YAML Mode toggle
    const toggleBtn = screen.getByText(/yaml mode/i)
    fireEvent.click(toggleBtn)

    // Form fields should be hidden or replaced, YAML area should be visible
    expect(screen.getByPlaceholderText(/raw yaml/i)).toBeInTheDocument()
    expect(screen.getByPlaceholderText(/raw yaml/i)).toHaveValue(defaultYaml)

    // Toggle back to Form Mode
    const formToggleBtn = screen.getByText(/form mode/i)
    fireEvent.click(formToggleBtn)
    expect(screen.queryByPlaceholderText(/raw yaml/i)).not.toBeInTheDocument()
    expect(screen.getByLabelText(/title/i)).toBeInTheDocument()
  })

  it('displays validation errors when provided', () => {
    const testErrors = ['"type" is a mandatory field', '"description" must be at least 10 characters']
    render(
      <PropertiesEditor
        initialYaml={defaultYaml}
        onSave={vi.fn()}
        errors={testErrors}
      />
    )

    // Errors list should be rendered
    expect(screen.getByText(testErrors[0])).toBeInTheDocument()
    expect(screen.getByText(testErrors[1])).toBeInTheDocument()
    expect(screen.getByText(/invalid okf/i)).toBeInTheDocument()
  })

  it('displays "Valid OKF" badge when no errors are provided', () => {
    render(
      <PropertiesEditor
        initialYaml={defaultYaml}
        onSave={vi.fn()}
        errors={[]}
      />
    )

    expect(screen.getByText(/valid okf/i)).toBeInTheDocument()
  })

  it('calls onSave with updated YAML and metadata when form values are edited', () => {
    const onSaveMock = vi.fn()
    render(
      <PropertiesEditor
        initialYaml={defaultYaml}
        onSave={onSaveMock}
      />
    )

    const titleInput = screen.getByLabelText(/title/i)
    fireEvent.change(titleInput, { target: { value: 'New Awesome Title' } })
    
    // Trigger save (on blur/change or save button)
    // Let's implement an explicit Save button or automatic save.
    // The plan specifies: "Integrate auto-save debounced callbacks to commit changes back to note files"
    // Or we can trigger manual save if we have a save action, or blur.
    // Let's support blur for auto-save, and we can also test manual save or blur event.
    fireEvent.blur(titleInput)

    expect(onSaveMock).toHaveBeenCalled()
    const lastCall = onSaveMock.mock.calls[0]
    expect(lastCall[1].title).toBe('New Awesome Title')
    expect(lastCall[0]).toContain('title: New Awesome Title')
  })

  it('calls onSave with updated YAML and parsed metadata when YAML is edited', () => {
    const onSaveMock = vi.fn()
    render(
      <PropertiesEditor
        initialYaml={defaultYaml}
        onSave={onSaveMock}
      />
    )

    // Toggle to YAML Mode
    fireEvent.click(screen.getByText(/yaml mode/i))

    const yamlTextarea = screen.getByPlaceholderText(/raw yaml/i)
    const updatedYaml = `type: Feature
title: Brand New Feature
description: Explaining this brand new feature
tags:
  - tagA`

    fireEvent.change(yamlTextarea, { target: { value: updatedYaml } })
    fireEvent.blur(yamlTextarea)

    expect(onSaveMock).toHaveBeenCalled()
    const lastCall = onSaveMock.mock.calls[0]
    expect(lastCall[0]).toBe(updatedYaml)
    expect(lastCall[1].type).toBe('Feature')
    expect(lastCall[1].title).toBe('Brand New Feature')
  })
})
