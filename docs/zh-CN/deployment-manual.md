# 单云部署手册（阿里云）

使用 Infracast 在阿里云上部署 Encore 应用的完整命令流参考。
目标读者：新开发者，30 分钟内从零到部署。

另请参见：[Error Code Matrix](error-code-matrix.md) 获取所有错误码及其来源文件和修复方法。

---

## 1. 环境前置条件

### 1.1 本地工具

| 工具 | 版本 | 检查 | 安装 |
|------|---------|-------|---------|
| Go | ≥ 1.22 | `go version` | https://golang.org/dl/ |
| Encore CLI | latest | `encore version` | https://encore.dev/docs/install |
| Docker | running | `docker info` | https://docs.docker.com/get-docker/ |
| Infracast CLI | latest | `infracast version` | `go install github.com/DaviRain-Su/infracast/cmd/infracast@latest` |
| kubectl | ≥ 1.28 | `kubectl version --client` | https://kubernetes.io/docs/tasks/tools/ |

### 1.2 阿里云账号

- 已完成实名认证
- 已开通 RDS 和 OSS 服务（分别打开一次对应控制台以触发服务关联角色创建）
- 按量付费资源余额充足（RDS pg.n2.medium.1 + Redis ≈ ¥20/天）
- 目标地域已创建并运行 ACK（容器服务）集群

### 1.3 凭证

```bash
# 必需 — 切勿提交到 git
export ALICLOUD_ACCESS_KEY="your-access-key-id"
export ALICLOUD_SECRET_KEY="your-access-key-secret"
export ALICLOUD_REGION="cn-hangzhou"

# Kubernetes — 指向你的 ACK 集群
export KUBECONFIG="$HOME/.kube/config"

# 可选 — 覆盖 RDS 白名单（默认：VSwitch CIDR）
# export ALICLOUD_RDS_SECURITY_IP_LIST="10.0.0.0/24"
```

### 1.4 RAM 权限（最小权限）

- `AliyunRDSFullAccess`
- `AliyunKvstoreFullAccess`
- `AliyunOSSFullAccess`
- `AliyunVPCFullAccess`
- `AliyunCSFullAccess`（ACK 操作）
- `AliyunCRFullAccess`（容器镜像仓库推送）

生产环境请使用自定义最小权限策略替换托管策略，范围限定在目标地域/资源。

---

## 2. 命令流程：init → env → provision → deploy → logs → destroy

### 步骤 1：初始化项目

```bash
infracast init my-app --provider alicloud --region cn-hangzhou -y
cd my-app
```

**预期输出（成功）：**
```
✓ Created infracast.yaml
✓ Created .infra/ directory
✓ Created .gitignore
✓ Project my-app initialized for alicloud (cn-hangzhou)
```

**预期输出（失败——目录已存在）：**
```
ECFG001: failed to load config: directory my-app already exists
```

### 步骤 2：创建环境

```bash
infracast env create dev --provider alicloud --region cn-hangzhou
infracast env list
```

**预期输出（成功）：**
```
Environment dev created.

ENVIRONMENT  PROVIDER  REGION       CURRENT
dev          alicloud  cn-hangzhou  →
```

### 步骤 3：预配基础设施

```bash
infracast provision --env dev
```

创建 VPC、VSwitch、RDS PostgreSQL、Redis，并生成 `infracfg.json`。

**预期输出（成功）：**
```
✓ VPC created (vpc-bp1xxx)
✓ VSwitch created (vsw-bp1xxx)
✓ RDS PostgreSQL created (pgm-bp1xxx)
✓ Redis created (r-bp1xxx)
✓ Generated infracfg.json

Provision complete. Resources ready in env dev.
```

**预期输出（失败——余额不足）：**
```
EPROV003: InvalidAccountStatus.NotEnoughBalance
  Hint: Top up your Alicloud account or use smaller instance specs.
```

### 步骤 4：部署应用

```bash
infracast deploy --env dev
```

构建 Docker 镜像，推送到 ACR，部署到 ACK，并验证健康检查。

**预期输出（成功）：**
```
Deploy to dev
  Trace: trc_1713184200000000000

  Step           Status  Duration
  ─────────────  ──────  ────────
  build          OK      12s
  push           OK      8s
  deploy         OK      15s
  health-check   OK      3s

  ✓ Deploy succeeded (4/4 steps passed, 38s total)
```

**预期输出（失败——健康检查超时）：**
```
  Step           Status  Duration
  ─────────────  ──────  ────────
  build          OK      12s
  push           OK      8s
  deploy         OK      15s
  health-check   FAIL    30s

  Hint: Application did not pass health check within timeout.
        Check application logs: infracast logs --trace trc_xxx
  Error: EDEPLOY050: deployment timeout after 30s

  ✗ Deploy failed (3/4 steps passed, 65s total)
```

### 步骤 5：验证与观察

```bash
# 查看部署状态
infracast status --env dev

# 查看最近审计日志
infracast logs --limit 10

# 追踪特定部署运行
infracast logs --trace trc_1713184200000000000

# 查看最近 1 小时的错误
infracast logs --level ERROR --since 1h
```

### 步骤 6：销毁（清理）

```bash
# 先空跑 — 务必执行
infracast destroy --env dev --dry-run --keep-vpc 1

# 确认无误后执行
infracast destroy --env dev --apply --keep-vpc 1
```

**预期输出（空跑）：**
```
[DRY-RUN] Would delete:
  - RDS: pgm-bp1xxx
  - Redis: r-bp1xxx
  - VSwitch: vsw-bp1xxx (skipped, --keep-vpc)
  - VPC: vpc-bp1xxx (skipped, --keep-vpc)

No resources were deleted. Run with --apply to execute.
```

---

## 3. 故障决策树

当部署或预配失败时，请遵循以下路径：

```
Deploy/Provision failed
│
├─ 从输出横幅获取 trace ID
│   └─ infracast logs --trace trc_xxx
│
├─ 识别失败的步骤（build / push / provision / deploy / health-check）
│
├─ 检查错误码：
│   │
│   ├─ ECFG001 — 缺少 provider / ECFG019 — 环境不存在
│   │   └─ 修复：检查 infracast.yaml 语法，运行 `infracast env list`
│   │
│   ├─ EDEPLOY001 — 无效环境
│   │   └─ 修复：验证 --env 标志与 `infracast env list` 一致
│   │
│   ├─ EDEPLOY050 — 部署/健康检查超时
│   │   └─ 修复：检查应用日志（`kubectl logs`），验证 /health 端点在本地可访问
│   │
│   ├─ EPROV003 / NotEnoughBalance — 计费
│   │   └─ 修复：充值账号，或使用更小规格
│   │
│   ├─ Docker build 失败
│   │   └─ 修复：`docker info` → 验证守护进程运行中，检查 Dockerfile
│   │
│   ├─ Registry push 失败 / 未授权
│   │   └─ 修复：`docker login <registry-url>`，检查 ACR 凭证
│   │
│   ├─ KUBECONFIG 未设置
│   │   └─ 修复：`export KUBECONFIG=~/.kube/config`，验证 ACK 集群
│   │
│   ├─ 超时（通用）
│   │   └─ 修复：检查网络，使用 `--verbose` 重试
│   │
│   └─ 未知错误
│       └─ 修复：从日志复制 request_id，检查阿里云控制台
│
└─ 修复后，重新运行相同命令
```

### 使用 Trace ID 进行深度诊断

```bash
# 1. 查找最近的错误
infracast logs --level ERROR --since 1h

# 2. 从输出中获取 trace ID，然后查看完整流水线
infracast logs --trace trc_17131...

# 3. 输出显示所有步骤及状态：
# TIME              TRACE         LEVEL  ACTION  STEP        STATUS  ENV  DURATION  MESSAGE
# 2026-04-15 16:30  trc_17131...  INFO   deploy  build       ok      dev  12s       Docker image built
# 2026-04-15 16:30  trc_17131...  INFO   deploy  push        ok      dev  8s        Image pushed
# 2026-04-15 16:30  trc_17131...  ERROR  deploy  provision   fail    dev  5s        EPROV003: NotEnoughBalance
#
#   Error in [deploy/provision]:
#     Code:       EPROV003
#     Request ID: 7B3A4C2D-...
#     Message:    InvalidAccountStatus.NotEnoughBalance

# 4. 使用错误码 + request ID 在阿里云控制台中查询
```

---

## 4. 回归测试命令集（单云）

一套可直接复制粘贴的完整流水线验证命令序列。代码变更后运行，以确认没有破坏任何功能。

### 4.1 成功路径

```bash
# 初始化
infracast init regression-test --provider alicloud --region cn-hangzhou -y
cd regression-test

# 复制健康检查示例
cp -r /path/to/infracast/examples/health-check/* .

# 创建环境
infracast env create dev --provider alicloud --region cn-hangzhou

# 预配（创建云资源 — 会产生费用）
infracast provision --env dev

# 部署
infracast deploy --env dev

# 验证健康（port-forward 到服务）
kubectl port-forward svc/demo-app -n demo-app-dev 8080:80 &
curl -s http://localhost:8080/livez | jq .
# 预期：{"status":"ok","uptime":"..."}

curl -s http://localhost:8080/readyz | jq .
# 预期：{"status":"ready","checks":{"self":"ok"},...}
kill %1

# 查看审计追踪
infracast logs --limit 5
# 预期：最近的部署步骤，status=ok

# 清理
infracast destroy --env dev --apply --keep-vpc 1
```

### 4.2 失败路径（模拟）

```bash
# 模拟失败进行部署
SIMULATE_FAILURE=true infracast deploy --env dev

# 验证就绪探针报告异常
kubectl port-forward svc/demo-app -n demo-app-dev 8080:80 &
curl -s http://localhost:8080/readyz | jq .
# 预期：{"status":"unhealthy","checks":{"self":"fail"},...}
kill %1

# 检查错误审计追踪
infracast logs --level ERROR --since 10m
# 预期：health-check 步骤，status=fail
```

### 4.3 冒烟测试（无需云资源）

用于不花钱的本地验证：

```bash
# 仅构建 — 验证 Go 编译和 Docker 镜像
infracast deploy --env dev --dry-run

# 验证 CLI 工具
infracast version
infracast env list
infracast logs --limit 1
```

---

## 5. 下载与验证发布版本

### 5.1 下载

从 [Releases](https://github.com/DaviRain-Su/Infracast/releases) 下载适用于你平台的预编译二进制文件：

```bash
# 示例：Apple Silicon（darwin/arm64）
VERSION="v0.1.0"
curl -LO "https://github.com/DaviRain-Su/Infracast/releases/download/${VERSION}/infracast_${VERSION}_darwin_arm64.tar.gz"
curl -LO "https://github.com/DaviRain-Su/Infracast/releases/download/${VERSION}/checksums.txt"
```

可用平台：
- `infracast_<version>_darwin_amd64.tar.gz` — macOS Intel
- `infracast_<version>_darwin_arm64.tar.gz` — macOS Apple Silicon
- `infracast_<version>_linux_amd64.tar.gz` — Linux x86_64

### 5.2 验证校验和

```bash
shasum -a 256 -c checksums.txt --ignore-missing
# 预期：infracast_v0.1.0_darwin_arm64.tar.gz: OK
```

### 5.3 安装

```bash
tar xzf "infracast_${VERSION}_darwin_arm64.tar.gz"
sudo mv "infracast_${VERSION}_darwin_arm64/infracast" /usr/local/bin/
infracast version
# 预期：
# infracast version v0.1.0
#   commit: abc1234
#   build time: 2026-04-15_12:00:00
```

### 5.4 从源码构建

```bash
git clone https://github.com/DaviRain-Su/Infracast.git
cd Infracast
make build
./bin/infracast version
```

---

## 6. 快速参考

| 任务 | 命令 |
|------|---------|
| 初始化 | `infracast init my-app --provider alicloud --region cn-hangzhou -y` |
| 创建环境 | `infracast env create dev --provider alicloud --region cn-hangzhou` |
| 切换环境 | `infracast env use staging` |
| 列出环境 | `infracast env list` |
| 预配 | `infracast provision --env dev` |
| 部署 | `infracast deploy --env dev` |
| 状态 | `infracast status --env dev` |
| 日志（最近） | `infracast logs --limit 20` |
| 日志（错误） | `infracast logs --level ERROR --since 1h` |
| 日志（追踪） | `infracast logs --trace trc_xxx` |
| 销毁（空跑） | `infracast destroy --env dev --dry-run --keep-vpc 1` |
| 销毁（执行） | `infracast destroy --env dev --apply --keep-vpc 1` |
