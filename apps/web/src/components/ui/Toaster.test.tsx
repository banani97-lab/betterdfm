import { describe, it, expect, afterEach, vi } from 'vitest'
import { render, screen, act, fireEvent, cleanup } from '@testing-library/react'
import { Toaster } from './Toaster'
import { toast } from '@/lib/toast'

afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('Toaster', () => {
  it('renders an emitted toast', () => {
    render(<Toaster />)
    act(() => {
      toast.success('Link copied')
    })
    expect(screen.getByText('Link copied')).toBeInTheDocument()
    expect(screen.getByRole('status')).toBeInTheDocument()
  })

  it('uses role="alert" for errors', () => {
    render(<Toaster />)
    act(() => {
      toast.error("Couldn't restart analysis — server error, try again")
    })
    expect(screen.getByRole('alert')).toHaveTextContent("Couldn't restart analysis")
  })

  it('dedupes an identical emit instead of stacking', () => {
    render(<Toaster />)
    act(() => {
      toast.success('Project created')
      toast.success('Project created')
    })
    expect(screen.getAllByText('Project created')).toHaveLength(1)
  })

  it('keeps distinct messages as separate toasts', () => {
    render(<Toaster />)
    act(() => {
      toast.success('Project created')
      toast.info('Re-analysis started')
    })
    expect(screen.getByText('Project created')).toBeInTheDocument()
    expect(screen.getByText('Re-analysis started')).toBeInTheDocument()
  })

  it('dismiss button removes the toast', () => {
    render(<Toaster />)
    act(() => {
      toast.info('Re-analysis started')
    })
    fireEvent.click(screen.getByLabelText('Dismiss notification'))
    expect(screen.queryByText('Re-analysis started')).not.toBeInTheDocument()
  })

  it('drops the oldest toast beyond the max of 4', () => {
    render(<Toaster />)
    act(() => {
      toast.info('one')
      toast.info('two')
      toast.info('three')
      toast.info('four')
      toast.info('five')
    })
    expect(screen.queryByText('one')).not.toBeInTheDocument()
    expect(screen.getByText('two')).toBeInTheDocument()
    expect(screen.getByText('five')).toBeInTheDocument()
  })

  it('auto-dismisses success after 4s and errors after 8s', () => {
    vi.useFakeTimers()
    render(<Toaster />)
    act(() => {
      toast.success('Saved')
      toast.error('Broke')
    })
    act(() => {
      vi.advanceTimersByTime(4000)
    })
    expect(screen.queryByText('Saved')).not.toBeInTheDocument()
    expect(screen.getByText('Broke')).toBeInTheDocument()
    act(() => {
      vi.advanceTimersByTime(4000)
    })
    expect(screen.queryByText('Broke')).not.toBeInTheDocument()
  })

  it('a duplicate emit refreshes the auto-dismiss timer', () => {
    vi.useFakeTimers()
    render(<Toaster />)
    act(() => {
      toast.success('Saved')
    })
    act(() => {
      vi.advanceTimersByTime(3000)
    })
    // Re-emit at t=3s — timer should restart, so the toast survives t=4s...
    act(() => {
      toast.success('Saved')
    })
    act(() => {
      vi.advanceTimersByTime(3000)
    })
    expect(screen.getByText('Saved')).toBeInTheDocument()
    // ...and dismisses 4s after the refresh.
    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(screen.queryByText('Saved')).not.toBeInTheDocument()
  })
})
