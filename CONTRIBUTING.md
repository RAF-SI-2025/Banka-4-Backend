# Contributing Guide

## Branch Naming

All branches follow the pattern `type/description`:

| Type      | Purpose                          | Example                      |
|-----------|----------------------------------|------------------------------|
| `feature` | New functionality                | `feature/user-auth`          |
| `bugfix`  | Fixing a bug                     | `bugfix/login-null-pointer`  |
| `hotfix`  | Urgent fix for production        | `hotfix/db-connection-leak`  |
| `chore`   | Maintenance, refactoring, config | `chore/update-dependencies`  |
| `docs`    | Documentation changes            | `docs/api-endpoints`         |
| `test`    | Adding or fixing tests           | `test/user-service-unit`     |

Rules:
- Use lowercase and hyphens: `feature/user-auth` not `Feature/UserAuth`.
- Should be short but descriptive enough.

---

## Commit Messages

This project uses [Conventional Commits](https://www.conventionalcommits.org/).

### Format

```
type(scope): description

[optional body]
```

### Types

| Type       | When to use                                     |
|------------|--------------------------------------------------|
| `feat`     | Adding new functionality                         |
| `fix`      | Fixing a bug                                     |
| `refactor` | Code change that neither fixes a bug nor adds a feature |
| `docs`     | Documentation only                               |
| `test`     | Adding or updating tests                         |
| `chore`    | Build, CI, dependencies, config                  |
| `style`    | Formatting, whitespace (no logic change)         |
| `perf`     | Performance improvement                          |
| `ci`       | CI/CD pipeline changes                           |

### Scope

The scope is optional but recommended. Use the service or package name:

```
feat(user-service): add login endpoint
fix(common/errors): handle nil pointer in ErrorHandler
chore(user-service): update gorm dependency
refactor(user-service/repository): extract common query logic
```

### Examples

```
feat(user-service): add user registration endpoint

fix(common/errors): return correct status code for conflict errors

refactor(user-service): extract password hashing to separate util

chore: update go.work with new service

test(user-service): add unit tests for user service layer

docs: update ARCHITECTURE.md with repository patterns
```

### Rules

- Imperative speech: "add feature" not "added feature".
- Lowercase, no period: `feat: add login` not `feat: Add login.`
- Keep the commit name as short as possible

---