# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- SECURITY.md with security policy and best practices
- CONTRIBUTING.md with contribution guidelines
- CHANGELOG.md to track project changes
- Makefile for common development tasks
- .env.example for configuration reference
- Comprehensive project documentation

### Changed
- Improved error handling with context
- Enhanced documentation and code comments

### Security
- Document TLS best practices
- Add security scanning recommendations

## [0.1.0] - Initial Release

### Added
- Basic pub/sub protocol implementation (inspired by NATS)
- TCP server and client
- PUB, SUB, UNSUB, STOP, PING/PONG message handling
- Queue groups for load-balanced subscriptions
- TLS support for encrypted communication
- Multi-tenancy with certificate-based authentication
- Integration tests with Docker
- CI/CD pipeline with GitHub Actions
- Example applications (publisher, subscriber, queue, request/reply)
- Comprehensive README with usage examples

### Features
- Lightweight protocol with line-based commands
- Connection keep-alive mechanism
- Request/reply pattern support
- Graceful shutdown handling
- Race detection in tests
- Code coverage reporting

[Unreleased]: https://github.com/mateusf777/pubsub/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/mateusf777/pubsub/releases/tag/v0.1.0
