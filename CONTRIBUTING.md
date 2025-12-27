# Contributing to PUBSUB

Thank you for your interest in contributing to this learning-oriented pub/sub implementation! This document provides guidelines for contributing to the project.

## Code of Conduct

This project follows a simple code of conduct: be respectful, be professional, and be constructive. We welcome contributions from developers of all experience levels.

## Getting Started

### Prerequisites

- Go 1.23.4 or later
- Docker (for integration tests)
- Git

### Setting Up Your Development Environment

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/pubsub.git
   cd pubsub
   ```

3. Add the upstream repository as a remote:
   ```bash
   git remote add upstream https://github.com/mateusf777/pubsub.git
   ```

4. Build the project:
   ```bash
   ./build.sh
   ```

5. Run the tests:
   ```bash
   ./test.sh
   ```

## Development Workflow

### Creating a Branch

Create a feature branch for your work:
```bash
git checkout -b feature/your-feature-name
```

Use descriptive branch names:
- `feature/` - for new features
- `fix/` - for bug fixes
- `docs/` - for documentation changes
- `refactor/` - for code refactoring
- `test/` - for test improvements

### Making Changes

1. Make your changes in your feature branch
2. Follow the Go coding standards:
   - Run `gofmt -s -w .` to format your code
   - Run `go vet ./...` to check for common mistakes
   - Run `staticcheck ./...` if available
3. Add or update tests as necessary
4. Ensure all tests pass
5. Update documentation if needed

### Commit Messages

Write clear, descriptive commit messages:
- Use the imperative mood ("Add feature" not "Added feature")
- Keep the first line under 50 characters
- Add a blank line, then a more detailed description if needed
- Reference issues and pull requests where appropriate

Example:
```
Add queue group load balancing

Implement round-robin distribution of messages to subscribers
in the same queue group. This ensures messages are distributed
evenly across multiple consumers.

Fixes #123
```

### Testing

- Write unit tests for new functionality
- Ensure existing tests continue to pass
- For the core, server, and client modules, tests require mockery-generated mocks
- Integration tests require Docker

Run tests:
```bash
# Unit tests (requires mockery)
cd core && mockery && go test -race -cover
cd ../server && mockery && go test -race -cover
cd ../client && mockery && go test -race -cover

# Integration tests (requires Docker)
cd ../example/integration && go test -race
```

### Code Review

All submissions require review. We use GitHub pull requests for this purpose.

#### Pull Request Process

1. Update your branch with the latest upstream changes:
   ```bash
   git fetch upstream
   git rebase upstream/master
   ```

2. Push your branch to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```

3. Create a pull request on GitHub:
   - Provide a clear title and description
   - Reference related issues
   - Describe the changes and their purpose
   - Include any relevant test results

4. Address review feedback:
   - Make requested changes
   - Push updates to your branch
   - Respond to reviewer comments

5. Once approved, a maintainer will merge your PR

## What to Contribute

### Good First Issues

Look for issues labeled `good first issue` - these are suitable for newcomers.

### Ideas for Contribution

- **Bug fixes**: Found a bug? Submit a fix!
- **Documentation**: Improve README, add examples, clarify confusing parts
- **Tests**: Increase test coverage, add edge case tests
- **Performance**: Optimize hot paths, reduce allocations
- **Features**: Propose and implement new features (discuss first in an issue)
- **Examples**: Add usage examples or tutorials

### What NOT to Contribute

- Breaking changes without prior discussion
- Changes that significantly alter the learning-focused nature of the project
- Features that would make the project production-ready (this is intentionally a learning tool)

## Code Style

Follow standard Go conventions:
- Use `gofmt` for formatting
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Write idiomatic Go code
- Keep functions focused and small
- Use meaningful variable names
- Add comments for exported functions and non-obvious logic
- Use interfaces for abstraction where appropriate

## Project Structure

```
.
├── core/          # Core protocol handling (shared by client and server)
├── server/        # Server implementation
│   └── cmd/       # Server main entry point
├── client/        # Client library
├── example/       # Example applications and integration tests
├── build/         # Build output directory
└── .github/       # CI/CD workflows
```

## Module Organization

The project uses Go modules with local replace directives:
- `core`: Shared protocol and connection handling
- `server`: Server-side pub/sub engine and message routing
- `client`: Client library for connecting to the server
- `example`: Example applications and integration tests

## Testing Strategy

1. **Unit Tests**: Test individual functions and methods
2. **Integration Tests**: Test end-to-end scenarios with Docker
3. **Race Detection**: Run tests with `-race` flag
4. **Coverage**: Aim for meaningful coverage, not just high percentages

## Questions or Need Help?

- Open an issue with your question
- Check existing issues and pull requests
- Review the README for project overview and examples

## License

By contributing, you agree that your contributions will be licensed under the same license as the project (see LICENSE file).

## Recognition

Contributors will be recognized in the project. Thank you for helping make this learning project better!
