# E-commerce App Deployment Manual

## Prerequisites

- Infracast CLI installed (`infracast version`)
- Encore CLI installed (`encore version`)
- Access to Alibaba Cloud with appropriate permissions
- kubectl configured (for verification)

## Environment Setup

### 1. Create Production Environment

```bash
# Create production environment configuration
infracast env create production \
  --provider alicloud \
  --region cn-hangzhou
```

### 2. Configure Credentials

```bash
# Set up Alibaba Cloud credentials
infracast config set-credentials alicloud \
  --access-key-id $ALICLOUD_ACCESS_KEY_ID \
  --access-key-secret $ALICLOUD_ACCESS_KEY_SECRET
```

### 3. Configure Notifications (Optional)

```bash
# Configure Feishu webhook for deployment notifications
infracast config set-notification feishu \
  --webhook-url "https://open.feishu.cn/open-apis/bot/v2/hook/..."
```

## Deployment Steps

### Step 1: Initialize Project

```bash
# Clone or create the e-commerce app
cd examples/ecommerce-app

# Initialize with Infracast
infracast init ecommerce-demo
```

### Step 2: Review Configuration

```bash
# View the generated configuration
infracast env show production
```

### Step 3: Provision Infrastructure

```bash
# Provision all resources (takes ~15-20 minutes)
infracast provision --env production --verbose

# Expected output:
# [1/6] Checking existing resources...
# [2/6] Creating VPC and VSwitch...
# [3/6] Creating RDS instances (users-db, products-db, orders-db)...
# [4/6] Creating Redis instances (session-cache, inventory-cache)...
# [5/6] Creating OSS bucket (assets)...
# [6/6] Generating infracfg.json...
# ✓ Provisioning completed
```

### Step 4: Build Application

```bash
# Build Docker image
encore build docker ecommerce-demo:$(git rev-parse --short HEAD)
```

### Step 5: Deploy Application

```bash
# Deploy to production
infracast deploy --env production --verbose

# Or with specific flags
infracast deploy --env production \
  --skip-build \
  --tag ecommerce-demo:abc123
```

### Step 6: Run Database Migrations

```bash
# Apply database migrations
infracast migrate --env production

# Or run specific migration
infracast migrate --env production --to 002_add_orders
```

### Step 7: Verify Deployment

```bash
# Check deployment status
infracast status --env production

# View application logs
infracast logs --env production --follow

# Check endpoints
infracast open --env production
```

## Rollback Procedure

### Quick Rollback (Last Known Good)

```bash
# Rollback to previous version
infracast rollback --env production

# Verify rollback
infracast status --env production
```

### Manual Rollback (Specific Version)

```bash
# List available deployments
infracast history --env production

# Rollback to specific version
infracast rollback --env production --to v1.2.3
```

## Troubleshooting

### Issue: Provisioning Failed

**Symptoms**: RDS creation timeout

**Solution**:
```bash
# Check cloud provider status
infracast provider status --env production

# Retry provisioning
infracast provision --env production --retry
```

### Issue: Deployment Health Check Failed

**Symptoms**: Pods not starting, CrashLoopBackOff

**Solution**:
```bash
# Check pod status
kubectl get pods -n ecommerce-demo-production

# View pod logs
kubectl logs -n ecommerce-demo-production deployment/api

# Check resource limits
kubectl describe pod -n ecommerce-demo-production <pod-name>
```

### Issue: Database Connection Failed

**Symptoms**: Application logs show connection errors

**Solution**:
```bash
# Verify infracfg.json
infracast config validate --env production

# Check database connectivity
infracast debug db-connect --env production --db users-db
```

## Production Checklist

Before going live:

- [ ] Resource limits configured (CPU/Memory)
- [ ] Database backups enabled
- [ ] SSL/TLS certificates configured
- [ ] Monitoring and alerting set up
- [ ] Log aggregation configured
- [ ] Security groups/firewall rules reviewed
- [ ] Disaster recovery plan documented

## Scaling

### Horizontal Scaling

```bash
# Scale to 5 replicas
infracast scale --env production --replicas 5
```

### Vertical Scaling

```bash
# Update resource configuration
infracast config edit --env production

# Apply changes
infracast provision --env production --update
```

## Cleanup

### Destroy Environment

```bash
# Destroy all provisioned resources
infracast destroy --env production

# Confirm destruction
infracast destroy --env production --confirm
```

**Warning**: This will delete all data. Ensure backups exist.

## Monitoring

### View Metrics

```bash
# Open monitoring dashboard
infracast dashboard --env production

# View resource usage
infracast metrics --env production
```

### Set Up Alerts

```bash
# Configure alert rules
infracast alert create --env production \
  --name "high-cpu" \
  --condition "cpu > 80%" \
  --notify "ops@company.com"
```

## Support

For issues or questions:

- Documentation: https://github.com/DaviRain-Su/Infracast/docs
- Issues: https://github.com/DaviRain-Su/Infracast/issues
- Discussions: https://github.com/DaviRain-Su/Infracast/discussions
