import { useCallback, useEffect, useRef } from 'react'

declare global {
  interface Window {
    turnstile?: {
      render: (
        element: string | HTMLElement,
        options: Record<string, unknown>
      ) => string | undefined
      remove: (widgetId: string) => void
      reset: (widgetId: string) => void
    }
  }
}

interface TurnstileProps {
  siteKey: string
  onVerify: (token: string) => void
  onExpire?: () => void
  className?: string
}

let scriptLoadPromise: Promise<void> | null = null

function loadTurnstileScript(): Promise<void> {
  if (window.turnstile) return Promise.resolve()
  if (scriptLoadPromise) return scriptLoadPromise
  scriptLoadPromise = new Promise<void>((resolve, reject) => {
    const id = 'cf-turnstile'
    const existingScript = document.getElementById(id) as HTMLScriptElement | null
    if (existingScript) {
      existingScript.addEventListener('load', () => resolve(), { once: true })
      existingScript.addEventListener('error', () => reject(new Error('Failed to load Turnstile script')), { once: true })
      return
    }
    const s = document.createElement('script')
    s.id = id
    s.src =
      'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'
    s.async = true
    s.defer = true
    s.onload = () => resolve()
    s.onerror = () => reject(new Error('Failed to load Turnstile script'))
    document.head.appendChild(s)
  })
  return scriptLoadPromise
}

export function Turnstile({
  siteKey,
  onVerify,
  onExpire,
  className,
}: TurnstileProps) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const widgetIdRef = useRef<string | undefined>(undefined)
  const onVerifyRef = useRef(onVerify)
  const onExpireRef = useRef(onExpire)

  onVerifyRef.current = onVerify
  onExpireRef.current = onExpire

  const handleExpired = useCallback(() => {
    if (widgetIdRef.current) {
      window.turnstile?.reset(widgetIdRef.current)
    }
    onExpireRef.current?.()
  }, [])

  useEffect(() => {
    let cancelled = false

    loadTurnstileScript().then(() => {
      if (cancelled || !containerRef.current || !window.turnstile) return

      // Remove existing widget if already rendered
      if (widgetIdRef.current) {
        window.turnstile.remove(widgetIdRef.current)
        widgetIdRef.current = undefined
      }

      try {
        widgetIdRef.current = window.turnstile.render(containerRef.current, {
          sitekey: siteKey,
          callback: (token: string) => onVerifyRef.current(token),
          'error-callback': handleExpired,
          'expired-callback': handleExpired,
        })
      } catch (e) {
        console.warn('Turnstile render error:', e)
      }
    }).catch((error) => {
      console.warn('Turnstile script load error:', error)
      onExpireRef.current?.()
    })

    return () => {
      cancelled = true
      if (widgetIdRef.current) {
        window.turnstile?.remove(widgetIdRef.current)
        widgetIdRef.current = undefined
      }
    }
  }, [siteKey, handleExpired])

  return <div ref={containerRef} className={className} />
}
