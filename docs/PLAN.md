---
PLAN: "test: indexdb prueba orm/conformance (DML) — backend WASM/IndexedDB"
TAG: v0.4.0
---

# PLAN — `tinywasm/indexdb`: probar `orm/conformance` (solo DML)

Orquestado por [`DDL_DML_SPLIT_MASTER_PLAN.md`](https://github.com/tinywasm/app-releases/blob/main/docs/DDL_DML_SPLIT_MASTER_PLAN.md)
— **pieza #5, Ola A**. Autocontenido, en español. **Solo tienes este repo** (`github.com/tinywasm/indexdb`).

> **Prerequisito:** `go install github.com/tinywasm/devflow/cmd/gotest@latest`.
> Tests con `gotest` (maneja el arnés WASM). Publica con `gopush 'mensaje'`.

## 1. Qué se hace y por qué

El split DDL/DML (ver master) parte el contrato de backend en dos: `orm/conformance` (datos) y
`ddl/conformance` (esquema SQL). **`indexdb` entra SOLO en el de DML** — su esquema no es DDL SQL: los
object stores se declaran por adelantado en `New(...structTables)` durante `onUpgradeNeeded`. Esa era
justo la fricción que impedía a `indexdb` conformar como los demás; al sacar el DDL del contrato de
`orm`, `indexdb` prueba exactamente el mismo `orm/conformance` que `mock`/`sqlite`/`postgres`, sin
cláusulas de tabla que no le aplican.

## 2. Estado verificado

- `indexdb.New(dbName string, idg idGenerator, logger func(...any), structTables ...any) *orm.DB`
  (`adapter.go:271`) → `orm.New(adapter, compiler)`. Los stores se crean desde `structTables` en
  `initialize`/`onUpgradeNeeded`; el adapter ejecuta las queries directo (el compiler mete `[q, m]` en
  `Plan.Args`, el filtrado real está en `execute.go`).
- Tests bajo `//go:build wasm`, `package tests_test`. `tests/setup_test.go` ya define `idGenerator`
  (counter) y `SetupDB(logger func(...any), dbName string, structTables ...any) *orm.DB` — **reúsalos**.
- IndexedDB persiste dentro de una corrida; el aislamiento por-cláusula se logra con un `dbName` único
  por cada `Factory.New`.

## 3. Cambios

### 3.1 `go.mod`
```
go get github.com/tinywasm/orm@v0.10.0   # trae orm/conformance (solo DML, wasm-safe)
go mod tidy
```

### 3.2 `tests/conformance_test.go` (`//go:build wasm`, `package tests_test`)

```go
//go:build wasm

package tests_test

import (
	"fmt"
	"testing"

	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	ormconf "github.com/tinywasm/orm/conformance"
)

func TestIndexDB_ORMConformance(t *testing.T) {
	var n int
	ormconf.Run(t, ormconf.Factory{
		Name: "indexdb",
		New: func(t *testing.T, models ...model.Model) *orm.DB {
			n++
			dbName := fmt.Sprintf("conformance_db_%d", n) // fresh IndexedDB per clause
			structs := make([]any, len(models))
			for i, m := range models { structs[i] = m } // declared as object stores up front
			return SetupDB(func(...any) {}, dbName, structs...) // from tests/setup_test.go
		},
	})
}
```

> La suite **nunca** llama DDL: entrega la tabla lista y solo hace `Create`/`Query`. `indexdb` la
> "crea" declarando el store vía `structTables` en `New` — encaja con el slot `models` del `Factory`.
> Respeta el `fmt` que use el paquete de tests actual.

## 4. Trabajo real esperado (la conformance revelará diferencias en `execute.go`/`adapter.go`)

Nunca la suite ni el modelo `Widget`. Puntos:

- **Sync sobre async**: `orm.Executor` es **síncrono**; el adapter debe **bloquear** sobre los callbacks
  de IndexedDB (canales, como `initialize` con `<-d.initDone`) hasta tener el resultado. Si no bloquea,
  las cláusulas ven datos incompletos o cuelgan.
- **Filtrado en memoria**: `execute.go` debe honrar `Conditions` (`= != > >= < <= IN LIKE`), lógica
  `AND`/`OR`, `OrderBy Asc/Desc`, `Limit`, `Offset`. Lo prueban `read_all_ands_two_conditions`,
  `read_all_ors_conditions`, `read_all_orders_asc_and_desc`, `read_all_applies_limit_and_offset`,
  `comparison_operators_filter`, `in_operator_filters`.
- **`ReadOne` sin match ⇒ `orm.ErrNotFound`**: el scanner debe devolver `orm.ErrNoRows`.
- **Tipos**: `qty int64` y `active bool` round-trip correcto a/desde valores JS.

## 5. Criterios de aceptación

- `tests/conformance_test.go` (`//go:build wasm`) corre `ormconf.Run` con `SetupDB` + `dbName` único +
  `models` como `structTables`. Sin referencias a DDL/`ddl/conformance` (a `indexdb` no le aplica).
- `gotest` (WASM) verde: todas las cláusulas DML pasan; tests WASM existentes siguen verdes.
- `go.mod` en `orm@v0.10.0`+; `go mod tidy` limpio; publicado con `gopush`.

## 6. Etapas

| # | Etapa | Archivo(s) | Criterio |
|---|---|---|---|
| 1 | Bump orm | `go.mod` | `orm@v0.10.0`+ con `orm/conformance` |
| 2 | Test de conformidad | `tests/conformance_test.go` | `ormconf.Run`, dbName único, structTables |
| 3 | Correcciones (probable) | `execute.go`/`adapter.go` | sync-sobre-async + filtrado |
| 4 | Publicar | — | `gotest` WASM verde; `gopush 'test: orm conformance'` |

## 7. Cierre

Tras `gopush`, **borra** `docs/PLAN.md`; correcciones de comportamiento → `README.md`.
