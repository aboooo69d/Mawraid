/**
 * Same-origin proxy for the Go backend's `/api/v1` surface.
 *
 * STATUS (Phase 1 cleanup): this route is RETAINED by project decision — it is
 * NOT deleted and its runtime behavior is unchanged — but it is NO LONGER used
 * by authentication. The auth module (lib/auth/api.ts) calls the backend
 * directly through `NEXT_PUBLIC_API_BASE_URL` with `credentials: 'include'`, so
 * the backend-managed HttpOnly `session` cookie flows straight from the
 * browser to the Go backend. This proxy deliberately does NOT forward cookies
 * (see the header allow-list below) and therefore CANNOT carry an
 * authenticated session — never route authentication through it.
 *
 * WHY IT STILL EXISTS
 *  - The backend base URL for server-side code is a server-only configuration
 *    (`API_BASE_URL`, no `NEXT_PUBLIC_` prefix — see .env.example), consumed
 *    directly by lib/api/client.ts. This route remains available should a
 *    future server-side feature need to relay `/api/v1` traffic without
 *    exposing the backend origin in the browser bundle.
 *
 * SECURITY NOTES
 *  - Only an allow-list of request headers is forwarded (never cookies).
 *  - Nothing is logged here: no tokens, no bodies.
 *  - Upstream failures return the backend's JSON error envelope shape with
 *    code `BACKEND_UNAVAILABLE` so clients keep a single error contract.
 */
import type { NextRequest } from 'next/server'

export const dynamic = 'force-dynamic'

/** Request headers the backend expects/accepts. Cookies are intentionally NOT forwarded. */
const FORWARD_REQUEST_HEADERS = ['accept', 'authorization', 'content-type', 'x-request-id']
/** Response headers worth relaying back to the browser. */
const FORWARD_RESPONSE_HEADERS = ['content-type', 'x-request-id']

const UPSTREAM_TIMEOUT_MS = 15_000

/** Build the upstream URL: strip the Next route prefix, join the server-only base. */
function upstreamUrl(request: NextRequest): string | null {
  const base = process.env.API_BASE_URL?.trim().replace(/\/+$/, '')
  if (!base) return null
  const suffix = request.nextUrl.pathname.replace(/^\/api\/v1/, '') || '/'
  return `${base}${suffix}${request.nextUrl.search}`
}

function unavailable(): Response {
  return Response.json(
    {
      error: {
        code: 'BACKEND_UNAVAILABLE',
        message: 'The Mawarid API is unreachable. Please try again shortly.',
        message_ar: 'تعذر الوصول إلى خادم موارد. يرجى المحاولة بعد قليل.',
        request_id: '',
      },
    },
    { status: 502 },
  )
}

async function handle(request: NextRequest): Promise<Response> {
  const target = upstreamUrl(request)
  if (!target) {
    return Response.json(
      {
        error: {
          code: 'CONFIG_ERROR',
          message: 'API_BASE_URL is not configured on the server.',
          message_ar: 'لم يتم إعداد رابط واجهة الخادم في الخادم الأم.',
          request_id: '',
        },
      },
      { status: 500 },
    )
  }

  const headers = new Headers()
  for (const name of FORWARD_REQUEST_HEADERS) {
    const value = request.headers.get(name)
    if (value) headers.set(name, value)
  }

  // Read the body as bytes (not a stream) so upstream fetch needs no duplex
  // option and binary payloads stay intact. GET requests have no body.
  let body: ArrayBuffer | undefined
  if (request.method !== 'GET' && request.body !== null) {
    body = await request.arrayBuffer()
  }

  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), UPSTREAM_TIMEOUT_MS)
  try {
    const upstream = await fetch(target, {
      method: request.method,
      headers,
      body,
      cache: 'no-store',
      signal: controller.signal,
      redirect: 'manual',
    })

    const out = new Headers()
    for (const name of FORWARD_RESPONSE_HEADERS) {
      const value = upstream.headers.get(name)
      if (value) out.set(name, value)
    }

    if (upstream.status === 204 || upstream.status === 205 || upstream.status === 304) {
      return new Response(null, { status: upstream.status, headers: out })
    }
    const buffer = await upstream.arrayBuffer()
    return new Response(buffer, { status: upstream.status, headers: out })
  } catch {
    // Transport-level failure (backend down / timeout). No details are logged.
    return unavailable()
  } finally {
    clearTimeout(timer)
  }
}

export async function GET(request: NextRequest): Promise<Response> {
  return handle(request)
}

export async function POST(request: NextRequest): Promise<Response> {
  return handle(request)
}

export async function PUT(request: NextRequest): Promise<Response> {
  return handle(request)
}

export async function PATCH(request: NextRequest): Promise<Response> {
  return handle(request)
}

export async function DELETE(request: NextRequest): Promise<Response> {
  return handle(request)
}
