# 单云运维手册（阿里云）

在阿里云上运维 Infracast 的可操作步骤。
架构与配置详情，请参阅[单云运维手册](06-single-cloud-operations.md)。
错误码参考，请参阅[错误码矩阵](error-code-matrix.md)。

---

## 1. 告警设置

### 1.1 阿里云预算告警

在 **阿里云控制台 → 费用中心 → 预算管理** 中设置：

| 告警项 | 阈值 | 动作 |
|-------|------|------|
| 日消费 | ¥50 | 邮件 + 短信通知 |
| 月消费 | ¥500 | 邮件 + 短信通知 |
| 余额不足 | 剩余 ¥100 | 邮件 + 短信 + 暂停非关键部署 |

### 1.2 资源数量告警

通过阿里云云监控或手动检查进行监控：

```bash
# 快速资源数量检查
infracast status --env dev
```

告警条件：
- 前缀为 `infracast-*` 的 RDS 实例 > 3 → 可能存在测试资源泄漏
- 前缀为 `infracast-*` 的 Redis 实例 > 3 → 同上
- VPC 数量 > 5 → 配额承压；执行清理

### 1.3 部署失败告警

Infracast 将所有操作记录到审计数据库中。按以下方式监控失败：

```bash
# 检查最近 1 小时内的错误
infracast logs --level ERROR --since 1h

# 检查最近 24 小时内的错误（JSON 格式，便于自动化）
infracast logs --level ERROR --since 24h --format json
```

如果配置了飞书/钉钉通知，当 `infracast.yaml` 中设置了 webhook 时，部署失败会自动发送通知：

```yaml
notifications:
  feishu:
    webhook_url: "https://open.feishu.cn/..."
```

---

## 2. 回滚流程

### 2.1 自动回滚（部署流水线）

部署期间健康检查失败时，Infracast 会自动触发回滚：

1. 部署检测到 Pod 不健康（EDEPLOY050）
2. 检查是否可安全回滚（存在上一个版本、无破坏性迁移）
3. 执行 `kubectl rollout undo`
4. 验证回滚是否稳定

检查回滚状态：

```bash
# 查看部署链路追踪，确认是否触发了回滚
infracast logs --trace <trace_id>

# 通过 kubectl 手动检查
kubectl rollout status deployment/<app-name> -n <namespace>
kubectl rollout history deployment/<app-name> -n <namespace>
```

### 2.2 手动回滚

当自动回滚失败或未触发时：

```bash
# 步骤 1：检查当前状态
kubectl get pods -n <namespace>
kubectl describe deployment <app-name> -n <namespace>

# 步骤 2：回滚到上一个版本
kubectl rollout undo deployment/<app-name> -n <namespace>

# 步骤 3：验证
kubectl rollout status deployment/<app-name> -n <namespace>

# 步骤 4：检查健康端点
curl -s http://<service-ip>/livez | jq .
curl -s http://<service-ip>/readyz | jq .
```

### 2.3 回滚被阻止：破坏性迁移（EDEPLOY068）

如果部署包含不可逆的数据库迁移，回滚将被阻止。此时：

1. **请勿**运行 `kubectl rollout undo` — 数据库 schema 将与旧代码不匹配
2. 修复应用代码并向前重新部署
3. 如果数据库已损坏，从备份恢复（见 §4.3）

---

## 3. 清理流程

### 3.1 标准清理（按环境）

```bash
# 始终先空跑
infracast destroy --env dev --dry-run --keep-vpc 1

# 检查输出后执行
infracast destroy --env dev --apply --keep-vpc 1
```

### 3.2 批量清理（所有测试资源）

当多个测试环境存在资源泄漏时：

```bash
# 使用前缀过滤器空跑
go run ./cmd/cleanup --region cn-hangzhou --prefix infracast

# 确认无误后执行
go run ./cmd/cleanup --region cn-hangzhou --prefix infracast --apply
```

### 3.3 清理期间处理依赖错误

常见错误：`DependencyViolation.VSwitch`、`DependencyViolation.Kvstore`、`DependencyViolation.NetworkInterface`

这些错误是**预期内的** — 云资源是异步释放的。处理步骤：

1. 确认 RDS/Redis 删除请求已被接受（检查阿里云控制台）
2. 等待 3–10 分钟进行异步释放
3. 重新运行 destroy/cleanup 命令
4. 如果 15 分钟后仍然失败，在阿里云控制台检查卡住的资源并手动删除

### 3.4 每周例行清理

每周一运行（或任何测试迭代结束后）：

```bash
# 1. 检查泄漏资源
go run ./cmd/cleanup --region cn-hangzhou --prefix infracast

# 2. 检查并在需要时执行
go run ./cmd/cleanup --region cn-hangzhou --prefix infracast --apply

# 3. 验证配额余量
# 在阿里云控制台检查 VPC 数量（默认限制：每个地域 10 个）

# 4. 检查审计日志中是否存在异常
infracast logs --since 7d --level WARN
infracast logs --since 7d --level ERROR
```

---

## 4. 事件响应：常见故障

### 4.1 余额不足（NotEnoughBalance / InvalidAccountStatus.NotEnoughBalance）

**现象**：预配或部署失败，报错 EPROV003，或 ACK 节点池扩容失败。

**诊断**：
```bash
infracast logs --level ERROR --since 1h
# 查找：EPROV003、NotEnoughBalance
```

**修复**：
1. 检查阿里云账单：**控制台 → 费用中心 → 账户总览**
2. 按量付费：需同时检查现金余额和信用额度
3. ACK 节点池即使对于小规格实例也可能需要 ¥100 元以上余额
4. 充值账户后重试：
   ```bash
   infracast provision --env dev
   # 或
   infracast deploy --env dev
   ```

### 4.2 服务关联角色不存在（ServiceLinkedRole.NotExist）

**现象**：首次为 RDS 或 OSS 预配时失败。

**诊断**：
```bash
infracast logs --level ERROR --since 1h
# 查找：ServiceLinkedRole
```

**修复**：
1. 打开 **阿里云 RDS 控制台** → 点击“创建实例”（无需实际购买）
2. 这会触发自动创建 ServiceLinkedRole
3. 对于 OSS：打开 **OSS 控制台** → 如提示则点击“开通”
4. 1 分钟后重试预配

### 4.3 部署超时（EDEPLOY050）

**现象**：部署后健康检查失败，Pod 未就绪。

**诊断**：
```bash
# 获取链路追踪
infracast logs --level ERROR --since 1h

# 检查 Pod 状态
kubectl get pods -n <namespace>
kubectl describe pod <pod-name> -n <namespace>

# 检查应用日志
kubectl logs <pod-name> -n <namespace>
```

**常见原因与修复**：

| Pod 状态 | 可能原因 | 修复 |
|----------|---------|------|
| Pending | 无可用节点（余额不足） | 充值账户，检查节点池 |
| CrashLoopBackOff | 应用启动时崩溃 | 检查日志，修复应用代码 |
| ImagePullBackOff | ACR 认证失败或镜像未找到 | `docker login <acr-url>`，确认镜像已推送 |
| Running 但未 Ready | 健康端点异常 | 检查 `/livez` 和 `/readyz` 的实现 |

### 4.4 ACR 推送失败（EDEPLOY040–047）

**现象**：推送镜像到阿里云容器镜像仓库失败。

**诊断**：
```bash
infracast logs --level ERROR --since 1h
# 查找：EDEPLOY040-047
```

**修复**：
```bash
# 重新认证
docker login --username=<access-key-id> <acr-registry-url>

# 验证本地镜像是否存在
docker images | grep <app-name>

# 手动推送测试
docker push <acr-registry-url>/<namespace>/<image>:<tag>
```

### 4.5 KUBECONFIG / 集群连通性（EDEPLOY011）

**现象**：K8s 操作因客户端初始化错误而失败。

**修复**：
```bash
# 验证 KUBECONFIG
echo $KUBECONFIG
kubectl cluster-info

# 如果使用 ACK，重新获取 kubeconfig
aliyun cs GET /k8s/<cluster-id>/user_config | jq -r '.config' > ~/.kube/config
export KUBECONFIG=~/.kube/config

# 测试连通性
kubectl get nodes
```

### 4.6 RDS/Redis 处于过渡状态（IncorrectDBInstanceState）

**现象**：销毁或修改操作因实例处于过渡状态而失败。

**修复**：
1. 在阿里云控制台检查实例状态
2. 等待当前操作完成（通常 5–15 分钟）
3. 重试操作
4. 如果卡住超过 30 分钟，携带 RequestID 提交阿里云工单

---

## 5. 升级路径

当自助修复无法解决问题时：

1. **收集证据**：
   ```bash
   infracast logs --trace <trace_id> --format json > /tmp/trace-dump.json
   ```

2. **检查阿里云侧**：
   - 从错误输出中复制 `Request ID`
   - 在 **阿里云控制台 → 操作审计（ActionTrail）** 中搜索该请求

3. **提交工单时附上**：
   - Infracast 链路追踪转储（JSON）
   - 阿里云 Request ID
   - 地域和资源 ID
   - 复现步骤

---

## 6. 快速参考

| 场景 | 命令 |
|------|------|
| 最近错误 | `infracast logs --level ERROR --since 1h` |
| 追踪一次部署 | `infracast logs --trace trc_xxx` |
| 完整链路追踪（JSON） | `infracast logs --trace trc_xxx --format json` |
| 宽输出 | `infracast logs --output wide` |
| Pod 状态 | `kubectl get pods -n <namespace>` |
| Pod 日志 | `kubectl logs <pod> -n <namespace>` |
| 手动回滚 | `kubectl rollout undo deployment/<app> -n <ns>` |
| 销毁（空跑） | `infracast destroy --env dev --dry-run --keep-vpc 1` |
| 销毁（执行） | `infracast destroy --env dev --apply --keep-vpc 1` |
| 批量清理 | `go run ./cmd/cleanup --region cn-hangzhou --prefix infracast --apply` |
| 检查健康状态 | `curl -s http://<ip>/livez \| jq .` |
