/**
 * ⚠️ PROVISIONAL TYPES — PLACEHOLDERS, NOT AN AUTHORITATIVE CONTRACT.
 * -----------------------------------------------------------------------------
 * TASK 02: these interfaces are best-effort guesses made before the Go backend
 * team shared their OpenAPI/Swagger specification.
 *
 * DO NOT treat them as final. Once the spec arrives, reconcile or regenerate this
 * file (e.g. with openapi-typescript) and update lib/api/orders.ts accordingly.
 *
 * OPEN QUESTIONS FOR THE BACKEND DEVELOPER (see also lib/api/orders.ts):
 *  1. Response envelope: bare JSON object, or wrapped as `{ data: ... }` /
 *     `{ error: ... }`? The client currently assumes a BARE payload for success
 *     responses and accepts both bare and `{ error: ... }` for failures.
 *  2. Identifiers: string UUIDs, numeric ids, or prefixed codes (demo uses `MR-...`)?
 *  3. Money: number (riyal units) or decimal string? Integer minor units?
 *  4. Timestamps: RFC3339/ISO-8601 strings with timezone?
 *  5. Enums: exact `status` / `paymentMethod` values and casing (snake_case vs camelCase).
 *  6. Pagination: `page`/`pageSize`+`total`, or cursor-based?
 *  7. Service catalogue: demo ships `gas` + `water` (electricity "قريباً") —
 *     confirm the canonical service identifiers.
 */

/** Placeholder service identifiers. Mirrors the demo's `Service` type in components/mawared-types.ts. */
export type OrderService = 'gas' | 'water'

/**
 * Placeholder order lifecycle.
 * TODO(BACKEND-CONFIRM): exact values come from the OpenAPI spec; the Go side may
 * use snake_case (`in_review`) or an integer enum instead.
 */
export type OrderStatus = 'pending' | 'in_review' | 'confirmed' | 'preparing' | 'out_for_delivery' | 'delivered' | 'cancelled'

/**
 * Placeholder payment methods.
 * TODO(BACKEND-CONFIRM): the demo only offers "الدفع النقدي عند الاستلام" (cash on delivery).
 */
export type PaymentMethod = 'cash_on_delivery' | 'wallet'

/** Placeholder money/quantity note: unit price in riyal (YER), demo values are integers. */
export type Currency = 'YER'

/** Customer/order payload as stored by the backend. */
export interface Order {
  /** Placeholder: confirm id format (`MR-XXXXXXX` vs UUID). */
  id: string
  service: OrderService
  /** Quantity in service units (أسطوانة / صهريج); demo caps at 3. */
  quantity: number
  /** Price of a single unit at the time of ordering. */
  unitPrice: number
  /** `unitPrice * quantity` — confirm whether the server computes this. */
  totalAmount: number
  currency: Currency
  customerName: string
  district: string
  address: string
  paymentMethod: PaymentMethod
  status: OrderStatus
  /** ISO-8601 timestamp, e.g. `2026-10-21T18:30:00Z`. */
  createdAt: string
  /** ISO-8601 timestamp. */
  updatedAt?: string
  /** ISO-8601 timestamp for cancelled/delivered orders. */
  completedAt?: string | null
  /** Human-readable cancellation reason, if any. */
  cancellationReason?: string | null
}

/** Body for `POST /orders` — matches the fields the demo form already collects. */
export interface CreateOrderRequest {
  service: OrderService
  quantity: number
  customerName: string
  district: string
  address: string
  paymentMethod: PaymentMethod
}

/** Body for `PATCH /orders/{id}`-style cancellation (see lib/api/orders.ts). */
export interface CancelOrderRequest {
  reason?: string
}

/** Placeholder filters for listing a customer's orders. */
export interface OrderListQuery {
  page?: number
  pageSize?: number
  status?: OrderStatus
  service?: OrderService
  /**
   * TODO(BACKEND-CONFIRM): is the customer derived from the authenticated session
   * server-side, or passed explicitly as a query/body parameter?
   */
  customerId?: string
}

/** Placeholder paginated list response. */
export interface OrderListResponse {
  items: Order[]
  page: number
  pageSize: number
  total: number
}

/**
 * Placeholder inventory snapshot for a service.
 * TODO(BACKEND-CONFIRM): does availability get exposed publicly, and is it cached
 * in Redis with a TTL? The frontend must not assume real-time stock.
 */
export interface InventoryAvailability {
  service: OrderService
  /** Units currently orderable (أسطوانة / صهريج). */
  availableQuantity: number
  /** Units already reserved by open orders. */
  reservedQuantity?: number
  unit: string
  unitPrice: number
  currency: Currency
  /** ISO-8601 timestamp of the last stock update. */
  updatedAt?: string
  /**
   * Placeholder for backend-side caching hints (e.g. Redis TTL in seconds).
   * Confirm whether this is exposed at all.
   */
  expiresInSeconds?: number
}

/** Standardized error body the client tries to read (bare or nested under `error`). */
export interface ApiErrorBody {
  /** Machine-readable code, e.g. `ORDER_NOT_FOUND`, `VALIDATION_ERROR`. */
  code?: string
  message?: string
  details?: unknown
  requestId?: string
}

/**
 * Envelope helpers — usage depends on OPEN QUESTION #1 above.
 * The client currently returns the parsed body as-is; if the backend wraps
 * responses, these become the `T` passed to `apiRequest<T>`.
 */
export interface ApiEnvelope<T> {
  data: T
}

export interface ApiErrorEnvelope {
  error: ApiErrorBody
}

/** Placeholder for field-level validation errors (GORM/Go validator style). */
export interface FieldViolation {
  field: string
  message: string
}
