# 单云运维手册（阿里云）

本手册是阿里云单云交付的运维基线，涵盖部署、清理、故障排查和成本控制。

## 1. 部署手册

### 1.1 前置条件

- 账户：
  - 已完成实名认证。
  - 已开通 RDS 和 OSS 服务。
  - 按量付费资源余额充足。
- 凭证：
  - `ALICLOUD_ACCESS_KEY`
  - `ALICLOUD_SECRET_KEY`
  - 可选：`ALICLOUD_REGION`（默认 `cn-hangzhou`）
- RAM 权限（最小权限基线）：
  - `AliyunRDSFullAccess`
  - `AliyunKvstoreFullAccess`
  - `AliyunOSSFullAccess`
  - `AliyunVPCFullAccess`

### 1.2 推荐的 shell 配置

```bash
set -a
source .env
set +a
```

### 1.3 仅预配验证（核心基础设施）

当前状态：专用的 `provision-verify` 辅助工具已规划，但尚未实现为稳定命令。

目前请使用以下验证路径：

```bash
infracast provision --env dev --region cn-hangzhou --dry-run
```

预期通过信号：
- 配置/提供商检查无验证错误。

### 1.4 完整部署链路（当 Encore/ACK 就绪时）

当前状态：完整的端到端命令已规划，并取决于完整的流水线前置条件。

计划命令：

```bash
E2E_FULL=1 go test ./e2e/ -run TestFullE2EDeployment -v
```

说明：
- 这将创建真实的云资源并产生费用。
- 如果缺少 Encore CLI 或 ACK 前置条件，请先运行仅预配流程。

## 2. 清理手册

### 2.1 主路径：正式的销毁命令

先空跑：

```bash
infracast destroy --env dev --region cn-hangzhou --dry-run --keep-vpc 1
infracast destroy --env dev --region cn-hangzhou --prefix infracast-dev --dry-run --keep-vpc 1
```

执行：

```bash
infracast destroy --env dev --region cn-hangzhou --apply --keep-vpc 1
```

安全说明：
- 未指定 `--apply` 时，命令保持空跑模式。
- 宽泛前缀（例如 `infracast`）需要 `--force` 才能实际删除。

### 2.2 批量回退清理（旧版辅助工具）

仅在需要按前缀进行大范围清理时使用：

```bash
go run ./cmd/cleanup --region cn-hangzhou --prefix infracast
go run ./cmd/cleanup --region cn-hangzhou --prefix infracast --apply
```

### 2.3 预期的异步行为

删除 RDS/Redis 后，VSwitch/VPC 经常因依赖错误而临时失败。
这是正常的云端异步释放过程。等待 3-10 分钟后重新运行清理/销毁。

## 3. 故障排查运维手册

### 3.1 账户与服务就绪性

- `ServiceLinkedRole.NotExist`
  - 通过打开 RDS 控制台并完成创建实例流程来触发创建（无需实际购买）。
- `CommodityServiceCalling.Exception`
  - 通常是账户侧问题：认证不完整、欠费状态或服务未开通。
- `UserDisable`（OSS）
  - 开通 OSS 服务并验证 OSS 的 RAM 权限。

### 3.2 网络与配额

- `QuotaExceeded.Vpc`
  - 清理旧的测试 VPC/VSwitch 资源，或申请提升配额。
- `IncorrectVpcStatus`
  - 由在 VPC 可用之前创建依赖资源引起。
  - 确保在创建 VSwitch 之前激活 VPC 等待/轮询逻辑。
- `InvalidvSwitchId ... zone not supported`
  - Redis/RDS 可用区不匹配；使用可用区回退或支持的可用区探测。

### 3.3 删除依赖错误

- `DependencyViolation.Kvstore`
- `DependencyViolation.NetworkInterface`
- `DependencyViolation.VSwitch`

处理措施：
1. 确认 RDS/Redis 删除请求已被接受。
2. 等待异步释放窗口。
3. 重新运行 destroy/cleanup。

### 3.4 RDS 临时删除状态

- `IncorrectDBInstanceState`
  - 实例处于过渡状态（创建中/修改中/删除中）。
  - 稍后重试；视为临时状态，而非致命配置问题。

## 4. 成本与告警建议

### 4.1 成本控制

- 保留 `--keep-vpc 1` 以减少重复的网络创建。
- 测试后立即运行清理。
- 在修复冲刺期间优先使用仅预配验证；仅在需要时运行完整端到端测试。
- 为所有测试资源添加统一前缀标签（`infracast-*`）以便快速清理。

### 4.2 建议的运维告警

- 预算告警：
  - 测试账户支出的月度和每日阈值。
- 资源数量告警：
  - 前缀为 `infracast-*` 的 RDS/Redis/VPC 资源数量。
- 长期存活资源告警：
  - 任何存活时间超过 24 小时的测试资源。
- 清理失败告警：
  - 重试窗口外仍持续出现的依赖错误。

### 4.3 每周维护检查清单

1. 对每个活跃环境执行空跑销毁。
2. 移除过期的测试资源。
3. 验证配额余量（VPC、RDS、Redis）。
4. 检查 RAM 访问密钥并在需要时轮换。
