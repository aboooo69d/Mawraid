/**
 * Mawarid API client — typed `fetch` wrapper for the Go (GORM/PostgreSQL/Redis) backend.
 *
 * STATUS: integration layer ONLY. No UI component imports this module yet, so the
 * existing demo flow is completely unaffected. The backend contract is provisional
 * until the Go team publishes the OpenAPI/Swagger specification.
 *
 * Server-only configuration (read from `process.env`, never bundled for the browser
 * because these variables intentionally have NO `NEXT_PUBLIC_` prefix):
 *
 *   API_BASE_URL    Base URL of the Go API, e.g. http://localhost:8080/api/v1
 *   API_TIMEOUT_MS  Optional per-request timeout in milliseconds (default 10000)
 *
 * SECURITY: secrets/tokens must never be added with a `NEXT_PUBLIC_` prefix — Next.js
 * inlines any `NEXT_PUBLIC_*` value into the client bundle. Attach credentials
 * server-side only (see TODO(BACKEND-CONFIRM) below).
 *
 * @see lib/api/types.ts     provisional response/request interfaces
 * @see lib/api/orders.ts    provisional order endpoints
 */

export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

/** Query string values; `null` / `undefined` / `''` entries are dropped. */
export type QueryParams = Record<string, string | number | boolean | null | undefined>

export interface ApiRequestOptions {
  /** Defaults to `GET`. */
  method?: HttpMethod
  /** Plain object => serialized as JSON. `string` is sent raw. */
  body?: unknown
  query?: QueryParams
  /** Extra headers merged over the defaults (e.g. server-side auth headers). */
  headers?: Record<string, string>
  /** Caller-controlled cancellation; merged with the internal timeout signal. */
  signal?: AbortSignal
  /** Overrides `API_TIMEOUT_MS` for this request only. */
  timeoutMs?: number
}

export const DEFAULT_TIMEOUT_MS = 10_000

/** Normalized error shape thrown by every failure path of this client. */
export class ApiError extends Error {
  /** HTTP status, or `0` for transport-level failures (timeout/network/config/abort). */
  readonly status: number
  /** Machine-readable code: backend `error.code`, otherwise `HTTP_<status>` / transport code. */
  readonly code: string
  /** Request URL (base + path + query) that produced the error. */
  readonly url: string
  /** Parsed response body, when the server returned one. */
  readonly details?: unknown
  /** Correlation id from `x-request-id` / `error.request_id`, when present. */
  readonly requestId?: string

  constructor(
    message: string,
    init: { status: number; code: string; url?: string; details?: unknown; requestId?: string; cause?: unknown },
  ) {
    super(message, init.cause !== undefined ? { cause: init.cause } : undefined)
    this.name = 'ApiError'
    this.status = init.status
    this.code = init.code
    this.url = init.url ?? ''
    this.details = init.details
    this.requestId = init.requestId
  }
}

export function isApiError(value: unknown): value is ApiError {
  return value instanceof ApiError
}

/** Resolved, validated base URL. Throws `ApiError` instead of returning a half-configured URL. */
export function getApiBaseUrl(): string {
  // Guard: this module must only be used from server code. Without the optional
  // `server-only` package (not installed — no new packages allowed), this runtime
  // check keeps the server-only base URL out of browser requests.
  if (typeof window !== 'undefined') {
    throw new ApiError(
      'lib/api may only be called from server-side code (the API base URL is a server-only secret-free config).',
      { status: 0, code: 'SERVER_ONLY' },
    )
  }
  const raw = process.env.API_BASE_URL?.trim()
  if (!raw) {
    throw new ApiError(
      'Missing server-only environment variable API_BASE_URL. Copy .env.example to .env.local and set it.',
      { status: 0, code: 'CONFIG_ERROR' },
    )
  }
  return raw.replace(/\/+$/, '')
}

function resolveTimeoutMs(override?: number): number {
  if (typeof override === 'number' && Number.isFinite(override) && override > 0) return override
  const fromEnv = Number(process.env.API_TIMEOUT_MS)
  if (Number.isFinite(fromEnv) && fromEnv > 0) return fromEnv
  return DEFAULT_TIMEOUT_MS
}

function normalizePath(path: string): string {
  if (!path) return ''
  return path.startsWith('/') ? path : `/${path}`
}

function withQuery(target: string, query?: QueryParams): string {
  if (!query) return target
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null || value === '') continue
    params.append(key, String(value))
  }
  const serialized = params.toString()
  if (!serialized) return target
  return `${target}${target.includes('?') ? '&' : '?'}${serialized}`
}

/**
 * TODO(BACKEND-CONFIRM): auth scheme is undefined until the OpenAPI spec lands.
 * When confirmed, derive headers here from a NON-public environment variable
 * (e.g. process.env.API_AUTH_TOKEN) — never NEXT_PUBLIC_*.
 */
function buildAuthHeaders(): Record<string, string> {
  return {}
}

async function parseBody(response: Response): Promise<unknown> {
  if (response.status === 204 || response.status === 205) return undefined
  let text: string
  try {
    text = await response.text()
  } catch {
    return undefined
  }
  if (!text) return undefined
  const contentType = response.headers.get('content-type') ?? ''
  if (!contentType.includes('json')) return text
  try {
    return JSON.parse(text) as unknown
  } catch (cause) {
    if (response.ok) {
      throw new ApiError('Response declared JSON but could not be parsed.', {
        status: response.status,
        code: 'INVALID_JSON',
        cause,
      })
    }
    return text // non-JSON error payload; handled by the error branch below
  }
}

function readString(source: Record<string, unknown>, ...keys: string[]): string | undefined {
  for (const key of keys) {
    const value = source[key]
    if (typeof value === 'string' && value) return value
  }
  return undefined
}

/**
 * Perform a typed JSON request against the Go API.
 *
 * Resolves with the parsed body cast to `T`, or rejects with an {@link ApiError}.
 *
 * @throws {ApiError} `CONFIG_ERROR` (no base URL), `SERVER_ONLY` (called from the
 * browser), `TIMEOUT`, `ABORTED`, `NETWORK_ERROR`, `INVALID_JSON`, or an HTTP error
 * (`status` >= 400) carrying the backend's `code` / `message` when available.
 */
export async function apiRequest<T>(path: string, options: ApiRequestOptions = {}): Promise<T> {
  const { method = 'GET', body, query, headers = {}, signal, timeoutMs } = options
  const url = withQuery(`${getApiBaseUrl()}${normalizePath(path)}`, query)
  const timeout = resolveTimeoutMs(timeoutMs)

  const controller = new AbortController()
  let timedOut = false
  const timer = setTimeout(() => {
    timedOut = true
    controller.abort()
  }, timeout)
  const forwardAbort = () => controller.abort()
  if (signal) {
    if (signal.aborted) controller.abort()
    else signal.addEventListener('abort', forwardAbort, { once: true })
  }

  const requestHeaders: Record<string, string> = {
    Accept: 'application/json',
    ...buildAuthHeaders(),
    ...headers,
  }
  let payload: string | undefined
  if (body !== undefined && body !== null) {
    if (typeof body === 'string') {
      payload = body
      if (!requestHeaders['Content-Type']) requestHeaders['Content-Type'] = 'text/plain; charset=utf-8'
    } else {
      payload = JSON.stringify(body)
      requestHeaders['Content-Type'] = 'application/json'
    }
  }

  let response: Response
  try {
    response = await fetch(url, {
      method,
      headers: requestHeaders,
      body: payload,
      signal: controller.signal,
      cache: 'no-store',
    })
  } catch (cause) {
    if (timedOut) {
      throw new ApiError(`Request timed out after ${timeout}ms.`, { status: 0, code: 'TIMEOUT', url, cause })
    }
    if (signal?.aborted) {
      throw new ApiError('Request was aborted by the caller.', { status: 0, code: 'ABORTED', url, cause })
    }
    throw new ApiError('Network request failed before reaching the API.', {
      status: 0,
      code: 'NETWORK_ERROR',
      url,
      cause,
    })
  } finally {
    clearTimeout(timer)
    signal?.removeEventListener('abort', forwardAbort)
  }

  const data = await parseBody(response)

  if (!response.ok) {
    const envelope: Record<string, unknown> =
      data !== null && typeof data === 'object' ? (data as Record<string, unknown>) : {}
    const node: Record<string, unknown> =
      envelope.error !== null && typeof envelope.error === 'object'
        ? (envelope.error as Record<string, unknown>)
        : envelope
    const message =
      readString(node, 'message', 'detail', 'title') ??
      `Request failed with status ${response.status} ${response.statusText}`.trim()
    const code = readString(node, 'code', 'error_code') ?? `HTTP_${response.status}`
    const requestId =
      response.headers.get('x-request-id') ?? readString(node, 'request_id', 'requestId') ?? undefined
    throw new ApiError(message, { status: response.status, code, url, details: data, requestId })
  }

  return data as T
}
