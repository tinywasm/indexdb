# Implementation Documentation

This document outlines the API and usage of the `tinywasm/indexdb` adapter for `tinywasm/orm`.

## Overview

The `tinywasm/indexdb` package provides an IndexedDB adapter for the `tinywasm/orm` library, allowing Go applications compiled to WebAssembly to persist data in the browser's IndexedDB storage.

## Installation

```bash
go get github.com/tinywasm/indexdb
```

## Usage

### 1. Define Models

Define your data models as Go structs. These structs must implement the `orm.Model` interface and the `StructName` interface required by the adapter.

```go
type User struct {
    ID    string
    Name  string
    Email string
}

// StructName returns the table/object store name.
func (u User) StructName() string {
    return "users"
}

// ORM Model interface implementation
func (u *User) TableName() string { return "users" }
func (u *User) Columns() []string { return []string{"ID", "Name", "Email"} }
func (u *User) Values() []any     { return []any{u.ID, u.Name, u.Email} }
func (u *User) Pointers() []any   { return []any{&u.ID, &u.Name, &u.Email} }
```

### 2. Initialize Database

Use `indexdb.InitDB` to initialize the database connection and register your models. This function returns an `*orm.DB` instance ready for use.

```go
import (
    "github.com/tinywasm/indexdb"
    "github.com/tinywasm/orm"
)

// IDGenerator implementation (optional, or use a library)
type MyIDGenerator struct{}
func (g *MyIDGenerator) GetNewID() string {
    return "unique-id" // Replace with UUID generation
}

func main() {
    // Logger function
    logger := func(args ...any) {
        println(args...)
    }

    // Initialize DB
    // Arguments: DB Name, ID Generator, Logger, List of Models
    db := indexdb.InitDB("my_app_db", &MyIDGenerator{}, logger, User{})

    // Application logic...
}
```

### 3. CRUD Operations

Perform Create, Read, Update, and Delete operations using the `orm.DB` instance.

#### Create

```go
user := &User{ID: "1", Name: "Alice", Email: "alice@example.com"}
err := db.Create(user)
if err != nil {
    // Handle error
}
```

#### Read One

```go
var readUser User
// Query by ID
err := db.Query(&readUser).Where(orm.Eq("ID", "1")).ReadOne()
if err != nil {
    // Handle error
}
```

#### Read All (with conditions)

```go
// Factory function to create new instances
factory := func() orm.Model { return &User{} }

// Callback for each result
each := func(m orm.Model) {
    u := m.(*User)
    println("Found user:", u.Name)
}

err := db.Query(&User{}).Where(orm.Eq("Name", "Alice")).ReadAll(factory, each)
```

#### Update

```go
user.Email = "newemail@example.com"
err := db.Update(user, orm.Eq("ID", "1"))
```

#### Delete

```go
err := db.Delete(user, orm.Eq("ID", "1"))
```

## key Features

- **WebAssembly Support**: Designed specifically for `GOOS=js GOARCH=wasm`.
- **IndexedDB Integration**: Uses `syscall/js` to interact directly with the browser's IndexedDB API.
- **ORM Compatibility**: Fully implements the `orm.Adapter` interface.
- **Automatic Table Creation**: Automatically creates Object Stores based on registered models during initialization.
