# Project Standards & Conventions

This document outlines the architectural patterns, coding standards, and best practices strictly followed in this repository (`boilerplate-golang`).

## 1. Architecture: Clean Architecture

The project follows a **Clean Architecture** structure with unidirectional dependencies:
`Handler (Delivery) -> Usecase (Business Logic) -> Repository (Data Access)`
 
### Directory Structure
- `cmd/`: Entry points (e.g., `cmd/api/main.go`).
-- for application service in `cmd/app/main.go`
-- for worker service in `cmd/worker/main.go`
-- for api service in `cmd/api/main.go`
- `internal/http/handler/`: HTTP handlers (Controllers).
- `internal/usecase/`: Business logic.
- `internal/repository/`: Data access layer (Interfaces and Implementations).
- `internal/repository/mysql/`: MySQL specific implementations.
- `internal/repository/mysql/entity/`: Database/ORM models.
- `internal/http/middleware/`: Application middleware.
- `entity/`: Shared domain entities/DTOs.
- `config/`: Configuration loading.
- `internal/views/`: HTML Views (use feature based layouting!).

## 2. Dependency Injection

- **Manual Injection**: Dependencies are injected via constructor functions (e.g., `NewAuthHandler`, `NewUserRepository`).
- **Interfaces**: Layers depend on interfaces, not concrete structs, enabling easy mocking and testing.

## 3. Libraries & Tools

- **Web Framework**: [Fiber v2](https://github.com/gofiber/fiber)
- **ORM**: [GORM](https://gorm.io/)
- **Testing**:
    - [Testify](https://github.com/stretchr/testify) (Assertions & Suits)
    - [Go-SQLMock](https://github.com/DATA-DOG/go-sqlmock) (Database Mocking)
    - [Mockery](https://github.com/vektra/mockery) (Interface Mocking)
- **Error Handling**: `github.com/pkg/errors` for wrapping errors with context.
- **Logging**: Custom `logger_pkg.go` in `internal/pkg`.

## 4. Coding Standards

### Naming Conventions
- **Files**: Snake_case (e.g., `auth_handler.go`, `todo_list_usecase.go`).
- **Structs**: PascalCase (e.g., `AuthHandler`, `UserRepository`).
- **Interfaces**: PascalCase, prefixed with `I` (e.g., `IAuthHandler`, `IUserRepository`).
- **Functions**: PascalCase for exported, camelCase for internal.
- **Variables**: camelCase (e.g., `userUsecase`, `todoListRepo`).

### Error Handling
- **Wrap Errors**: Use `errwrap.Wrap(err, funcName)` to add context stack.
- **Sentinel Errors**: Define sentinel errors in `error` package or `apperr` (e.g., `apperr.ErrRecordNotFound()`).
- **Check Specific Errors**: Use `errwrap.Is(err, target)` or `errors.As`.

### Context Propagation
- Always pass `context.Context` as the first argument to internal methods (`Usecase` and `Repository`).
- Respect context cancellation in repositories (use `util.CheckDeadline(ctx)`).

## 5. Layer Implementation Guidelines

### Repository Layer (`internal/repository`)
- **Interface Definition**: Defined in the root of the specific implementation package or `repository` package.
- **Implementation**:
    - Method receiver name: `r` or specific (e.g., `u` for user).
    - **Transaction Support**: Use `TrxSupportRepo` interface and `GormTrxSupport` for transaction management.
    - **Methods**: `GetBy...`, `Create`, `Update`, `Delete`, `LockByID`.
    - **GORM Usage**: Use `Take` for single records, `Find` for slices. Handle `gorm.ErrRecordNotFound`.

### Usecase Layer (`internal/usecase`)
- **Validation**: Validate input structs using `usecase.ValidateStruct` or specific logic *before* processing.
- **Business Logic**: Orchestrate repositories and other services.
- **Response Construction**: Convert domain/DB entities to Response DTOs within the usecase.

### Handler Layer (`internal/http/handler`)
- **Request Parsing**: Use `parser.ParserBodyRequest` to bind inputs.
- **Response**: Use `presenter.BuildSuccess` or `presenter.BuildError`.
- **Swagger**: Add Swaggo comments for API documentation.

## 6. Testing Standards

### Unit Tests
- **Location**: Adjacent to source files (e.g., `user_test.go` next to `user.go`).
- **Framework**: Use `testify/suite`.
- **Naming**: `Test<StructName>_<MethodName>`.
- **Table-Driven Tests**: Use table-driven tests for covering multiple scenarios (Success, NotFound, DBError).
- **Mocking**:
    - Use `go-sqlmock` for GORM/Repository MYSQL tests. 
    - Use interface mocks for Usecase tests (mock inputs from Repositories).
    - Use `make mock` to generate mocks for interfaces.
- **Coverage**: Aim for high code coverage (100% for critical logic).

#### Example Repository Test Pattern
```go
func (s *RepoTestSuite) TestCreate() {
    tests := []struct {
        name    string
        args    args
        mock    func()
        wantErr bool
    }{
        {
            name: "Success",
            // ... args & mock setup
            wantErr: false,
        },
    }
    // ... run loop
}
```

## 7. Version Control & Commits
- **Conventional Commits**: Use descriptive messages (e.g., `feat: add user login`, `fix: handle db connection error`).
- **Branching**: Feature branches merged via PR.