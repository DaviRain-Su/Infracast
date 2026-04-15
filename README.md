# Infracast

Infracast is a Code-First infrastructure automation tool that deploys [Encore](https://encore.dev) applications to Alibaba Cloud (Alicloud) with a single command.

## Features

- **Single-Command Deploy**: `infracast deploy --env dev` handles build, push, provision, and health check
- **Infrastructure as Config**: Define resources in `infracast.yaml` — VPC, RDS, Redis, OSS
- **Automatic Rollback**: Failed health checks trigger rollback to previous version
- **Audit Trail**: All operations logged with trace IDs and structured error codes
- **Deployment Notifications**: Feishu and DingTalk webhook support

## Quick Start

### Installation

```bash
# Build from source
git clone https://github.com/DaviRain-Su/Infracast.git
cd Infracast
make build
./bin/infracast version
```

Or download a pre-built binary from [Releases](https://github.com/DaviRain-Su/Infracast/releases).

### Deploy in 5 Steps

```bash
infracast init my-app --provider alicloud --region cn-hangzhou -y
cd my-app
infracast provision --env dev
infracast deploy --env dev
infracast status --env dev
```

See [Getting Started](docs/getting-started.md) for the full walkthrough.

### Prerequisites

Before your first deploy, verify your environment: [Prerequisites Checklist](docs/prerequisites-checklist.md).

## Documentation

| Document | Description |
|----------|-------------|
| [Getting Started](docs/getting-started.md) | 5-step quickstart guide |
| [Deployment Manual](docs/deployment-manual.md) | Full command flow with expected outputs |
| [Error Code Matrix](docs/error-code-matrix.md) | All 78 error codes with fixes |
| [Operations Runbook](docs/runbook.md) | Alerting, rollback, cleanup, incident response |
| [Prerequisites Checklist](docs/prerequisites-checklist.md) | Account, credentials, tools, quota checks |
| [Demo Script](docs/demo-script.md) | 10-15 minute demo with talking points |
| [Examples](examples/README.md) | 6 example applications |

## Configuration

Create an `infracast.yaml` in your project root:

```yaml
app_name: my-app
provider: alicloud
region: cn-hangzhou

environments:
  dev:
    description: Development environment

resources:
  sql_servers:
    main:
      instance_class: pg.n2.medium.1
      storage: 20
  redis:
    cache:
      node_type: redis.master.small.default
```

## Command Reference

| Command | Description |
|---------|-------------|
| `infracast init <app>` | Initialize a new project |
| `infracast env create <name>` | Create an environment |
| `infracast env list` | List environments |
| `infracast provision --env <env>` | Provision cloud resources |
| `infracast deploy --env <env>` | Build, push, deploy, and verify |
| `infracast status --env <env>` | Show infrastructure status |
| `infracast logs` | View audit logs |
| `infracast destroy --env <env>` | Destroy cloud resources |
| `infracast run` | Start local development server |
| `infracast version` | Show version info |

## Development

### Prerequisites

- Go 1.22+
- [Encore CLI](https://encore.dev/docs/install)
- Docker
- Alibaba Cloud account ([Prerequisites](docs/prerequisites-checklist.md))

### Build & Test

```bash
make build       # Build CLI binary
make test        # Run all tests
make fmt         # Format code
make vet         # Run go vet
make regression  # Run regression suite
make release     # Build release archives with checksums
```

## Architecture

- [PRD](docs/01-prd.md)
- [Architecture](docs/02-architecture.md)
- [Technical Spec](docs/03-technical-spec.md)

## License

MPL-2.0
