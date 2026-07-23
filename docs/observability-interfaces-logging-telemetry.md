# Interfaces, Logging & Telemetry — analysis of `risk_audit` and an adoption plan for Lodge Management System

This document analyses how the **`risk_audit`** project (`C:\development\infratel\risk_audit`) structures its
service interfaces, structured logging, and telemetry, then lays out how we can adopt the same pattern in the
**Lodge Management System** backend (`lodge-system`).

---

## 1. What `risk_audit` does

### 1.1 The big idea — interfaces as a decoration seam

Every domain capability is expressed as a **Go interface** in a dedicated `internal/interfaces` package. The
concrete service implements it, and cross-cutting concerns (logging, telemetry) are layered on as
**decorators** that wrap the interface — never by editing the service itself.

```
raw service  ──►  LoggerMiddleware  ──►  TelemetryMiddleware  ──►  handler
(business logic)   (times each call)     (adds a trace event)      (HTTP)
```

Because each decorator implements the *same* interface it wraps, they compose transparently. The handler only
ever sees `interfaces.CountryInterface` — it can't tell whether it's talking to the raw service or a stack of
decorators. This is the classic **middleware/decorator pattern** applied at the service boundary (as opposed to
the HTTP boundary).

### 1.2 The interface (the contract)

`internal/interfaces/country_interface.go`:

```go
package interfaces

type CountryInterface interface {
	CreateCountry(context.Context, string, string, string, string) error
	UpdateCountry(context.Context, uuid.UUID, string, string, string, bool) error
	ListCountries(context.Context) ([]*models.Country, error)
	GetCountryByID(context.Context, uuid.UUID) (*models.Country, error)
	DeleteCountry(context.Context, uuid.UUID) error
}
```

Key conventions:
- One interface file per domain entity (`country_interface.go`, `risk_interface.go`, `auth_interface.go`, …).
- **`context.Context` is the first argument of every method** — this is what makes telemetry possible, because
  the trace span travels in the context.
- Methods return domain models + `error`.

### 1.3 The logger decorator

`internal/middleware/logger/country_logger.go`:

```go
type CountryLoggerMiddleware struct {
	next interfaces.CountryInterface
}

func (m *CountryLoggerMiddleware) CreateCountry(ctx context.Context, name, code, iso2Code, region string) error {
	start := time.Now()
	defer func() {
		zap.L().Info("CreateCountry", zap.Duration("took", time.Since(start)))
	}()
	return m.next.CreateCountry(ctx, name, code, iso2Code, region)
}

// … one wrapper per interface method …

func NewCountryLoggerMiddleware(next interfaces.CountryInterface) interfaces.CountryInterface {
	return &CountryLoggerMiddleware{next}
}
```

Every method:
1. Records `start := time.Now()`.
2. `defer`s a `zap.L().Info(<method name>, zap.Duration("took", …))`.
3. Delegates to `m.next`.

Uses **`go.uber.org/zap`** (`zap.L()` — the global logger). Structured, leveled, JSON-capable.

### 1.4 The telemetry decorator

`internal/middleware/telemetry/country_telemetry.go`:

```go
type CountryTelemetryMiddleware struct {
	next interfaces.CountryInterface
}

func (c *CountryTelemetryMiddleware) CreateCountry(ctx context.Context, name, code, iso2Code, region string) error {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(fmt.Sprintf("CreateCountry: %v", name))
	return c.next.CreateCountry(ctx, name, code, iso2Code, region)
}

// … one wrapper per interface method …

func NewCountryTelemetryMiddleware(next interfaces.CountryInterface) interfaces.CountryInterface {
	return &CountryTelemetryMiddleware{next}
}
```

Every method:
1. Pulls the active span out of the context: `span := trace.SpanFromContext(ctx)`.
2. Records a span event describing the operation: `span.AddEvent(...)`.
3. Delegates to `c.next`.

Uses **`go.opentelemetry.io/otel/trace`** (the OTel tracing API).

### 1.5 The handler

`internal/handlers/country_handler.go`:

```go
type CountryHandler struct {
	country interfaces.CountryInterface   // depends on the INTERFACE, not the concrete service
	logger  *zap.Logger                   // handler gets its own injected logger too
}

func (h *CountryHandler) ListCountries(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("listcountries called",
		zap.String("method", "ListCountries"),
		zap.String("request_path", r.URL.Path))

	countries, err := h.country.ListCountries(r.Context())   // passes request context through
	if err != nil {
		h.logger.Error("operation failed", zap.Error(err))
		httputil.SendError(w, http.StatusInternalServerError, "Failed to list countries")
		return
	}
	httputil.SendSuccess(w, "Success", countries)
}
```

The handler:
- Depends on the **interface**, so the decorator stack is injectable.
- Also carries its own `*zap.Logger` and logs at entry and on error, with structured fields.
- Passes `r.Context()` down — carrying the span so the telemetry decorator has something to attach to.

### 1.6 Composition / wiring (the assembly point)

`internal/server/server.go` (~line 377) — the raw service is wrapped, then handed to the handler:

```go
countryServiceWithLogging   := logger.NewCountryLoggerMiddleware(countryService)
countryServiceWithTelemetry := telemetry.NewCountryTelemetryMiddleware(countryServiceWithLogging)
// …
countryHandler := handlers.NewCountryHandler(countryServiceWithTelemetry, s.config.App.Logger)
```

The nesting order (`telemetry(logger(service))`) means, at call time, control flows
**telemetry → logger → service**, and the deferred log fires as the stack unwinds. This wiring is repeated
verbatim for every entity, giving a highly uniform (if verbose) `server.go`.

### 1.7 Initialisation

`cmd/app/main.go` builds a production zap logger and stores it on the config:

```go
logConfig := zap.NewProductionConfig()
logConfig.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
logConfig.EncoderConfig.TimeKey   = "timestamp"
logConfig.EncoderConfig.LevelKey  = "level"
logConfig.EncoderConfig.MessageKey = "message"
logger, _ := logConfig.Build()
cfg.App.Logger = logger
```

There's also a reusable `internal/utils/logger/logger.go` (`NewLogger(cfg *config.LogConfig)`) that maps a
`LogConfig{Level, Format}` to a zap logger (json→production, text→development).

### 1.8 Honest gaps in `risk_audit`'s implementation

Two things are worth calling out so we adopt the *good* parts and fix the weak ones:

1. **`zap.ReplaceGlobals` is never called.** The logger decorators use `zap.L()` (the global logger), but the
   configured logger is only stored on `cfg.App.Logger` and injected into *handlers*. The global zap logger is
   therefore the library default (a no-op-ish production logger), so the **decorator log lines may not go where
   the configured logger points**. Fix: call `zap.ReplaceGlobals(logger)` at startup, or inject the logger into
   the decorators instead of using `zap.L()`.

2. **The OpenTelemetry SDK is never initialised.** There is no `otel.SetTracerProvider(...)`, no exporter, and
   no HTTP middleware that calls `tracer.Start(...)` to open a span per request. So `trace.SpanFromContext(ctx)`
   returns a **non-recording (no-op) span**, and every `span.AddEvent(...)` in the telemetry decorators is
   effectively discarded at runtime. The telemetry layer is **scaffolding that compiles and is ready**, but not
   yet "lit up." Fix: initialise an OTel TracerProvider + exporter at startup and add a tracing HTTP middleware
   that starts a span per request (see §3.3).

The pattern is sound; these two omissions just mean it isn't fully wired end-to-end yet in `risk_audit`.

---

## 2. Where Lodge Management System stands today

| Concern | `risk_audit` | `lodge-system` (current) |
|---|---|---|
| Structured logging | `go.uber.org/zap` | **None** — raw `fmt.Printf("warning: …")` scattered in services |
| Telemetry / tracing | OTel API (scaffolded) | **None** |
| Service interfaces | `internal/interfaces` per entity | **None** — handlers depend on concrete `*services.OrderService` |
| Decorator seam | Yes (logger + telemetry MW) | **No** — nothing to insert middleware into |
| Log config | `LogConfig{level, format}` via viper | **None** |

Concretely in our code today:
- `internal/handlers/order_handler.go`: `func NewOrderHandler(service *services.OrderService)` — a **concrete**
  dependency. There is no interface to decorate.
- Services log with `fmt.Printf` (e.g. `internal/services/guest_auth_service.go`,
  `backoffice_*_service.go`) — unstructured, unleveled, no timing, no request correlation.

So adopting the pattern requires **introducing the interface seam first**, then the two decorator layers, then
init — in that order.

---

## 3. Adoption plan for `lodge-system`

We can adopt this incrementally, one service at a time, without a big-bang rewrite. Recommended phasing:

### Phase 0 — Foundations (once)

1. **Add dependencies:**
   ```
   go get go.uber.org/zap
   go get go.opentelemetry.io/otel go.opentelemetry.io/otel/sdk go.opentelemetry.io/otel/trace
   # for a real exporter later, e.g. OTLP:
   go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
   ```

2. **Create `internal/logger/logger.go`** (mirror risk_audit's `utils/logger`), returning a configured
   `*zap.Logger` from our existing config, and **call `zap.ReplaceGlobals(l)` at startup** (fixing gap #1 above)
   so both injected loggers and any `zap.L()` decorator calls share one configured logger.

3. **Initialise logging in `cmd/api/main.go`:** build the logger, replace globals, log a structured startup
   line, and thread the logger into the handler constructors.

4. **(Optional, when we want real traces) Initialise OTel** in `main.go`: create a TracerProvider with an
   exporter (start with a stdout exporter for local dev, OTLP for prod), `otel.SetTracerProvider(tp)`, and add
   an **HTTP tracing middleware** that starts a span per request and injects it into the context (fixing gap #2).
   Until this exists, telemetry decorators are harmless no-ops — so we can add the decorators first and light up
   tracing later.

### Phase 1 — Introduce the interface seam (per service)

For a target service (start with a high-value one, e.g. **OrderService** or **InvoiceService**):

1. Create `internal/interfaces/order_interface.go` declaring an `OrderInterface` whose methods match the
   service's public methods. **Add `context.Context` as the first parameter** to each method (we'll need to
   thread context through the service anyway for telemetry to work — see the caveat below).
2. Make `*services.OrderService` satisfy it (it already has the methods; we add the `ctx` param).
3. Change `handlers.NewOrderHandler` to take `interfaces.OrderInterface` instead of `*services.OrderService`.

> **Caveat / biggest lift:** our services currently do **not** take `context.Context`. `risk_audit`'s entire
> pattern hinges on `ctx` being the first arg of every method (that's how the span and request-scoped values
> travel). Threading `context.Context` through our service + repository methods is the largest part of this
> work. We can do it service-by-service. Handlers already have `r.Context()` available to pass in.

### Phase 2 — Add the decorators (per service)

1. `internal/middleware/logger/order_logger.go` — `OrderLoggerMiddleware` wrapping `interfaces.OrderInterface`,
   timing each method and logging `zap.L().Info(method, zap.Duration("took", …))` (plus, better than
   risk_audit, `zap.Error(err)` on failure and any key IDs like `order_id`).
2. `internal/middleware/telemetry/order_telemetry.go` — `OrderTelemetryMiddleware` wrapping the same interface,
   `trace.SpanFromContext(ctx).AddEvent(...)` per method.

### Phase 3 — Wire it up (per service)

In `cmd/api/main.go` (our assembly point — we don't have a `server.go` service-wiring block like risk_audit,
we wire in `main.go`):

```go
orderSvc := services.NewOrderService(orderRepo, invoiceRepo, bookingRepo, auditLogRepo)

var orderIface interfaces.OrderInterface = orderSvc
orderIface = logger.NewOrderLoggerMiddleware(orderIface)
orderIface = telemetry.NewOrderTelemetryMiddleware(orderIface)

orderHandler := handlers.NewOrderHandler(orderIface)
```

### Phase 4 — Repeat & standardise

- Roll the same three steps across services in priority order.
- Optionally add a small **code generator** or a scaffolding script: given an interface, emit the boilerplate
  logger + telemetry decorators (they're 100% mechanical — this is exactly the kind of repetitive code that a
  generator or an editor macro handles well, and it's why risk_audit has ~70 near-identical files per layer).

---

## 4. Trade-offs & recommendations

**What's genuinely worth adopting:**
- **Structured logging with zap** — high value, low risk. Replaces our `fmt.Printf` soup with leveled,
  queryable, JSON logs. We can start using it in handlers/services immediately, independent of interfaces.
- **The interface seam** — enables the decorator pattern *and* makes services unit-testable with mocks. Good
  architectural hygiene.
- **Per-method timing logs** — cheap, and instantly useful for spotting slow operations.

**What to weigh carefully:**
- **Verbosity.** risk_audit has one interface + one logger file + one telemetry file **per entity** — ~70×3
  files of near-identical boilerplate. For our ~20 services that's ~60 extra files. Mitigate with a generator,
  or only decorate the services where observability actually pays off (orders, invoices, bookings, auth,
  payments) rather than every CRUD entity.
- **Context threading is the real cost.** Adding `context.Context` everywhere is the bulk of the effort. It's
  worth it (cancellation, deadlines, tracing, request-scoped values), but it's a broad, mechanical change.
- **Don't ship the two risk_audit gaps.** If we adopt telemetry, actually initialise the OTel SDK + a request
  span middleware, and call `zap.ReplaceGlobals` — otherwise we'd be copying dead scaffolding.

**Suggested minimal first step (highest ROI, lowest risk):**
1. Add zap, create `internal/logger`, `zap.ReplaceGlobals` in `main.go`.
2. Replace the `fmt.Printf` warnings in services with `zap.L()` structured logs.
3. Pick **one** service (OrderService) and take it fully through Phases 1–3 as a reference implementation the
   rest of the team can copy.
4. Defer OTel tracing until we have somewhere to send spans (a collector/Jaeger/Tempo) — add the telemetry
   decorators opportunistically since they're no-ops until the SDK is initialised.

---

## 5. File-by-file cheat sheet (risk_audit → lodge-system mapping)

| risk_audit | lodge-system equivalent to create |
|---|---|
| `internal/utils/logger/logger.go` | `internal/logger/logger.go` |
| `internal/interfaces/<entity>_interface.go` | `internal/interfaces/<entity>_interface.go` |
| `internal/middleware/logger/<entity>_logger.go` | `internal/middleware/logger/<entity>_logger.go` |
| `internal/middleware/telemetry/<entity>_telemetry.go` | `internal/middleware/telemetry/<entity>_telemetry.go` |
| wiring in `internal/server/server.go` | wiring in `cmd/api/main.go` |
| logger init in `cmd/app/main.go` | logger + OTel init in `cmd/api/main.go` |
