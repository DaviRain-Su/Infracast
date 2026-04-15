# Infracast 演示脚本（10–15 分钟）

单云（阿里云）端到端演示：初始化 → 预配 → 部署 → 验证 → 清理。

**前置条件**：运行本演示前，请先完成[前置条件检查清单](prerequisites-checklist.md)。

---

## 概述

| 步骤 | 耗时 | 需要云资源 | 命令 |
|------|------|------------|------|
| 0. 预检 | 1 分钟 | 否 | 凭证与工具检查 |
| 1. 初始化 | 30 秒 | 否 | `infracast init` |
| 2. 构建（空跑） | 1 分钟 | 否 | `infracast deploy --env dev --dry-run` |
| 3. 预配 | 3–5 分钟 | **是** | `infracast provision --env dev` |
| 4. 部署 | 2–3 分钟 | **是** | `infracast deploy --env dev` |
| 5. 验证 | 1 分钟 | **是** | 通过端口转发进行健康检查 |
| 6. 审计日志 | 30 秒 | 否 | `infracast logs` |
| 7. 通知 | 30 秒 | 可选 | 飞书/钉钉 Webhook |
| 8. 清理 | 1–2 分钟 | **是** | `infracast destroy` |

标记为 **Yes** 的步骤会创建真实的阿里云资源并产生费用（完整演示一次约 ¥2–5）。
步骤 0–2 和 6 可在无云环境的情况下作为空跑演示运行。

---

## 步骤 0：预检（1 分钟）

```bash
# 验证凭证
echo "Access Key: ${ALICLOUD_ACCESS_KEY:0:8}..."
echo "Region: $ALICLOUD_REGION"

# 验证工具
infracast version
encore version
docker info | head -3
kubectl cluster-info | head -1
```

**预期输出**：
```
Access Key: LTAI5t5o...
Region: cn-hangzhou
infracast version v0.1.0
  commit: abc1234
  build time: 2026-04-15_12:00:00
encore version v1.56.6
 Context:    default
Kubernetes control plane is running at https://xxx.xxx.xxx.xxx:6443
```

如果任何检查失败，请参阅[前置条件检查清单](prerequisites-checklist.md)。

---

## 步骤 1：初始化项目（30 秒）

```bash
infracast init demo-app --provider alicloud --region cn-hangzhou -y
cd demo-app
```

**预期输出**：
```
✓ Created infracast.yaml
✓ Created .infra/ directory
✓ Created .gitignore
✓ Project demo-app initialized for alicloud (cn-hangzhou)
```

将 health-check 示例应用复制到项目中：

```bash
cp -r /path/to/infracast/examples/health-check/* .
```

查看生成的配置：

```bash
cat infracast.yaml
```

---

## 步骤 2：构建空跑（1 分钟）

在无云资源的情况下测试构建流水线：

```bash
infracast deploy --env dev --dry-run
```

**预期输出**：
```
[DRY-RUN] Build: would run `encore build docker`
[DRY-RUN] Push: would push to ACR
[DRY-RUN] Deploy: would apply K8s manifests
No resources were created. Run without --dry-run to execute.
```

这可以验证 CLI、配置和 Encore 项目是否正确设置。

> **空跑演示到此结束。** 步骤 3–7 需要真实的阿里云资源。

---

## 步骤 3：预配基础设施（3–5 分钟）

创建云资源（VPC、RDS、Redis）：

```bash
infracast provision --env dev
```

**预期输出**：
```
✓ VPC created (vpc-bp1xxx)
✓ VSwitch created (vsw-bp1xxx)
✓ RDS PostgreSQL created (pgm-bp1xxx)
✓ Redis created (r-bp1xxx)
✓ Generated infracfg.json

Provision complete. Resources ready in env dev.
```

等待期间，可解释正在发生的事情：
- VPC + VSwitch：所有资源的私有网络
- RDS PostgreSQL：托管数据库，自动生成安全密码
- Redis：托管缓存，仅允许 VPC 内访问
- `infracfg.json`：应用的连接信息

**如果失败**：在[错误码矩阵](error-code-matrix.md)中查找错误码。
常见原因：`EPROV003`（余额不足）→ 为账户充值。

---

## 步骤 4：部署应用（2–3 分钟）

构建、推送并部署应用：

```bash
infracast deploy --env dev
```

**预期输出**：
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

流水线说明：
1. **构建**：`encore build docker` 创建容器镜像
2. **推送**：镜像被推送到阿里云容器镜像仓库（ACR）
3. **部署**：Kubernetes 清单被应用到 ACK 集群
4. **健康检查**：验证 `/livez` 端点健康

如果健康检查失败（EDEPLOY050）：
```bash
# 检查出错原因
infracast logs --trace trc_xxx
kubectl get pods -n demo-app-dev
kubectl logs <pod-name> -n demo-app-dev
```

---

## 步骤 5：验证部署（1 分钟）

获取服务端点并检查应用健康状态：

```bash
# 获取服务外部 IP/端口
kubectl get svc -n demo-app-dev
# 注意 EXTERNAL-IP，或在演示中使用端口转发：
kubectl port-forward svc/demo-app -n demo-app-dev 8080:80 &
```

```bash
# 存活探针
curl -s http://localhost:8080/livez | jq .
```

**预期输出**：
```json
{
  "status": "ok",
  "uptime": "45s"
}
```

```bash
# 就绪探针
curl -s http://localhost:8080/readyz | jq .
```

**预期输出**：
```json
{
  "status": "ready",
  "checks": {
    "self": "ok"
  }
}
```

```bash
# 诊断（显示已连接的资源）
curl -s http://localhost:8080/diag | jq .
```

```bash
# 完成后停止端口转发
kill %1
```

---

## 步骤 6：查看审计日志（30 秒）

显示部署历史：

```bash
# 最近日志
infracast logs --limit 10
```

**预期输出**：
```
Audit Logs (10 entries):

TIME                TRACE           LEVEL  ACTION     STEP          STATUS  ENV  DURATION  MESSAGE
----                -----           -----  ------     ----          ------  ---  --------  -------
2026-04-15 16:35    trc_17131...    INFO   deploy     health-check  ok      dev  3s        Health check passed
2026-04-15 16:35    trc_17131...    INFO   deploy     deploy        ok      dev  15s       K8s manifests applied
2026-04-15 16:34    trc_17131...    INFO   deploy     push          ok      dev  8s        Image pushed to ACR
2026-04-15 16:34    trc_17131...    INFO   deploy     build         ok      dev  12s       Docker image built
2026-04-15 16:30    trc_17130...    INFO   provision  -             ok      dev  180s      Provision complete

  ● 5 info
```

显示 JSON 输出（用于自动化）：

```bash
infracast logs --trace trc_xxx --format json | jq '.[].step'
```

---

## 步骤 7：通知（30 秒）

配置后，Infracast 会通过飞书或钉钉 Webhook 自动发送部署通知。

### 配置（部署前）

添加到 `infracast.yaml`：

```yaml
notifications:
  feishu:
    webhook_url: "https://open.feishu.cn/open-apis/bot/v2/hook/your-token"
  # 或钉钉：
  # dingtalk:
  #   webhook_url: "https://oapi.dingtalk.com/robot/send?access_token=your-token"
```

### 触发条件

通知会在 `infracast deploy` 结束时自动触发：
- **成功**：`✅ Deployment success`，包含应用、环境、提交、耗时
- **失败**：`❌ Deployment failed`，包含错误详情
- **回滚**：`🔄 Deployment rollback`，当自动回滚触发时

### 验证

配置通知后，完成一次部署：

```bash
# 检查审计日志中的通知步骤
infracast logs --trace <trace_id> --output wide
```

在流水线输出中查找 `notify` 步骤。如果 Webhook 投递失败，会记录为警告（非阻塞 — 通知失败不会导致部署失败）。

没有 Webhook（空跑演示）：未配置 Webhook 时，通知会被静默跳过。你可以通过检查审计日志来验证通知代码路径 — 如果 `notify` 步骤的状态为 `skip`，说明没有设置 Webhook。

---

## 步骤 8：清理（1–2 分钟）

始终先空跑：

```bash
infracast destroy --env dev --dry-run --keep-vpc 1
```

**预期输出**：
```
[DRY-RUN] Would delete:
  - RDS: pgm-bp1xxx
  - Redis: r-bp1xxx
  - VSwitch: vsw-bp1xxx (skipped, --keep-vpc)
  - VPC: vpc-bp1xxx (skipped, --keep-vpc)

No resources were deleted. Run with --apply to execute.
```

确认无误后执行：

```bash
infracast destroy --env dev --apply --keep-vpc 1
```

清理演示项目：

```bash
cd ..
rm -rf demo-app
```

---

## 演示要点

### Infracast 的功能
- 通过一条命令将 Encore 应用部署到阿里云
- 管理完整生命周期：初始化 → 预配 → 部署 → 验证 → 销毁
- 生成安全凭证、配置网络、处理健康检查

### 需要强调的关键特性
- **Trace ID**：每次部署都会生成一个 trace ID，实现全流程可见性
- **自动回滚**：健康检查失败会自动触发回滚到上一个版本
- **审计日志**：所有操作都通过结构化错误码记录
- **JSON 输出**：`--format json` 便于脚本化和 CI/CD 集成
- **空跑**：任何云操作前的安全预览

### 常见问题

| 问题 | 回答 |
|------|------|
| 费用是多少？ | 开发环境约 ¥20–25/天；测试完毕后销毁 |
| 支持哪些云服务商？ | 阿里云（专注单云以保障可靠性） |
| 如果部署失败怎么办？ | 自动回滚 + trace ID 用于诊断 |
| 可以使用自己的 K8s 集群吗？ | 可以，将 KUBECONFIG 指向任意 ACK 集群即可 |
| 如何添加更多环境？ | `infracast env create staging --provider alicloud --region cn-shanghai` |

---

## 精简演示（5 分钟，仅空跑）

适用于无云环境访问的演示：

```bash
# 1. 初始化
infracast init demo-app --provider alicloud --region cn-hangzhou -y
cd demo-app

# 2. 展示配置
cat infracast.yaml

# 3. 展示环境
infracast env list

# 4. 展示审计日志（来自之前的运行）
infracast logs --limit 5

# 5. 展示错误码参考
head -30 /path/to/infracast/docs/error-code-matrix.md

# 6. 展示版本
infracast version
```
