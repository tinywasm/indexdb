---
PLAN: "refactor!: indexdb implementa storage.Conn (contrato movido de orm a tinywasm/storage) — solo DML"
TAG: v0.4.0
---

# PLAN — `tinywasm/indexdb`: migrar de `*orm.DB` a `storage.Conn`

Orquestado por
[`DB_PORT_MASTER_PLAN.md`](https://github.com/tinywasm/app/blob/main/docs/DB_PORT_MASTER_PLAN.md)
— **pieza #6**. Autocontenido, en español. **Solo tienes este repo** (`github.com/tinywasm/indexdb`).

> **Prerequisito:** `go install github.com/tinywasm/devflow/cmd/gotest@latest` (maneja el arnés WASM).
> Tests con `gotest`. Publica con `gopush 'mensaje'`.
> Este plan **requiere `tinywasm/storage@v0.0.1` ya publicado**. Si no resuelve en `go get`, para y
> repórtalo. **No** necesitas `tinywasm/ddl` — `indexdb` nunca hace DDL SQL (ver §1).

## 0. Qué cambió respecto a la versión anterior de este plan

Antes: `indexdb` iba a probar `orm/conformance` (`orm` seguía siendo dueño del contrato DML). Ya no lo
es — se extrajo a `tinywasm/storage`. Ahora:

- `adapter` implementa **`storage.Conn`** (Executor+Compiler unidos) en vez de `orm.Executor`+`orm.Compiler`
  vía `orm.New(adapter, compiler)`.
- `New(dbName, idg, logger, structTables...)` devuelve **`storage.Conn`**, no `*orm.DB`.
- Se prueba contra **`storage/conformance`** (no `orm/conformance`).
- `go.mod` final: `storage`+`model`+`fmt`+`jsvalue`. **Cero `tinywasm/orm`.**
- Este repo **nunca** implementó DDL (su esquema son `structTables` declarados por adelantado, no
  `CREATE TABLE`), así que no gana ninguna dependencia de `tinywasm/ddl` en este cambio — sigue sin
  tenerla, igual que en el plan anterior.

## 1. Qué se hace y por qué

El puerto `tinywasm/storage` reemplaza el contrato que antes vivía en `orm`. `indexdb` entra **solo** en su
mitad DML (`storage/conformance`) — su esquema no es DDL SQL: los object stores se declaran por adelantado
en `New(...structTables)` durante `onUpgradeNeeded`. Eso no cambia con este plan; lo único que cambia
es de qué paquete vienen los tipos del contrato.

## 2. Estado verificado (código actual, antes de este plan)

- `adapter.go:18` `type adapter struct{...}` implementa `orm.Executor` (`Exec`/`QueryRow`/`Query`/
  `Close`, líneas 31-240).
- `adapter.go:56,64` `simpleScanner`/`QueryRow` devuelven `orm.Scanner`.
- `adapter.go:85-184` `simpleRows` implementa `orm.Rows`.
- `adapter.go:263-265` `type compiler struct{}` con `Compile(q orm.Query, m Model) (orm.Plan, error)`
  — `Model` aquí es el tipo local del propio `indexdb` (no cambia con este plan, no está relacionado
  con el split DDL/DML — no lo toques salvo para lo que exige la firma nueva de `Compile`).
- `adapter.go:271` `New(dbName string, idg idGenerator, logger func(...any), structTables ...any)
  *orm.DB` → hoy hace `orm.New(adapter, compiler)` internamente.
- `execute.go`: `execute`/`create`/`update`/`delete`/`readOne`/`readAll`/`checkCondition` — todos
  tipados sobre `orm.Query`/`orm.Condition`. El filtrado en memoria (`Conditions`, `AND`/`OR`,
  `OrderBy`, `Limit`, `Offset`) vive aquí — **su lógica no cambia**, solo los tipos.
- `tests/setup_test.go` ya define `idGenerator` (counter) y `SetupDB(logger func(...any), dbName
  string, structTables ...any) *orm.DB` — se adapta a devolver `storage.Conn` (§3.3).
- `go.mod`: `orm@v0.9.26`, `jsvalue@v0.0.14`, `model@v0.0.6`.

## 3. Cambios

### 3.1 `go.mod`

```
go get github.com/tinywasm/storage@v0.0.1
go mod tidy   # quita github.com/tinywasm/orm por completo
```

### 3.2 `adapter.go` — `storage.Conn`, sin cambios de lógica interna

Cambia únicamente los tipos de las firmas — el cuerpo de cada método (la parte que habla con
IndexedDB vía `js.Value`) no cambia:

```go
// Exec/QueryRow/Query/Close: sin cambios de cuerpo, solo firmas:
func (d *adapter) Exec(query string, args ...any) error                  { /* sin cambios */ }
func (d *adapter) QueryRow(query string, args ...any) storage.Scanner    { /* sin cambios */ }
func (d *adapter) Query(query string, args ...any) (storage.Rows, error) { /* sin cambios */ }
func (d *adapter) Close() error                                          { /* sin cambios */ }

// simpleScanner/simpleRows: mismo cuerpo, implementan storage.Scanner/storage.Rows en vez de orm.*.

type compiler struct{}

func (c *compiler) Compile(q storage.Query, m Model) (storage.Plan, error) { /* sin cambios de cuerpo */ }

// adapter debe implementar Compile también, delegando al compiler embebido — storage.Conn exige
// Executor+Compiler en el MISMO valor (antes orm.New(adapter, compiler) los tomaba por
// separado). La forma más simple: que adapter tenga un campo compiler y delegue.
func (d *adapter) Compile(q storage.Query, m Model) (storage.Plan, error) {
	return d.compiler.Compile(q, m)
}

// New builds a fresh IndexedDB-backed storage.Conn. dbName must be unique per independent database
// (tests use one dbName per conformance clause for isolation — see setup_test.go, §3.3).
func New(dbName string, idg idGenerator, logger func(...any), structTables ...any) storage.Conn {
	a := newAdapter(dbName, idg, logger)
	a.compiler = &compiler{} // nuevo campo — ver arriba
	a.initialize(structTables...)
	return a
}

var _ storage.Conn = (*adapter)(nil)
```

> **Ajusta `newAdapter`/`type adapter struct`** para añadir el campo `compiler *compiler` si no lo
> tenía ya (antes `compiler` vivía suelto, pasado como segundo argumento a `orm.New`; ahora vive
> dentro de `adapter` porque `storage.Conn` necesita un solo valor que sea ambas cosas). No inventes un
> segundo tipo — es el mismo `compiler{}` de siempre, solo que `adapter` lo guarda y delega.

### 3.3 `execute.go` — solo tipos, cero cambios de lógica de filtrado

```go
func (d *adapter) execute(q storage.Query, m Model, factory func() Model, each func(Model), eachJS func(js.Value)) error { /* sin cambios */ }
func (d *adapter) create(q storage.Query, m Model) error  { /* sin cambios */ }
func (d *adapter) update(q storage.Query, m Model) error  { /* sin cambios */ }
func (d *adapter) delete(q storage.Query, m Model) error  { /* sin cambios */ }
func (d *adapter) readOne(q storage.Query, m Model) error { /* sin cambios */ }
func (d *adapter) readAll(q storage.Query, factory func() Model, each func(Model), eachJS func(js.Value)) error { /* sin cambios */ }
func checkCondition(val js.Value, cond storage.Condition) bool { /* sin cambios */ }
```

`storage.Action`/`storage.Query`/`storage.Condition` tienen exactamente los mismos campos/getters que sus
equivalentes `orm.*` de antes (`Action`/`Table`/`Columns`/`Values`/`Conditions`/`OrderBy`/`Limit`/
`Offset`, `Field()`/`Operator()`/`Value()`/`Logic()`) — este archivo es un cambio de import, no de
comportamiento.

### 3.4 `tests/setup_test.go` — `SetupDB` devuelve `storage.Conn`

```go
func SetupDB(logger func(...any), dbName string, structTables ...any) storage.Conn {
	return New(dbName, &counterIDGen{}, logger, structTables...)
}
```

### 3.5 `tests/conformance_test.go` (`//go:build wasm`, `package tests_test`)

```go
//go:build wasm

package tests_test

import (
	"fmt"
	"testing"

	"github.com/tinywasm/storage"
	dbconf "github.com/tinywasm/storage/conformance"
	"github.com/tinywasm/model"
)

func TestIndexDB_DBConformance(t *testing.T) {
	var n int
	dbconf.Run(t, dbconf.Factory{
		Name: "indexdb",
		New: func(t *testing.T, models ...model.Model) storage.Conn {
			n++
			dbName := fmt.Sprintf("conformance_db_%d", n) // fresh IndexedDB per clause
			structs := make([]any, len(models))
			for i, m := range models {
				structs[i] = m // declared as object stores up front
			}
			return SetupDB(func(...any) {}, dbName, structs...) // from tests/setup_test.go
		},
	})
}
```

> La suite **nunca** llama DDL: entrega la tabla lista y solo hace `Create`/`Query`. `indexdb` la
> "crea" declarando el store vía `structTables` en `New` — encaja con el slot `models` del `Factory`.
> Respeta el `fmt` que use el paquete de tests actual (puede ser stdlib `fmt` aquí, ya que es
> test-only y no compila a producción — pero si el resto del repo usa `tinywasm/fmt`, sé
> consistente).

## 4. Trabajo real esperado (la conformance revelará diferencias en `execute.go`/`adapter.go`)

Nunca la suite ni el modelo `Widget`. Puntos (sin cambios respecto al plan anterior, siguen siendo
válidos con los tipos nuevos):

- **Sync sobre async**: `storage.Executor` es **síncrono**; el adapter debe **bloquear** sobre los
  callbacks de IndexedDB (canales, como `initialize` con `<-d.initDone`) hasta tener el resultado. Si
  no bloquea, las cláusulas ven datos incompletos o cuelgan.
- **Filtrado en memoria**: `execute.go` debe honrar `Conditions` (`= != > >= < <= IN LIKE`), lógica
  `AND`/`OR`, `OrderBy Asc/Desc`, `Limit`, `Offset`.
- **`ReadOne` sin match ⇒ `storage.ErrNoRows`** (antes `orm.ErrNoRows`): el scanner debe devolver ese
  sentinela nuevo.
- **Tipos**: `qty int64` y `active bool` round-trip correcto a/desde valores JS.

## 5. Criterios de aceptación

- `adapter` implementa `storage.Conn` (`var _ storage.Conn = (*adapter)(nil)`). **Cero**
  `github.com/tinywasm/orm` (`grep -rn "tinywasm/orm" .` vacío).
- `New(dbName, idg, logger, structTables...) storage.Conn` (antes `*orm.DB`).
- `tests/conformance_test.go` (`//go:build wasm`) corre `dbconf.Run` con `SetupDB` + `dbName` único +
  `models` como `structTables`. Sin referencias a `ddl`/`ddl/conformance` (no le aplica).
- `gotest` (WASM) verde: todas las cláusulas DML pasan; tests WASM existentes siguen verdes.
- `go.mod` en `storage@v0.0.1`+; `go mod tidy` limpio; publicado con `gopush`.

## 6. Etapas

| # | Etapa | Archivo(s) | Criterio |
|---|---|---|---|
| 1 | Bump storage, quitar orm | `go.mod` | `storage@v0.0.1`+ con `storage/conformance`; `orm` fuera |
| 2 | `adapter` como `storage.Conn` | `adapter.go` | `var _ storage.Conn`, `New(...) storage.Conn` (§3.2) |
| 3 | Ejecución (solo tipos) | `execute.go` | `storage.Query`/`storage.Condition` (§3.3) |
| 4 | Test helpers | `tests/setup_test.go` | `SetupDB(...) storage.Conn` (§3.4) |
| 5 | Test de conformidad | `tests/conformance_test.go` | `dbconf.Run`, dbName único, structTables (§3.5) |
| 6 | Correcciones (probable) | `execute.go`/`adapter.go` | sync-sobre-async + filtrado (§4) |
| 7 | Publicar | — | `gotest` WASM verde; `gopush 'refactor!: storage.Conn'` |

## 7. Cierre

Tras `gopush`, **borra** `docs/PLAN.md`; correcciones de comportamiento → `README.md`.
