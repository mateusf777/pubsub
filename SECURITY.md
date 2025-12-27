# Security Policy

## Supported Versions

This project is a **learning-oriented** implementation and is not intended for production use. However, we take security seriously and welcome reports of security vulnerabilities.

| Version | Supported          |
| ------- | ------------------ |
| master  | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability within this project, please send an email to the repository maintainers. All security vulnerabilities will be promptly addressed.

**Please do not report security vulnerabilities through public GitHub issues.**

When reporting a vulnerability, please include:

- Description of the vulnerability
- Steps to reproduce the issue
- Potential impact
- Suggested fix (if available)

## Security Best Practices for Users

If you choose to use this project for learning or experimentation, please follow these security best practices:

### TLS Configuration
- Always use TLS in any network-accessible deployment
- Use strong certificates signed by a trusted CA
- Enable mutual TLS (mTLS) with client certificate verification for production-like scenarios
- Regularly rotate certificates and keys

### Secrets Management
- Never commit secrets (certificates, keys, passwords) to version control
- Use environment variables or secure secret management systems (e.g., HashiCorp Vault, AWS Secrets Manager)
- Follow the principle of least privilege for service accounts

### Network Security
- Run the server behind a firewall or in a private network
- Implement rate limiting and connection limits
- Use network policies in Kubernetes environments

### Container Security
- The project uses distroless base images to minimize attack surface
- Run containers with non-root users
- Set resource limits to prevent resource exhaustion
- Regularly scan container images for vulnerabilities

### Input Validation
- Be aware that this is a learning project and may not have comprehensive input validation
- Validate and sanitize all inputs in production environments
- Implement message size limits

## Known Limitations

As stated in the README, this project is:
- **Under development**
- **For learning purposes only**
- **Not production-ready**

For production use cases, please consider mature alternatives like [NATS](https://nats.io).

## Security Scanning

The project includes GitHub Actions workflows that:
- Build and test the code
- Run integration tests with TLS configurations
- Build container images

We recommend adding the following security scanning tools to your workflow:
- `govulncheck` - Go vulnerability scanner
- `trivy` - Container image vulnerability scanner
- `gosec` - Go security checker
- `gitleaks` - Secrets scanner

## Acknowledgments

We appreciate the security research community's efforts in identifying and responsibly disclosing vulnerabilities.
