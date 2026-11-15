/**
 * ⚠️ PROVISIONAL ORDER ENDPOINTS — NOT CONFIRMED BY THE BACKEND.
 * -----------------------------------------------------------------------------
 * TASK 02: these paths, methods, and payloads are educated guesses made before the
 * Go backend team published the OpenAPI/Swagger specification. NOTHING in the UI
 * calls them yet (the demo flow stays fully local and functional).
 *
 * When the spec arrives, update `ORDER_ENDPOINTS` and the function signatures in
 * this file to match it exactly.
 *
 * ENDPOINT INVENTORY (provisional):
 *   POST   /orders                create an order
 *   GET    /orders/{orderId}      get order details
 *   GET    /orders                list the current customer's orders (paginated)
 *   POST   /orders/{orderId}/cancel  cancel an order
 *
 * Relative to `API_BASE_URL` (server-only, e.g. http://localhost:8080/api/v1).
 *
 * OPEN QUESTIONS FOR THE BACKEND DEVELOPER:
 *  A. Path prefix: `/orders` relative to an `/api/v1` base, or `/api/v1/orders`
 *     relative to the host root?
 *  B. Cancellation: `POST .../cancel` vs `DELETE /orders/{id}` — and is a
 *     `{ reason }` body accepted/required? Response: updated `Order`, `204`, or
 *     an ack object?
 *  C. Customer scoping on `GET /orders`: from the authenticated session
 *     server-side, or a `customerId` query parameter? Is auth by cookie, bearer
 *     token, or API key (and from which server-only env var)?
 *  D. Success shape: bare object or `{ data: ... }` envelope (see lib/api/types.ts)?
 *  E. Error codes for the standardized `ApiError.code` (e.g. `ORDER_NOT_FOUND`,
 *     `ORDER_NOT_CANCELLABLE`, `VALIDATION_ERROR`) and the status mapping.
 *  F. Idempotency: should `POST /orders` accept an idempotency key?
 */

import { apiRequest, type ApiRequestOptions } from '@/lib/api/client'
import type {
  CancelOrderRequest,
  CreateOrderRequest,
  Order,
  OrderListQuery,
  OrderListResponse,
} from '@/lib/api/types'

/** ⚠️ Provisional paths — replace with the confirmed OpenAPI paths. */
export const ORDER_ENDPOINTS = {
  create: '/orders',
  list: '/orders',
  byId: (orderId: string) => `/orders/${encodeURIComponent(orderId)}`,
  cancel: (orderId: string) => `/orders/${encodeURIComponent(orderId)}/cancel`,
} as const

/** Per-call overrides (timeout, headers, abort signal, extra query) minus method/body. */
export type CallOptions = Omit<ApiRequestOptions, 'method' | 'body' | 'query'>

/**
 * Create an order.
 * ⚠️ Provisional: `POST {API_BASE_URL}/orders` → `201 Created` with the created `Order`.
 * TODO(BACKEND-CONFIRM): status code, response envelope, idempotency key, validation rules.
 */
export async function createOrder(input: CreateOrderRequest, options: CallOptions = {}): Promise<Order> {
  return apiRequest<Order>(ORDER_ENDPOINTS.create, {
    ...options,
    method: 'POST',
    body: input,
  })
}

/**
 * Get a single order by id.
 * ⚠️ Provisional: `GET {API_BASE_URL}/orders/{orderId}` → `200 OK` with an `Order`.
 * TODO(BACKEND-CONFIRM): error code/status when the order does not exist or is
 * not owned by the caller (404 vs 403).
 */
export async function getOrder(orderId: string, options: CallOptions = {}): Promise<Order> {
  return apiRequest<Order>(ORDER_ENDPOINTS.byId(orderId), {
    ...options,
    method: 'GET',
  })
}

/**
 * List the current customer's orders (newest first assumed).
 * ⚠️ Provisional: `GET {API_BASE_URL}/orders?page=&pageSize=&status=&service=` → `200 OK`.
 * TODO(BACKEND-CONFIRM): customer scoping (session vs `customerId`), pagination
 * style, sort order, and whether filters are supported at all.
 */
export async function listOrders(
  query: OrderListQuery = {},
  options: CallOptions = {},
): Promise<OrderListResponse> {
  return apiRequest<OrderListResponse>(ORDER_ENDPOINTS.list, {
    ...options,
    method: 'GET',
    query: { ...query },
  })
}

/**
 * Cancel an order.
 * ⚠️ Provisional: `POST {API_BASE_URL}/orders/{orderId}/cancel` with an optional
 * `{ reason }` body → `200 OK` with the updated `Order`.
 * TODO(BACKEND-CONFIRM): HTTP verb, response shape (Order vs `204`), which
 * statuses are cancellable, and the error code when they are not.
 */
export async function cancelOrder(
  orderId: string,
  request: CancelOrderRequest = {},
  options: CallOptions = {},
): Promise<Order> {
  return apiRequest<Order>(ORDER_ENDPOINTS.cancel(orderId), {
    ...options,
    method: 'POST',
    body: request,
  })
}
