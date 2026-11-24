# Unit Price – Frontend API Changes

A `price` field has been added to stock units. Every unit now carries its own price (stored as `numeric(10,2)`), which is required on creation, optionally updatable, and returned in all unit, stock, and transaction responses.

> **Breaking Change:** `price` is now **required** and must be `> 0` when creating a stock (`units[].price`) or adding a new unit. Requests without `price` or with `price <= 0` will return `400 Bad Request`.

---

## Affected Endpoints

Base prefix: `/api/v1`

| Method & Endpoint | Role | What Changed |
|---|---|---|
| `POST /stocks/me` | `provider` | **Request:** `units[].price` is now **required** (`> 0`).<br>**Response:** `data.units[].price` added. |
| `POST /stocks/me/:stock_id/units` | `provider` | **Request:** `price` is now **required** (`> 0`).<br>**Response:** `data.price` added. |
| `PATCH /stocks/me/:stock_id/units/:unit_id` | `provider` | **Request:** optional `price` (`> 0`) accepted.<br>**Response:** `data.price` added. |
| `GET /stocks` | `consumer`, `admin` | **Response:** `data[].units[].price` added. |
| `GET /stocks/me` | `provider` | **Response:** `data[].units[].price` added. |
| `GET /stocks/me/:stock_id` | `provider` | **Response:** `data.units[].price` added. |
| `POST /stocks/me/:stock_id/transactions` | `provider` | **Response:** `data.unit.price` added. |
| `GET /stocks/me/:stock_id/transactions` | `provider` | **Response:** `data[].unit.price` added. |

---

## Updated TypeScript Interfaces

```ts
export interface CreateUnitRequest {
  name: string;     // required, max 50 chars
  rate: number;     // required, integer >= 1 (must be 1 when is_main is true)
  price: number;    // NEW — required, > 0 (up to 2 decimal places)
  is_main?: boolean;
}

export interface UpdateUnitRequest {
  name?: string;    // optional, 1..50 chars
  rate?: number;    // optional, integer >= 1 (cannot change if unit has transactions or is main)
  price?: number;   // NEW — optional, > 0 (up to 2 decimal places)
}

export interface UnitResponse {
  id: number;
  name: string;
  rate: number;
  price: number;    // NEW
  is_main: boolean;
}
```

---

## Request & Response Examples

### 1. Create Stock (`POST /api/v1/stocks/me`)

```json
{
  "stock_type_id": 1,
  "stock_type": "gaz",
  "address": "Al-Zubairi Street, Sana'a",
  "location": {
    "lat": 15.3694,
    "lng": 44.191
  },
  "units": [
    {
      "name": "Cylinder",
      "rate": 1,
      "price": 7500,
      "is_main": true
    },
    {
      "name": "Pack of 5",
      "rate": 5,
      "price": 36000,
      "is_main": false
    }
  ],
  "stock_details": {
    "gaz_stock_type": "exchange"
  }
}
```

### 2. Add Unit (`POST /api/v1/stocks/me/:stock_id/units`)

```json
{
  "name": "Pack of 10",
  "rate": 10,
  "price": 70000
}
```

**Response (`200 OK`):**

```json
{
  "success": true,
  "data": {
    "id": 12,
    "name": "Pack of 10",
    "rate": 10,
    "price": 70000,
    "is_main": false
  }
}
```

### 3. Update Unit (`PATCH /api/v1/stocks/me/:stock_id/units/:unit_id`)

Only send the fields you want to change. You can update `price` by itself:

```json
{
  "price": 8000.5
}
```

> **Note:** Unlike `rate`, updating `price` **is allowed** on the main unit and on units that already have past transactions.

---

## Validation Rules & Errors

| Case | HTTP Status |
|---|---|
| `price` omitted in `POST /stocks/me` (`units[]`) or `POST /stocks/me/:stock_id/units` | `400 Bad Request` |
| `price <= 0` (zero or negative) in create or update payload | `400 Bad Request` |
| `price` omitted in `PATCH /stocks/me/:stock_id/units/:unit_id` | Allowed (existing price is kept) |

- Precision: stored as `numeric(10,2)` (up to 8 digits before the decimal point and 2 decimal places).
- Existing units created before this change default to `price: 0` in the database until updated via `PATCH`.

---

## Frontend Checklist

1. **Create Stock form:** Add a required positive numeric `price` input for each unit row.
2. **Add Unit modal/form:** Add a required positive numeric `price` input.
3. **Edit Unit modal/form:** Add an optional positive numeric `price` input (enabled even when `rate` is locked due to transactions or `is_main === true`).
4. **Stock & Consumer views:** Display `unit.price` alongside each unit in provider stock details and consumer stock listings (`GET /api/v1/stocks`).
5. **TypeScript models / API clients:** Add `price: number` to `UnitResponse` and `CreateUnitRequest`, and `price?: number` to `UpdateUnitRequest`.
