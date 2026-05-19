import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { hello } from '@/features/hello/api'

export const Route = createFileRoute('/')({
  component: Home,
})

function Home() {
  const [name, setName] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [pending, setPending] = useState(false)

  async function onClick() {
    setPending(true)
    try {
      setMessage(await hello(name))
    } finally {
      setPending(false)
    }
  }

  return (
    <main className="mx-auto flex max-w-xl flex-col gap-6 p-12">
      <h1 className="text-3xl font-semibold tracking-tight">self-checkout-pos</h1>
      <p className="text-muted-foreground">
        Wails + React + TanStack Router smoke test. Click below to ping the Go backend.
      </p>

      <div className="flex gap-2">
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="Your name (optional)"
          className="flex-1 rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        />
        <Button onClick={onClick} disabled={pending}>
          {pending ? 'Calling...' : 'Say Hello'}
        </Button>
      </div>

      {message ? (
        <pre className="rounded-md bg-muted p-4 text-sm whitespace-pre-wrap break-words">{message}</pre>
      ) : null}
    </main>
  )
}
