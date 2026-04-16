# broccoli

OpenAPI generators are great until you actually have to set one up. Suddenly you're writing YAML specs, wiring up plugins, configuring codegen, and before you know it you've spent a day on tooling instead of shipping features.

Broccoli takes a different but simpler approach. If you're a backend engineer writing Go, you already have the source of truth: your structs. Just point broccoli at them and it'll hand your frontend teammates a ready-to-use TypeScript types file.

## Installation

```bash
go install github.com/faizalv/broccoli@latest
```

## Usage

```bash
broccoli --modular --moddir ./your-modules-dir
```

- `--modular`  enables modular mode (required)
- `--moddir`  path to the directory where your Go modules/packages live
- `--with-sdk`  also generate a typed fetch SDK from your handler annotations (see below)

Generated TypeScript files land in `contract/typescript/`, one file per module. Commit that folder and your frontend team always has up-to-date types.

## Example

You write this on the backend:

```go
// modules/library/response/response.go
type ErrorInfo struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}

// modules/order/entity/order.go
type OrderDetailResponse struct {
    ID          int    `json:"id"`
    ProductName string `json:"product_name"`
    Quantity    int    `json:"quantity"`
    Price       int    `json:"price"`
}

type OrderResponse struct {
    ID         int                   `json:"id"`
    TotalPrice int                   `json:"total_price"`
    CreatedAt  string                `json:"created_at"`
    Details    []OrderDetailResponse `json:"details"`
    Error      ErrorInfo             `json:"error"`
}
```

Run broccoli, and your frontend team gets this:

```typescript
// contract/typescript/general.ts

export interface ErrorInfo {
  code: number
  message: string
}
```

```typescript
// contract/typescript/order.ts

import {ErrorInfo} from './general'

export interface OrderDetailResponse {
  id: number
  product_name: string
  quantity: number
  price: number
}

export interface OrderResponse {
  id: number
  total_price: number
  created_at: string
  details: OrderDetailResponse[]
  error: ErrorInfo
}
```

When a type references something from another module, broccoli figures out the import and writes it for you.

### SDK generation

Pass `--with-sdk` to also emit `contract/typescript/sdk.ts` — a file of typed `fetch` wrappers, one function per annotated handler.

Broccoli reads [swaggo](https://github.com/swaggo/swag) comments. The `@Summary` tag is used as the generated function name, so name it like you'd name the function:

```go
// @Summary Create Order
// @Router /orders [post]
// @Param body body CreateOrderRequest true "request body"
// @Success 200 {object} OrderResponse
func CreateOrderHandler(c *gin.Context) { ... }

// @Summary Get Order
// @Router /orders/{id} [get]
// @Param id path int true "order ID"
// @Success 200 {object} OrderResponse
func GetOrderHandler(c *gin.Context) { ... }

// @Summary List Orders
// @Router /orders [get]
// @Param page query int false "page number"
// @Success 200 {array} OrderResponse
func ListOrdersHandler(c *gin.Context) { ... }
```

Run broccoli with `--with-sdk` and your frontend team gets:

```typescript
// contract/typescript/sdk.ts

export type {CreateOrderRequest,OrderDetailResponse,OrderResponse} from './order'

import type {CreateOrderRequest,OrderResponse} from './order'

export async function createOrder(body: CreateOrderRequest, options?: RequestInit): Promise<OrderResponse> {
  const res = await fetch('/orders', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(body),
    ...options,
  })
  return res.json()
}

export async function getOrder(id: number, options?: RequestInit): Promise<OrderResponse> {
  const res = await fetch(`/orders/${id}`, {
    method: 'GET',
    ...options,
  })
  return res.json()
}

export async function listOrders(page?: number, options?: RequestInit): Promise<OrderResponse[]> {
  const _q = new URLSearchParams()
  if (page !== undefined) _q.append('page', String(page))
  const _qs = _q.toString()
  const res = await fetch(_qs ? '/orders' + '?' + _qs : '/orders', {
    method: 'GET',
    ...options,
  })
  return res.json()
}
```

Both `{id}` (chi/swaggo) and `:id` (gin) path param styles are supported. The generated SDK uses the native Fetch API — no extra dependencies.

`sdk.ts` re-exports every type from every module, so the frontend only ever needs one import path — functions, response shapes, header structs, all of it:

```typescript
import { createOrder, RequestHeaders } from './contract/typescript/sdk'

const order = await createOrder(body, { headers: { Authorization: `Bearer ${token}` } satisfies RequestHeaders })
```

## What gets converted

| Go | TypeScript |
|---|---|
| `string` | `string` |
| `bool` | `boolean` |
| `int`, `float64`, etc. | `number` |
| `time.Time` | `string` |
| `interface{}` | `any` |
| `[]T` | `T[]` |
| `map[K]V` | `Record<K, V>` |
| `*T` | same as `T` |
| embedded struct | fields inlined |

Only structs with `json` tags are emitted — if it's not going over the wire, broccoli ignores it. Fields with `omitempty` become optional (`?`) on the TypeScript side.

## License

MIT
