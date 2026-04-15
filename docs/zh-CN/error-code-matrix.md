# 错误码矩阵

Infracast 全部错误码的完整参考文档。使用本文档诊断故障并找到可操作的修复方案。

另请参阅：[部署手册 — 故障决策树](deployment-manual.md#3-failure-decision-tree)

---

## ECFG — 配置错误

来源：`internal/config/`、`cmd/infracast/internal/commands/init.go`、`cmd/infracast/internal/commands/run.go`

| 错误码 | 来源 | 错误信息 | 触发条件 | 修复方案 | 是否可重试 |
|------|--------|---------|-------------------|----------------|-----------|
| ECFG001 | `internal/config/errors.go` | provider 为必填项 | `infracast.yaml` 缺失 `provider` 字段，或 `init` 未传入应用名称 | 在配置中添加 `provider: alicloud`，或在运行 `infracast init` 时传入应用名称 | 否 |
| ECFG002 | `internal/config/errors.go` | region 为必填项 | `infracast.yaml` 缺失 `region` 字段，或 `init` 中的应用名称无效 | 在配置中添加 `region: cn-hangzhou` | 否 |
| ECFG003 | `internal/config/errors.go` | 环境名称为必填项 | 在必需位置省略了 `--env` 标志，或配置文件未找到 | 添加 `--env dev` 标志，或先运行 `infracast init` | 否 |
| ECFG004 | `internal/config/errors.go` | 不支持的 provider | Provider 值不在允许列表中（`alicloud`） | 使用 `provider: alicloud` | 否 |
| ECFG005 | `internal/config/errors.go` | 无效的 region 格式 | Region 字符串不符合预期格式 | 使用有效的阿里云 region（例如 `cn-hangzhou`、`cn-shanghai`） | 否 |
| ECFG006 | `internal/config/errors.go` | 无效的环境名称 | 环境名称包含非法字符 | 仅使用字母、数字和连字符（例如 `dev`、`staging`、`production`） | 否 |
| ECFG007 | `internal/config/errors.go` | storage_gb 超出范围 | `storage_gb` 覆盖值 < 20 或 > 32768 | 将 `storage_gb` 设置为 20 到 32768 之间 | 否 |
| ECFG008 | `internal/config/errors.go` | replicas 超出范围 | `replicas` 覆盖值 < 1 或 > 100 | 将 `replicas` 设置为 1 到 100 之间 | 否 |
| ECFG009 | `internal/config/errors.go` | 无效的 CPU 格式 | CPU 资源格式无法识别 | 使用 `"500m"` 或 `"2"` 等格式 | 否 |
| ECFG010 | `internal/config/errors.go` | 无效的内存格式 | 内存资源格式无法识别 | 使用 `"512Mi"` 或 `"2Gi"` 等格式 | 否 |
| ECFG011 | `internal/config/errors.go` | 无效的数据库引擎 | 数据库引擎不在支持列表中 | 使用 `postgresql`（阿里云 RDS） | 否 |
| ECFG012 | `internal/config/errors.go` | 无效的数据库版本 | 数据库版本不受支持 | 查看阿里云 RDS 支持的版本 | 否 |
| ECFG013 | `internal/config/errors.go` | 无效的实例规格 | RDS 实例规格无法识别 | 使用有效的阿里云实例规格（例如 `pg.n2.medium.1`） | 否 |
| ECFG014 | `internal/config/errors.go` | memory_mb 超出范围 | `memory_mb` < 256 或 > 65536 | 设置为 256 到 65536 之间 | 否 |
| ECFG015 | `internal/config/errors.go` | 无效的缓存引擎 | 缓存引擎不在支持列表中 | 使用 `redis` | 否 |
| ECFG016 | `internal/config/errors.go` | 无效的缓存版本 | 缓存版本不受支持 | 查看阿里云 Redis 支持的版本 | 否 |
| ECFG017 | `internal/config/errors.go` | 无效的 ACL 值 | OSS ACL 值无法识别 | 使用 `private`、`public-read` 或 `public-read-write` | 否 |
| ECFG018 | `internal/config/errors.go` | 无效的逐出策略 | Redis 逐出策略无法识别 | 使用 `noeviction`、`allkeys-lru` 等 | 否 |
| ECFG019 | `internal/config/errors.go` | 环境未找到 | `--env` 的值与任何已定义的环境都不匹配 | 运行 `infracast env list` 查看可用环境 | 否 |
| ECFG020 | `internal/config/errors.go` | 加载配置失败 | `infracast.yaml` 缺失、损坏或无法读取 | 运行 `infracast init` 或修复 YAML 语法 | 否 |
| ECFG021 | `internal/config/errors.go` | 环境名称过长 | 环境名称超过 50 个字符 | 使用更短的环境名称 | 否 |

---

## EDEPLOY — 部署流水线错误

来源：`internal/deploy/`、`cmd/infracast/internal/commands/deploy.go`、`cmd/infracast/internal/commands/run.go`

### 构建（EDEPLOY001–002）

| 错误码 | 来源 | 错误信息 | 触发条件 | 修复方案 | 是否可重试 |
|------|--------|---------|-------------------|----------------|-----------|
| EDEPLOY001 | `internal/deploy/build.go` | 构建失败 / 超时 | Docker 构建失败或超出超时时间 | 检查 Dockerfile，确保 Docker 守护进程正在运行（`docker info`），查看构建输出 | 是 |
| EDEPLOY002 | `internal/deploy/build.go` | 无效的构建元数据 | 缺失 AppName、BuildCommit 或 Services | 确保项目具有有效的 Encore 应用配置 | 否 |

### K8s 部署（EDEPLOY010–015）

| 错误码 | 来源 | 错误信息 | 触发条件 | 修复方案 | 是否可重试 |
|------|--------|---------|-------------------|----------------|-----------|
| EDEPLOY010 | `internal/deploy/k8s.go` | 清单为空 / 命名空间错误 | 生成的清单为空，或命名空间创建失败 | 检查 `infracfg.json` 的生成情况，验证集群连通性 | 是 |
| EDEPLOY011 | `internal/deploy/k8s.go` | K8s 客户端初始化失败 | `KUBECONFIG` 缺失或无效，ACK 集群无法访问 | 执行 `export KUBECONFIG=~/.kube/config`，验证 ACK 集群是否正在运行 | 否 |
| EDEPLOY012 | `internal/deploy/k8s.go` | `infracfg.json` 读取失败 / 清单生成失败 | 预配步骤后配置文件缺失 | 重新运行 `infracast provision --env <env>` | 否 |
| EDEPLOY013 | `internal/deploy/k8s.go` | ConfigMap 应用失败 | K8s API 拒绝了 ConfigMap | 检查集群权限，验证 ConfigMap 内容 | 是 |
| EDEPLOY014 | `internal/deploy/k8s.go` | Deployment 应用失败 | K8s API 拒绝了 Deployment 清单 | 检查集群权限，验证资源限制 | 是 |
| EDEPLOY015 | `internal/deploy/k8s.go` | Service 应用失败 | K8s API 拒绝了 Service 清单 | 检查集群权限，验证端口配置 | 是 |

### 镜像推送（EDEPLOY040–047）

| 错误码 | 来源 | 错误信息 | 触发条件 | 修复方案 | 是否可重试 |
|------|--------|---------|-------------------|----------------|-----------|
| EDEPLOY040 | `internal/deploy/docker.go` | 重试后镜像推送失败 / 镜像为空 | 推送到 ACR 失败 3 次，或本地镜像引用为空 | 检查 ACR 凭证（`docker login`），验证镜像是否构建成功 | 是 |
| EDEPLOY041 | `internal/deploy/docker.go` | 推送期间上下文被取消 | 推送被取消（超时或用户中断） | 重新执行部署 | 是 |
| EDEPLOY042 | `internal/deploy/docker.go` | 无效的镜像名称 | 镜像名称格式无效 | 检查应用配置中的镜像命名是否有效 | 否 |
| EDEPLOY043 | `internal/deploy/docker.go` | 解析源镜像失败 | 本地镜像引用无法解析 | 验证 Docker 构建是否生成了有效的镜像 | 否 |
| EDEPLOY044 | `internal/deploy/docker.go` | 解析目标镜像失败 | ACR 镜像引用无法解析 | 检查 ACR 仓库 URL 配置 | 否 |
| EDEPLOY045 | `internal/deploy/docker.go` | 加载源镜像失败 | 本地守护进程或仓库中未找到镜像 | 确保构建步骤已完成，运行 `docker images` | 否 |
| EDEPLOY046 | `internal/deploy/docker.go` | 推送到 ACR 失败 | ACR 拒绝了推送 | 重新认证：`docker login <acr-url>` | 是 |
| EDEPLOY047 | `internal/deploy/docker.go` | 获取 ACR 认证失败 | ACR 认证令牌获取失败 | 检查 `ALICLOUD_ACCESS_KEY`/`ALICLOUD_SECRET_KEY` | 否 |

### 健康检查与验证（EDEPLOY050–057）

| 错误码 | 来源 | 错误信息 | 触发条件 | 修复方案 | 是否可重试 |
|------|--------|---------|-------------------|----------------|-----------|
| EDEPLOY050 | `internal/deploy/health.go` | 部署超时 | Pod 在超时时间内未就绪；可能触发回滚 | 检查应用日志（`kubectl logs`），验证 `/health` 端点是否正常 | 是 |
| EDEPLOY051 | `internal/deploy/health.go` | 获取部署状态失败 | 健康检查轮询期间 K8s API 查询失败 | 检查集群连通性和 `KUBECONFIG` | 是 |
| EDEPLOY052 | `internal/deploy/health.go` | 获取 Deployment 失败 | 健康检查期间未找到 Deployment 资源 | 验证 Deployment 是否已应用（EDEPLOY014） | 否 |
| EDEPLOY053 | `internal/deploy/health.go` | K8s 客户端未初始化（健康检查） | 健康检查期间客户端为 nil | 确保已设置 `KUBECONFIG` | 否 |
| EDEPLOY054 | `internal/deploy/health.go` | 获取 Service 失败 | 未找到 Service 资源 | 验证 Service 是否已应用（EDEPLOY015） | 否 |
| EDEPLOY055 | `internal/deploy/health.go` | 创建健康检查请求失败 | HTTP 请求构造失败 | 检查健康检查端点 URL 格式 | 否 |
| EDEPLOY056 | `internal/deploy/health.go` | 健康检查 HTTP 失败 | 连接健康检查端点时发生网络错误 | 检查 Pod 状态和网络策略 | 是 |
| EDEPLOY057 | `internal/deploy/health.go` | 健康检查返回非 OK 状态 | 健康检查端点返回非 200 状态码 | 检查应用日志，验证应用是否正常启动 | 是 |

### 回滚（EDEPLOY060–068）

| 错误码 | 来源 | 错误信息 | 触发条件 | 修复方案 | 是否可重试 |
|------|--------|---------|-------------------|----------------|-----------|
| EDEPLOY060 | `internal/deploy/rollback.go` | 无先前版本 / K8s 客户端未初始化 | 首次部署（无版本可回滚），或客户端为 nil | 首次部署时：直接修复部署问题 | 否 |
| EDEPLOY061 | `internal/deploy/rollback.go` | 回滚执行失败 / 获取 Deployment 失败 | 回滚期间发生 K8s API 错误 | 检查集群权限和连通性 | 是 |
| EDEPLOY062 | `internal/deploy/rollback.go` | 回滚未稳定 / K8s 客户端未初始化 | 回滚后的 Pod 未恢复健康 | 手动干预：执行 `kubectl rollout status` | 是 |
| EDEPLOY063 | `internal/deploy/rollback.go` | 获取部署状态失败 | 回滚监控期间状态检查失败 | 检查集群连通性 | 是 |
| EDEPLOY064 | `internal/deploy/rollback.go` | 回滚进度超出截止时间 | 回滚本身超时 | 手动回滚：执行 `kubectl rollout undo` | 否 |
| EDEPLOY065 | `internal/deploy/rollback.go` | K8s 客户端未初始化（回滚） | 安全回滚检查期间客户端为 nil | 确保已设置 `KUBECONFIG` | 否 |
| EDEPLOY066 | `internal/deploy/rollback.go` | 未找到 Deployment（回滚） | 目标 Deployment 不存在 | 验证 Deployment 名称和命名空间 | 否 |
| EDEPLOY067 | `internal/deploy/rollback.go` | 无先前版本（安全检查） | 仅存在一个版本 | 首次部署无法回滚 | 否 |
| EDEPLOY068 | `internal/deploy/rollback.go` | 破坏性迁移阻止回滚 | 部署包含不可逆的数据库迁移 | 需要手动干预；请勿回滚 | 否 |

### 流水线（EDEPLOY070–082）

| 错误码 | 来源 | 错误信息 | 触发条件 | 修复方案 | 是否可重试 |
|------|--------|---------|-------------------|----------------|-----------|
| EDEPLOY070 | `internal/deploy/pipeline.go` | 构建失败（流水线） | Encore 构建步骤失败 | 检查构建输出，验证是否已安装 Encore CLI | 是 |
| EDEPLOY071 | `internal/deploy/pipeline.go` | 预配需要构建结果 | 未先执行构建就调用了预配步骤 | 确保先执行构建步骤 | 否 |
| EDEPLOY072 | `internal/deploy/pipeline.go` | 需要 ACR 凭证 / 初始化失败 | 缺少容器镜像仓库的阿里云凭证 | 设置 `ALICLOUD_ACCESS_KEY` 和 `ALICLOUD_SECRET_KEY` | 否 |
| EDEPLOY073 | `internal/deploy/pipeline.go` | 需要阿里云凭证 | 预配步骤需要云凭证 | 设置 `ALICLOUD_ACCESS_KEY` 和 `ALICLOUD_SECRET_KEY` | 否 |
| EDEPLOY074 | `internal/deploy/pipeline.go` | 创建 provider 失败 | 阿里云 provider 初始化失败 | 验证凭证和 region | 否 |
| EDEPLOY075 | `internal/deploy/pipeline.go` | 预配资源失败 | 数据库、缓存或 OSS 预配失败 | 查看阿里云控制台，验证配额和余额 | 是 |
| EDEPLOY080 | `cmd/.../run.go` | 未找到 encore CLI / 通知失败 | Encore 不在 PATH 中，或通知 Webhook 失败 | 安装 Encore CLI：https://encore.dev/docs/install | 否 |
| EDEPLOY081 | `cmd/.../run.go` | 启动 encore run 失败 | Encore 开发服务器启动失败 | 检查 Encore 项目配置，验证 `encore.app` 是否存在 | 否 |
| EDEPLOY082 | `cmd/.../run.go` | 未找到前置依赖 | PATH 中缺少必需的工具 | 安装缺失的工具（错误信息中会显示） | 否 |

---

## EPROV — 预配器错误

来源：`internal/provisioner/`

| 错误码 | 来源 | 错误信息 | 触发条件 | 修复方案 | 是否可重试 |
|------|--------|---------|-------------------|----------------|-----------|
| EPROV001 | `internal/provisioner/errors.go` | 获取凭证失败 | 阿里云凭证缺失或无效 | 设置 `ALICLOUD_ACCESS_KEY`/`ALICLOUD_SECRET_KEY`，验证 RAM 权限 | **否** |
| EPROV002 | `internal/provisioner/errors.go` | 云厂商 SDK 可重试错误 | 临时性云 API 错误（速率限制、网络问题） | 重试该命令 | **是** |
| EPROV003 | `internal/provisioner/errors.go` | 云资源配额超限 | 余额不足或 VPC/RDS/Redis 配额不足 | 充值账户、申请提升配额，或使用更小的规格 | 否 |
| EPROV004 | `internal/provisioner/errors.go` | 资源依赖冲突 | 资源存在冲突或无法解析的依赖 | 检查资源顺序，验证 VPC/VSwitch 是否存在 | 是 |
| EPROV005 | `internal/provisioner/errors.go` | 无效的资源规格 | 资源规格未通过验证 | 检查 `infracast.yaml` 中的资源定义 | 否 |
| EPROV006 | `internal/provisioner/errors.go` | 资源依赖未满足 | 所需的上游资源尚未预配 | 先预配依赖资源（VPC → VSwitch → RDS） | 否 |
| EPROV007 | `internal/provisioner/errors.go` | 资源销毁失败 | 云 API 拒绝了删除请求 | 等待异步释放，3–10 分钟后重试 | **是** |
| EPROV008 | `internal/provisioner/errors.go` | 计算规格哈希失败 | 内部哈希计算错误 | 作为 Bug 上报 | 否 |
| EPROV009 | `internal/provisioner/errors.go` | 资源未找到 | 预期的云资源不存在 | 重新运行预配，或检查资源是否被手动删除 | 否 |
| EPROV010 | `internal/provisioner/errors.go` | 检测到并发更新 | 另一个操作正在修改同一资源 | 等待后重试 | **是** |

---

## EIGEN — Infragen（配置生成）错误

来源：`pkg/infragen/`、`internal/infragen/`

| 错误码 | 来源 | 错误信息 | 触发条件 | 修复方案 | 是否可重试 |
|------|--------|---------|-------------------|----------------|-----------|
| EIGEN001 | `pkg/infragen/generator.go` | 无效 / 不支持的资源类型 | 预配结果包含未知的资源类型 | 检查 `infracast.yaml` 中支持的资源类型 | 否 |
| EIGEN002 | `pkg/infragen/generator.go` | 缺失必填字段 | 生成的配置缺少必需的连接信息 | 验证所有资源是否都已成功完成预配 | 否 |
| EIGEN003 | `pkg/infragen/generator.go` | 写入失败 | 无法将 `infracfg.json` 写入磁盘 | 检查磁盘权限，确保 `.infra/` 目录存在 | 否 |

---

## ESTATE — 状态数据库错误

来源：`cmd/infracast/internal/commands/logs.go`

| 错误码 | 来源 | 错误信息 | 触发条件 | 修复方案 | 是否可重试 |
|------|--------|---------|-------------------|----------------|-----------|
| ESTATE001 | `cmd/.../commands/logs.go` | 打开状态数据库失败 | `.infra/state.db` 缺失或损坏 | 运行 `infracast init` 创建状态目录，或检查文件权限 | 否 |
| ESTATE002 | `cmd/.../commands/logs.go` | 初始化审计表失败 | SQLite schema 迁移失败 | 检查磁盘空间和 `.infra/state.db` 的文件权限 | 否 |

---

## 快速查找

错误码总数：**78**

| 模块 | 范围 | 数量 |
|--------|-------|-------|
| ECFG | 001–021 | 21 |
| EDEPLOY | 001–082 | 42 |
| EPROV | 001–010 | 10 |
| EIGEN | 001–003 | 3 |
| ESTATE | 001–002 | 2 |

**不是错误码？** 如果你看到云厂商错误（例如 `NotEnoughBalance`、`ServiceLinkedRole.NotExist`），请参阅 [单云运维手册](06-single-cloud-operations.md#3-troubleshooting-runbook) 中针对阿里云故障排查的内容。
