# Getting Started with Infracast

This guide will walk you through deploying your first Encore application using Infracast.

## Prerequisites

- [Go](https://golang.org/dl/) 1.22 or later
- [Encore CLI](https://encore.dev/docs/install)
- [Infracast CLI](../README.md#installation)
- Alibaba Cloud account with access credentials

## 5-Step Quickstart

### Step 1: Initialize Your Project

Create a new Infracast project:

```bash
infracast init my-app --provider alicloud --region cn-hangzhou
```

This creates:
- `infracast.yaml` - Project configuration
- `.infra/` - Infrastructure state directory
- `.gitignore` - Git ignore rules
- `README.md` - Project documentation

### Step 2: Configure Resources

Edit `infracast.yaml` to define your infrastructure:

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

### Step 3: Provision Infrastructure

Create the cloud resources:

```bash
infracast provision --env dev
```

This will:
- Create VPC and networking
- Provision RDS PostgreSQL instance
- Create Redis cache cluster
- Generate `infracfg.json` with connection details

### Step 4: Deploy Your Application

Build and deploy your Encore app:

```bash
infracast deploy --env dev
```

The deployment process:
1. Builds Docker image (`encore build docker`)
2. Pushes to container registry
3. Deploys to Kubernetes
4. Verifies health checks

### Step 5: Verify Deployment

Check your deployment status:

```bash
# View status
infracast status --env dev

# View logs
infracast logs --env dev

# Open application URL
infracast open --env dev
```

## Next Steps

### Add More Environments

```bash
# Create staging environment
infracast env create staging --provider alicloud --region cn-shanghai

# Deploy to staging
infracast deploy --env staging
```

### Configure Notifications

Add webhook notifications for deployments:

```yaml
# infracast.yaml
notifications:
  feishu:
    webhook_url: "https://open.feishu.cn/..."
```

### Learn More

- [Configuration Reference](configuration.md)
- [Deployment Guide](deployment.md)
- [Troubleshooting](troubleshooting.md)

## Example Applications

Check out the [examples](../examples/) directory for complete sample applications:

- [hello-world](../examples/hello-world/) - Minimal example
- [todo-app](../examples/todo-app/) - Todo app with database
- [blog-api](../examples/blog-api/) - Blog API with OSS uploads
