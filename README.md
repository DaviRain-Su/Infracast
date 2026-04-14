# Infracast

Infracast is a Code-First infrastructure automation platform based on the Encore framework, designed specifically for Chinese cloud providers.

## Overview

Infracast enables developers to deploy applications to Chinese clouds (Alicloud, Huawei Cloud, Tencent Cloud, Volcengine) with minimal infrastructure configuration. It bridges the gap between Encore's developer experience and China's cloud ecosystem.

## Features

- **Code-First Infrastructure**: Declare infrastructure in your application code
- **Multi-Cloud Support**: Deploy to Alicloud, Huawei Cloud, Tencent Cloud, and more
- **Local Development**: Full local development environment with infrastructure mocking
- **Automated Deployment**: Git-push deployment with preview environments
- **Observability**: Built-in distributed tracing and metrics

## Quick Start

### Installation

```bash
# Build from source
git clone https://github.com/DaviRain-Su/infracast.git
cd infracast
make build

# Or install directly
make install
```

### Initialize a Project

```bash
infracast init --provider alicloud --region cn-hangzhou
```

### Local Development

```bash
infracast run
```

### Deploy to Cloud

```bash
infracast deploy --env production
```

## Configuration

Create an `infracast.yaml` file in your project root:

```yaml
provider: alicloud
region: cn-hangzhou

environments:
  production:
    provider: alicloud
    region: cn-shanghai
  staging:
    provider: alicloud
    region: cn-hangzhou
```

## Development

### Prerequisites

- Go 1.22+
- Docker (for local development)
- Access to target cloud provider

### Building

```bash
make build
```

### Testing

```bash
make test
```

### Linting

```bash
make lint
```

## Architecture

See [docs/01-prd.md](docs/01-prd.md) for detailed architecture documentation.

## License

MPL-2.0

## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting PRs.
