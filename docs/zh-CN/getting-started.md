# Infracast 快速开始

本指南将带你使用 Infracast 部署你的第一个 Encore 应用。

## 前置条件

- [Go](https://golang.org/dl/) 1.22 或更高版本
- [Encore CLI](https://encore.dev/docs/install)
- [Infracast CLI](../../README.md#installation)
- 阿里云账号及访问凭证

## 凭证与安全基线（阿里云）

在 Shell 中设置凭证（切勿提交到 git）：

```bash
export ALICLOUD_ACCESS_KEY="your-access-key-id"
export ALICLOUD_SECRET_KEY="your-access-key-secret"
export ALICLOUD_REGION="cn-hangzhou"

# 可选：显式覆盖 RDS 白名单。
# 默认行为（推荐）：自动使用当前 VSwitch CIDR。
export ALICLOUD_RDS_SECURITY_IP_LIST="10.0.0.0/24"
```

单云流程推荐的最小 RAM 权限：
- `AliyunRDSFullAccess`
- `AliyunKvstoreFullAccess`
- `AliyunOSSFullAccess`
- `AliyunVPCFullAccess`
- `AliyunSTSAssumeRoleAccess`（仅在使用 STS 模式时需要）

生产环境请使用自定义最小权限策略替换宽泛的托管策略，范围限定在目标地域/资源。

## 5 步快速上手

### 步骤 1：初始化项目

创建一个新的 Infracast 项目：

```bash
infracast init my-app --provider alicloud --region cn-hangzhou
```

这将创建：
- `infracast.yaml` - 项目配置
- `.infra/` - 基础设施状态目录
- `.gitignore` - Git 忽略规则
- `README.md` - 项目文档

### 步骤 2：配置资源

编辑 `infracast.yaml` 来定义你的基础设施：

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

### 步骤 3：预配基础设施

创建云资源：

```bash
infracast provision --env dev
```

这将：
- 创建 VPC 和网络
- 预配 RDS PostgreSQL 实例
- 创建 Redis 缓存集群
- 生成包含连接信息的 `infracfg.json`

安全说明：
- 数据库和 Redis 密码采用加密安全的随机数生成。
- RDS 白名单默认为 VSwitch CIDR（私有网络），而非 `127.0.0.1` 或 `0.0.0.0/0`。

### 步骤 4：部署应用

构建并部署你的 Encore 应用：

```bash
infracast deploy --env dev
```

部署流程：
1. 构建 Docker 镜像（`encore build docker`）
2. 推送到容器镜像仓库
3. 部署到 Kubernetes
4. 验证健康检查

### 步骤 5：验证部署

检查部署状态：

```bash
# 查看状态
infracast status --env dev

# 查看日志
infracast logs --env dev

# 通过 port-forward 验证
kubectl port-forward svc/$(infracast status --env dev 2>/dev/null | grep -o '[^ ]*-dev') -n $(infracast status --env dev 2>/dev/null | grep -o '[^ ]*-dev') 8080:80 &
curl -s http://localhost:8080/livez | jq .
kill %1
```

## 下一步

### 添加更多环境

```bash
# 创建 staging 环境
infracast env create staging --provider alicloud --region cn-shanghai

# 部署到 staging
infracast deploy --env staging
```

### 配置通知

为部署添加 webhook 通知：

```yaml
# infracast.yaml
notifications:
  feishu:
    webhook_url: "https://open.feishu.cn/..."
```

### 了解更多

- [Technical Spec](../03-technical-spec.md)
- [Task Breakdown](../04-task-breakdown.md)
- [单云运维手册](06-single-cloud-operations.md)

## 快速命令流程

从零到部署的最短路径：

```bash
infracast init my-app --provider alicloud --region cn-hangzhou -y
cd my-app
# 编辑 infracast.yaml → 取消注释你需要的资源
infracast provision --env dev
infracast deploy --env dev
infracast status --env dev
```

## 常见错误与后续步骤

完整错误码参考请参见 [Error Code Matrix](error-code-matrix.md)。

| 错误 | 原因 | 修复方法 |
|------|------|----------|
| `ECFG001: failed to load config` | 缺少或无效的 `infracast.yaml` | 运行 `infracast init` 或检查 YAML 语法 |
| `ECFG019: environment not found` | 环境名称不存在 | 运行 `infracast env list` 查看可用环境 |
| `EDEPLOY001: invalid environment` | `--env` 标志拼写错误 | 有效值：`dev`、`staging`、`production`、`local` |
| `NotEnoughBalance` | 云账号余额不足以预配节点 | 充值账号或使用抢占式实例 |
| `KUBECONFIG` not set | 缺少 Kubernetes 配置 | `export KUBECONFIG=~/.kube/config` |
| Docker build fails | Docker 守护进程未运行 | 运行 `docker info` 验证，然后 `docker start` |
| Registry push fails | 镜像仓库凭证无效 | 重新认证：`docker login <registry-url>` |
| Deploy timeout | 网络或集群问题 | 检查连通性；使用 `--verbose` 重试以获取详情 |

## 使用 Trace ID 排查部署失败

每次部署/预配运行都会生成一个 `trace_id`，用于关联该流水线中的所有步骤。当部署失败时，使用 trace ID 查看完整时间线：

**步骤 1**：查找失败的部署追踪

```bash
infracast logs --level ERROR --since 1h
```

输出示例：
```
TIME              TRACE         LEVEL  ACTION  STEP        STATUS  ENV  DURATION  MESSAGE
2026-04-15 16:30  trc_17131...  ERROR  deploy  provision   fail    dev  5s        EPROV003: NotEnoughBalance...
```

**步骤 2**：查看该追踪的所有步骤

```bash
infracast logs --trace trc_17131...
```

输出显示完整的流水线：
```
TIME              TRACE         LEVEL  ACTION  STEP        STATUS  ENV  DURATION  MESSAGE
2026-04-15 16:30  trc_17131...  INFO   deploy  build       ok      dev  12s       Docker image built
2026-04-15 16:30  trc_17131...  INFO   deploy  push        ok      dev  8s        Image pushed to registry
2026-04-15 16:30  trc_17131...  ERROR  deploy  provision   fail    dev  5s        EPROV003: NotEnoughBalance...

  Error in [deploy/provision]:
    Code:       EPROV003
    Request ID: 7B3A4C2D-...
    Message:    InvalidAccountStatus.NotEnoughBalance
```

**步骤 3**：根据错误采取行动

错误码（`EPROV003`）和云服务提供商请求 ID 可帮助你：
- 查找失败的确切云 API 调用
- 与你的提供商控制台/支持进行交叉验证
- 查看上方的“常见错误”表格获取建议修复方法

## 示例应用

查看 [examples](../../examples/) 目录获取完整的示例应用：

- [hello-world](../../examples/hello-world/) - 最小示例
- [todo-app](../../examples/todo-app/) - 带数据库的 Todo 应用
- [blog-api](../../examples/blog-api/) - 支持 OSS 上传的博客 API
