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

- `--modular` — enables modular mode (required)
- `--moddir` — path to the directory where your Go modules/packages live

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
