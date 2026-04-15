# 成本与环境前置条件检查清单（阿里云）

在运行 `infracast provision` 或 `infracast deploy` 之前的预检清单。
首次部署前请逐项验证，账号发生变更后请重新检查。

---

## 1. 账号前置条件

| 序号 | 检查项 | 验证方法 | 失败时的修复 |
|---|-------|---------------|---------------|
| 1 | 已完成实名认证 | 阿里云控制台 → 账号 → 认证状态 | 完成认证流程 |
| 2 | 账号余额 ≥ ¥100 | 控制台 → 费用 → 账户总览 | 通过控制台 → 费用 → 充值进行充值 |
| 3 | 按量付费信用额度充足 | 控制台 → 费用 → 信用额度 | 申请提升额度或预付费 |
| 4 | 已开通 RDS 服务 | 控制台 → RDS → 检查是否有“开通”提示 | 点击“开通”或进入创建实例向导 |
| 5 | 已开通 Redis（Kvstore）服务 | 控制台 → Redis → 检查是否有“开通”提示 | 点击“开通” |
| 6 | 已开通 OSS 服务 | 控制台 → OSS → 检查是否有“开通”提示 | 点击“开通” |
| 7 | 目标地域有运行中的 ACK 集群 | 控制台 → 容器服务 → 集群 | 创建 ACK 集群（托管版，≥1 节点） |
| 8 | 可访问 ACR（容器镜像仓库） | `docker login <acr-url>` 成功 | 在控制台开通 ACR 并创建命名空间 |

---

## 2. 凭证前置条件

| 序号 | 检查项 | 验证方法 | 失败时的修复 |
|---|-------|---------------|---------------|
| 1 | `ALICLOUD_ACCESS_KEY` 已设置 | `echo $ALICLOUD_ACCESS_KEY` | `export ALICLOUD_ACCESS_KEY="your-key"` |
| 2 | `ALICLOUD_SECRET_KEY` 已设置 | `echo $ALICLOUD_SECRET_KEY` | `export ALICLOUD_SECRET_KEY="your-secret"` |
| 3 | `ALICLOUD_REGION` 已设置 | `echo $ALICLOUD_REGION` | `export ALICLOUD_REGION="cn-hangzhou"` |
| 4 | `KUBECONFIG` 已设置且有效 | `kubectl cluster-info` | `export KUBECONFIG=~/.kube/config` |
| 5 | Docker 守护进程正在运行 | `docker info` | 启动 Docker Desktop 或 `systemctl start docker` |

### RAM 权限（最小权限）

AccessKey 必须附加以下托管策略：

| 策略 | 用途 |
|--------|-------------|
| `AliyunRDSFullAccess` | RDS PostgreSQL 预配 |
| `AliyunKvstoreFullAccess` | Redis 预配 |
| `AliyunOSSFullAccess` | OSS 存储桶创建 |
| `AliyunVPCFullAccess` | VPC/VSwitch 网络 |
| `AliyunCSFullAccess` | ACK 集群操作 |
| `AliyunCRFullAccess` | ACR 镜像推送 |

生产环境请使用自定义最小权限策略替换托管策略，范围限定在目标地域/资源。

---

## 3. 本地工具前置条件

| 工具 | 版本 | 检查 | 安装 |
|------|---------|-------|---------|
| Go | ≥ 1.22 | `go version` | https://golang.org/dl/ |
| Encore CLI | latest | `encore version` | https://encore.dev/docs/install |
| Docker | running | `docker info` | https://docs.docker.com/get-docker/ |
| Infracast CLI | latest | `infracast version` | `go install github.com/DaviRain-Su/infracast/cmd/infracast@latest` |
| kubectl | ≥ 1.28 | `kubectl version --client` | https://kubernetes.io/docs/tasks/tools/ |

---

## 4. 配额与资源限制

Infracast 相关的阿里云默认配额：

| 资源 | 默认配额 | 需求 | 检查路径 |
|----------|---------------|----------|-------|
| 每地域 VPC | 10 | ≥ 1 可用 | 控制台 → VPC → VPC 列表 |
| 每 VPC 的 VSwitch | 24 | ≥ 1 可用 | 控制台 → VPC → VSwitch |
| 每地域 RDS 实例 | 30 | ≥ 1 可用 | 控制台 → RDS → 实例列表 |
| 每地域 Redis 实例 | 20 | ≥ 1 可用 | 控制台 → Redis → 实例列表 |
| 每账号 OSS 存储桶 | 100 | ≥ 1 可用 | 控制台 → OSS → 存储桶列表 |

如果配额已耗尽：
1. 先清理未使用的测试资源：`go run ./cmd/cleanup --region cn-hangzhou --prefix infracast --apply`
2. 申请提升配额：控制台 → 配额中心

---

## 5. 成本估算

单个 `dev` 环境的大致费用（杭州地域，按量付费）：

| 资源 | 规格 | 大致费用 |
|----------|------|-----------------|
| RDS PostgreSQL | pg.n2.medium.1, 20GB | ~¥8/天 |
| Redis | redis.master.small.default | ~¥5/天 |
| OSS | 标准存储，极小用量 | < ¥1/天 |
| ACK 托管集群 | 控制面免费，工作节点费用另计 | ¥0（控制面）+ 工作节点费用 |
| ACK 工作节点 | ecs.u1-c1m2.xlarge（4vCPU/8GiB） | ~¥5–10/天 |
| **dev 总计** | | **~¥20–25/天** |

降低成本建议：
- 测试后运行 `infracast destroy --env dev --apply --keep-vpc 1` 进行清理
- 使用 `--keep-vpc 1` 避免重复创建网络（节省时间并避免配额压力）
- 测试时使用更小规格（如 `pg.n2.small.1`）
- 设置每日 ¥50 的预算告警

---

## 6. 预检脚本

首次部署前运行此脚本以验证所有前置条件：

```bash
#!/bin/bash
# Infracast pre-flight check

echo "=== Account & Credentials ==="
echo -n "ALICLOUD_ACCESS_KEY: "
[ -n "$ALICLOUD_ACCESS_KEY" ] && echo "SET ✓" || echo "MISSING ✗"
echo -n "ALICLOUD_SECRET_KEY: "
[ -n "$ALICLOUD_SECRET_KEY" ] && echo "SET ✓" || echo "MISSING ✗"
echo -n "ALICLOUD_REGION:     "
[ -n "$ALICLOUD_REGION" ] && echo "$ALICLOUD_REGION ✓" || echo "MISSING ✗"

echo ""
echo "=== Local Tools ==="
echo -n "Go:        "; go version 2>/dev/null || echo "NOT FOUND ✗"
echo -n "Encore:    "; encore version 2>/dev/null || echo "NOT FOUND ✗"
echo -n "Docker:    "; docker info >/dev/null 2>&1 && echo "RUNNING ✓" || echo "NOT RUNNING ✗"
echo -n "kubectl:   "; kubectl version --client --short 2>/dev/null || echo "NOT FOUND ✗"
echo -n "Infracast: "; infracast version 2>/dev/null || echo "NOT FOUND ✗"

echo ""
echo "=== Kubernetes ==="
echo -n "KUBECONFIG: "
[ -n "$KUBECONFIG" ] && echo "$KUBECONFIG ✓" || echo "MISSING ✗"
echo -n "Cluster:   "; kubectl cluster-info 2>/dev/null | head -1 || echo "NOT REACHABLE ✗"

echo ""
echo "=== Pre-flight complete ==="
```

---

## 7. 故障排查路径

当前置条件检查失败时，请遵循以下决策树：

```
Pre-flight check failed
│
├─ Credential missing → 设置环境变量（§2）
│
├─ Tool not found → 通过 §3 中的 URL 安装
│
├─ Docker not running → 启动 Docker 守护进程
│
├─ kubectl cannot reach cluster
│   ├─ KUBECONFIG not set → export KUBECONFIG=~/.kube/config
│   └─ Cluster unreachable → 在控制台检查 ACK 集群状态
│
├─ Alicloud API error
│   ├─ InvalidAccessKeyId → 在 RAM 控制台验证密钥
│   ├─ Forbidden / NoPermission → 附加所需策略（§2）
│   └─ ServiceLinkedRole.NotExist → 打开 RDS/OSS 控制台触发创建
│
└─ Quota exceeded → 清理资源（§4）或申请提升配额
```
