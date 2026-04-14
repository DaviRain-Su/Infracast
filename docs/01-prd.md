# Infracast — Product Requirements Document

> **Version** 1.1 (Frozen) · **Date** 2026-04-14 · **Status** Frozen · **Confidential**

---

## v1.1 冻结决议

以下决策已在团队评审中确认，进入实施阶段后不再变更：

1. **Compute** = ACK Serverless（单一形态，后续再支持 ACK 标准版）
2. **配置文件** `infracast.yaml` 中 `provider` 与 `region` 为必填字段
3. **运行时配置** 主链路为 `infracfg.json`，由 CLI 生成并注入，用户可预置文件作为覆盖
4. **计费** 先只分 Free 与 Team 两档，Enterprise 进入 P2
5. **资源供应** 主策略为 SDK Direct，Terraform 进入后续可选项，不作为 P0 关键路径
6. **IR 依赖** 作为增强输入，不作为 P0 绑定契约；先靠 `infracast.yaml` + 构建元信息
7. **Pub/Sub** Redis Stream 作为降级方案，原生 RocketMQ/TDMQ 不进 P0
8. **商业模式** SaaS + 用户自带云账号（轻资产），平台只收服务费
9. **数据库迁移** Forward-only，失败时回滚到上一个应用版本，保留 DB 状态
10. **Runtime** P0 不 Fork Encore 核心 Runtime

---

## 1. Executive Summary 执行摘要

### 1.1 产品愿景

Infracast 是一个基于 Encore 开源框架（MPL 2.0）构建的 **Code-First 基础设施自动化平台**，专为中国云市场设计。开发者只需编写业务代码，平台通过静态分析自动推导基础设施需求，并将其映射到阿里云、华为云、腾讯云、火山引擎及运营商云的具体服务，实现从代码到生产环境的全自动化部署。

### 1.2 核心问题

1. **Encore 无法服务中国市场** — 仅支持 AWS/GCP，无法满足中国数据主权要求（《数据安全法》+《个人信息保护法》）。
2. **AI 加速暴露基础设施瓶颈** — DORA 研究：每 25% AI 采用率提升带来 7.2% 交付不稳定性增长。团队写代码的速度远超部署代码的速度。
3. **中国云生态碎片化** — 三大云 + 运营商云 + 专项服务商 SDK 互不兼容，多云策略实施成本极高。
4. **IaC 生态薄弱** — 国内 Terraform Provider 成熟度参差不齐，许多团队仍停留在手动/半自动化阶段。

### 1.3 产品目标

| 优先级 | 目标 |
|--------|------|
| **P0 Must Have** | 实现阿里云的完整 Code-First 部署链路（ACK Serverless + RDS + OSS + Redis） |
| **P0 Must Have** | 本地开发环境一键启动，与云端行为一致 |
| **P1 Should Have** | 华为云 + 腾讯云适配，覆盖市场 60%+ 的 IaaS 份额 |
| **P1 Should Have** | 原生 Pub/Sub 适配（阿里云 Kafka/RocketMQ） |
| **P1 Should Have** | 基于 OpenTelemetry 的可观测性对接 |
| **P2 Nice to Have** | 火山引擎 + 运营商云适配，七牛云等存储集成 |
| **P2 Nice to Have** | 多租户管理平台 + Preview 环境 + Enterprise 版 |

### 1.4 Non-Goals（不做的事）

- 不修改 Encore 开源框架的核心解析器和编译器（保持上游兼容）
- 不自研编程语言 SDK（复用 Encore 的 Go/TypeScript SDK）
- 不提供独立的 CDN/音视频处理等垂直 SaaS 功能
- 不支持 AWS/GCP 部署（这是 Encore Cloud 的领域）
- P0 不做原生 Pub/Sub、Cron、Secrets 服务映射、跨云切换、全量 Portal

### 1.5 Encore Fork 策略

**方式**: 构建时 Patch（Build-Time Patch），而非硬 Fork。

| 策略 | 说明 |
|--------|------|
| **基线** | 锁定 Encore 稳定版本（如 v1.38.x），作为 Go module 依赖引入 |
| **扩展点** | 通过 `go:generate` + AST rewrite 工具在构建时注入 CloudProvider 枚举值，而非直接修改源码 |
| **同步节奏** | 每季度评估上游新版本，运行兼容性测试套件后升级 |
| **Fallback** | 如构建时 Patch 不可行（如 Encore 重构了内部类型），退到浅 Fork + cherry-pick 模式 |
| **CI 保护** | 每日构建任务检测上游 HEAD 是否破坏 Patch，提前预警 |

---

## 2. User Personas 用户画像

| 角色 | 背景 | 痛点 | 期望 |
|------|------|------|------|
| **AI 创业工程师** | 2-5 人团队，使用 Cursor/Copilot 高速产出代码 | 代码写完了不知道怎么部署到阿里云，Terraform 学习成本高 | 写完代码 git push 就能跑在阿里云上 |
| **成长期后端 Lead** | 10-30 人团队，维护 5-15 个微服务 | 每个新服务都要重复配置 IaC + CI/CD + 监控，耗时 1-2 周 | 新服务从零到生产 < 1 天，基础设施自动跟随代码 |
| **企业平台工程师** | 负责内部开发者平台，服务 50+ 开发者 | 维护 Terraform 模块库消耗大量精力，各团队配置不一致 | 统一的 Code-First 平台替代自建 IDP |
| **出海回流 CTO** | 产品从海外回中国，从 AWS 迁移到国内云 | AWS 的 IaC/CI/CD 全部失效，需要重建 | 应用代码不改，切换部署目标即可 |

---

## 3. Competitive Analysis 竞争分析

| 维度 | **Infracast** | **Pulumi** | **Serverless Framework** | **阿里云 ROS** | **Encore Cloud** |
|------|---------------------|-----------|------------------------|--------------|----------------|
| **定位** | Code-First，代码即基础设施 | IaC-First，通用语言写基础设施 | Serverless 专用 | 阿里云原生 IaC | Code-First，AWS/GCP 专用 |
| **中国云支持** | ✅ 阿里/华为/腾讯/火山 | ⚠️ 阿里云 Provider 基本可用，其他厂商缺失 | ❌ 仅支持腾讯云 SCF | ✅ 阿里云原生，仅此一家 | ❌ 无中国云支持 |
| **开发者体验** | 零 IaC 代码，写业务即部署 | 需写 IaC 代码（只是用 Go/TS 而非 HCL） | 需写 serverless.yml | 需写 ROS 模板（JSON/YAML） | 零 IaC 代码 |
| **多云** | ✅ 统一接口，切换 Provider 即可 | ✅ 多云，但需重写 IaC 代码 | ❌ 厂商锁定 | ❌ 阿里云单一 | ❌ AWS/GCP 双云 |
| **本地开发** | ✅ 一键本地环境（复用 Encore） | ❌ 无本地模拟 | ⚠️ 仅 serverless-offline | ❌ 无 | ✅ 一键本地环境 |
| **可观测性** | ✅ 自动对接国内云 APM | ❌ 需自行配置 | ⚠️ 参差不齐 | ⚠️ 仅阿里云 ARMS | ✅ 自动对接 |
| **学习曲线** | 低（写业务代码即可） | 中（需学 Pulumi 概念） | 中 | 高（ROS 模板语法） | 低 |
| **开源** | ✅ 核心开源 | ✅ 开源 | ✅ 开源 | ❌ 闭源 | ⚠️ 框架开源，平台闭源 |

### 与国内平台对比

| 维度 | **Infracast** | **Rainbond** | **KubeSphere** | **Sealos** |
|------|--------------|-------------|----------------|-----------|
| **核心理念** | Code-First，从代码声明到资源 | 应用级云原生平台 | K8s 管理平台 | 云操作系统 |
| **差异化** | 零 IaC + 多云目标映射 + 少改代码跨云 | 应用级抽象，仍需配置 | 侧重 K8s 运维 | 侧重容器化 |
| **目标用户** | 写代码的开发者 | 运维 + 开发者 | 运维工程师 | 开发者 |

**核心差异化**:

1. **Code-First + 中国云 = 唯一组合** — Encore 不做中国，Pulumi/Terraform 不做 Code-First，ROS 不做多云
2. **零 IaC 代码的多云体验** — 同一份业务代码，切换 `provider: huaweicloud` 即可部署到不同云
3. **本地开发体验** — 复用 Encore 的本地基础设施模拟，这是 Pulumi/Terraform 无法提供的

---

## 4. System Architecture 系统架构

### 4.1 架构分层

系统采用三层架构，清晰划分复用层和自研层。

#### Layer 1: Encore Open Source Framework（复用层）

- **来源**: github.com/encoredev/encore (MPL 2.0)
- **组件**: Go/TypeScript SDK, Code Parser, Compiler, Runtime, CLI, Local Dev Tools, MCP Server
- **职责**: 提供声明式基础设施语义、编译时代码分析、本地开发环境、API 文档生成、架构图生成
- **修改策略**: 原则上不修改。如需扩展（如添加 CloudProvider 枚举值），通过构建时 Patch 维护。Fallback 到浅 Fork + cherry-pick 仅作为 P1/P2 风险应对方案，不作为 P0 交付路径

#### Layer 2: Infracast Platform（自研层 — 核心产品）

| 模块 | 职责 | 核心技术 | 输入 → 输出 |
|------|------|---------|------------|
| **Service Mapper** 服务映射引擎 | 将 Encore 的基础设施语义映射到目标云厂商的具体服务标识 | Go, Provider Registry | infracast.yaml + 构建元信息 → CloudResourceSpec[] |
| **Provisioner** 资源供应器 | 通过云厂商 SDK 创建/更新/删除基础设施资源 | Go, 各云 SDK Direct | CloudResourceSpec[] → 云资源实例 |
| **Config Generator** 配置生成器 | 生成 Encore Runtime 可消费的 `infracfg.json` | Go | 云资源连接信息 → infracfg.json |
| **Deploy Pipeline** 部署管道 | 执行 Docker 镜像构建、资源供应、配置注入、K8s 部署、健康检查 | Go, Docker SDK, K8s client | Git push event → Running deployment |
| **Observability Bridge** 可观测性桥接 | 将 Encore Runtime 的 tracing/metrics 数据桥接到各云厂商的 APM 服务 | OpenTelemetry Collector, OTLP exporters | OTLP spans/metrics → Vendor APM data |
| **Environment Manager** 环境管理器 | 管理开发、Preview、Staging、Production 等多环境的生命周期 | Go, Kubernetes operators | Environment config → Isolated env instances |
| **Management Portal** 管理平台 | Web UI 用于项目管理、环境查看、日志追踪、团队协作、计费 | React/Next.js, TypeScript | User actions → Platform state views |

#### Layer 3: Cloud Provider Adapters（云厂商适配层）

每个云厂商对应一个 Adapter 包，实现统一的 `CloudProviderInterface`。

| 阶段 | 云厂商 | SDK 成熟度 | 备注 | 目标时间 |
|------|--------|----------|------|----------|
| Phase 1 (MVP) | 阿里云 | ★★★★★ | 市占 33%，SDK 最成熟 | Q3 2026 |
| Phase 2 | 华为云 | ★★★★ | 市占 18%，政企渗透率高 | Q4 2026 |
| Phase 2 | 腾讯云 | ★★★★ | 市占 8%，游戏/互联网强势 | Q4 2026 |
| Phase 3 | 火山引擎 | ★★★ | AI 云增速最快（14.8%） | Q1 2027 |
| Phase 3 | 天翼云 | ★★ | 运营商云，政务必需 | Q1 2027 |
| Phase 4 | 七牛云 (存储) | N/A | 对象存储/CDN 替代后端 | Q2 2027 |

### 4.2 核心部署链路（v1.1）

```
infracast.yaml + encore build output
    │
    ▼
Service Mapper → ResourceSpec[]
    │
    ▼
AlicloudAdapter (SDK Direct) → 创建/更新云资源 → 写入 infra state
    │
    ▼
Config Generator → 生成 infracfg.json（对齐 Encore 官方 schema）
    │
    ▼
镜像构建 → 推送 ACR → 部署到 ACK Serverless
    │
    ▼
健康检查 → 通过则上线，失败则回滚到上一应用版本
    │
    ▼
通知（飞书/钉钉）+ 审计日志
```

---

## 5. Core Interfaces 核心接口定义

### 5.1 CloudProviderInterface

每个云厂商 Adapter 必须实现此接口（P0 仅需实现标注方法）：

```go
type CloudProviderInterface interface {
    // 基础信息
    Name() string                    // e.g. "alicloud", "huaweicloud"
    DisplayName() string             // e.g. "阿里云", "华为云"
    Regions() []Region               // 可用区域列表

    // P0 资源供应
    ProvisionDatabase(spec DatabaseSpec) (*DatabaseOutput, error)
    ProvisionObjectStorage(spec ObjectStorageSpec) (*ObjectStorageOutput, error)
    ProvisionCache(spec CacheSpec) (*CacheOutput, error)
    ProvisionCompute(spec ComputeSpec) (*ComputeOutput, error)

    // P1 资源供应（P0 阶段 stub）
    ProvisionPubSub(spec PubSubSpec) (*PubSubOutput, error)
    ProvisionSecrets(spec SecretsSpec) (*SecretsOutput, error)
    ProvisionCronJob(spec CronSpec) (*CronOutput, error)

    // 生命周期
    Plan(specs []ResourceSpec) (*PlanResult, error)    // dry-run
    Apply(plan *PlanResult) (*ApplyResult, error)      // 执行供应
    Destroy(env EnvironmentID) error                   // 销毁环境

    // 可观测性
    OTLPEndpoint() string                              // OpenTelemetry 导出端点
    DashboardURL(env EnvironmentID) string             // APM 仪表盘链接
}
```

### 5.2 ResourceSpec 资源规格定义

```go
type DatabaseSpec struct {
    Name          string            // Encore 代码中的数据库名
    Engine        DBEngine          // POSTGRES | MYSQL
    Version       string            // e.g. "15", "8.0"
    InstanceClass string            // 映射到各云厂商的实例规格
    StorageGB     int               // 存储容量
    HighAvail     bool              // 是否高可用
    Migrations    []MigrationFile   // SQL 迁移文件
}

type ObjectStorageSpec struct {
    Name       string
    ACL        BucketACL            // PRIVATE | PUBLIC_READ
    CORSRules  []CORSRule
    Lifecycle  []LifecycleRule      // 对象生命周期
}

type CacheSpec struct {
    Name           string
    Engine         CacheEngine      // REDIS
    Version        string
    MemoryMB       int
    EvictionPolicy string
}

type PubSubSpec struct {
    Name          string
    TopicType     TopicType         // STANDARD | FIFO
    Subscriptions []SubSpec
}

type ComputeSpec struct {
    ServiceName string
    Replicas    int
    CPU         string              // e.g. "1000m"
    Memory      string              // e.g. "512Mi"
    Port        int
    EnvVars     map[string]string   // 非敏感环境变量
    SecretRefs  []string            // 引用的 Secret 名称
}

type CronSpec struct {
    Name       string
    Schedule   string               // cron 表达式
    Endpoint   string               // 触发的 API 端点
    HTTPMethod string
}
```

### 5.3 配置文件 infracast.yaml

用户通过项目根目录的配置文件覆盖默认映射。`provider` 和 `region` 为**必填字段**。

```yaml
# infracast.yaml
provider: alicloud       # 必填
region: cn-hangzhou       # 必填

environments:
  production:
    provider: alicloud
    region: cn-shanghai
  staging:
    provider: huaweicloud
    region: cn-north-4

overrides:
  databases:
    mydb:
      instance_class: rds.mysql.s3.large
      storage_gb: 100
      high_avail: true
  
  compute:
    api-service:
      replicas: 3
      cpu: "2000m"
      memory: "1Gi"

notifications:
  deploy:
    - type: feishu
      url: https://open.feishu.cn/open-apis/bot/v2/hook/xxx
    - type: dingtalk
      url: https://oapi.dingtalk.com/robot/send?access_token=xxx
```

### 5.4 运行时配置 infracfg.json

由 CLI 生成，注入到应用容器。Schema 严格对齐 Encore 官方 `infracfg` 格式：

- **主链路**: CLI 根据 ResourceSpec + 云资源连接信息自动生成
- **覆盖层**: 用户可在 `infra/` 目录预置基础配置，CLI 合并覆盖
- **注入方式**: 写入 K8s ConfigMap → 容器挂载

P0 覆盖字段：`sql_servers`、`redis`、`object_storage`

---

## 6. Cloud Service Mapping 云服务映射

| ResourceSpec | 阿里云 | 华为云 | 腾讯云 | 火山引擎 | 七牛云 |
|-------------|-------|--------|--------|---------|-------|
| **DatabaseSpec** | RDS for MySQL/PG, PolarDB | RDS for MySQL/PG, GaussDB | TDSQL-C, CDB | RDS for MySQL/PG | N/A |
| **ObjectStorageSpec** | OSS | OBS | COS | TOS | Kodo |
| **CacheSpec** | Tair (Redis) | DCS (Redis) | TencentDB Redis | Redis 托管版 | N/A |
| **PubSubSpec** | RocketMQ, Kafka | DMS for Kafka, SMN | TDMQ (Pulsar), CMQ | RocketMQ 托管版 | N/A |
| **ComputeSpec** | ACK Serverless | CCE (K8s) | TKE (K8s) | VKE (K8s) | N/A |
| **SecretsSpec** | KMS | DEW + KMS | SSM + KMS | KMS | N/A |
| **CronSpec** | FC 定时触发器 | FunctionGraph | SCF 定时触发 | 函数服务 | N/A |
| **Observability** | ARMS + SLS | AOM + LTS | APM + CLS | APM + TLS | N/A |

---

## 7. Module Specifications 模块详细设计

### 7.1 Service Mapper 服务映射引擎

**职责**: 解析 `infracast.yaml` + 构建元信息，转换为 CloudResourceSpec。

**构建元信息最小字段清单**（供 Task 2 服务映射 / Task 3 配置生成使用）:

| 字段 | 类型 | 说明 |
|------|------|------|
| `app_name` | string | 应用名，用于资源命名前缀 |
| `services` | []ServiceMeta | 服务列表（名称、入口路径、端口） |
| `databases` | []DatabaseMeta | 声明的数据库（名称、迁移目录路径） |
| `caches` | []CacheMeta | 声明的缓存（名称、集群名） |
| `object_stores` | []ObjectStoreMeta | 声明的对象存储（桶名） |
| `pubsub_topics` | []PubSubMeta | 声明的 Pub/Sub 主题（名称、订阅者） |
| `build_commit` | string | 构建对应的 Git commit SHA |
| `build_image` | string | 构建产物 Docker 镜像 tag |

**处理流程**:

```
infracast.yaml + 构建元信息
    │
    ▼
┌─────────────────┐
│  Parse Config   │  提取资源声明
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Resolve Deps   │  构建资源依赖图 (DAG)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Map to Spec    │  标准化为 ResourceSpec[]
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Apply Override  │  合并 infracast.yaml 覆盖值
└────────┬────────┘
         │
         ▼
  CloudResourceSpec[]
```

**关键设计决策**:

- 采用 **Provider Registry** 模式，运行时动态加载云厂商 Adapter（`provider.Register("alicloud", NewAlicloudAdapter)`）
- ResourceSpec 支持 **Override** 机制，用户可通过 `infracast.yaml` 覆盖默认映射
- 依赖图支持 **并行供应**（无依赖关系的资源可并行创建）
- P0 不依赖 Encore IR 作为核心契约（IR 稳定性风险），后续可增强

### 7.2 Provisioner 资源供应器

**策略**: P0 使用 **SDK Direct**，直接调用各云厂商 Go SDK 创建资源。

**凭证管理**:
- 优先使用 **STS 临时凭证**（Security Token Service）+ AssumeRole
- 支持 AK/SK 回退（开发/测试场景）
- 不落盘明文密钥

**幂等控制**:
- 每个资源用 `(env_id, resource_name)` 唯一标识
- Create 前查询已有资源
- `spec_hash()` 只 hash 影响资源创建的 spec 字段（排除 metadata/timestamp）
- hash 相同 → 跳过；hash 不同 → 更新 + version++；不存在 → 创建

### 7.3 Deploy Pipeline 部署管道（v1.1）

**部署链路（7 步）**:

```
Step 1 — Build
  └─ 运行 encore build，生成 Docker 镜像

Step 2 — Map
  └─ 调用 Service Mapper，生成 CloudResourceSpec[]

Step 3 — Provision
  └─ SDK Direct 创建/更新资源，写入 infra state

Step 4 — Configure
  └─ 生成 infracfg.json（对齐 Encore schema）
  └─ 合并用户预置覆盖

Step 5 — Deploy
  └─ Docker 镜像 → ACR 推送
  └─ 生成标准 K8s Deployment + Service YAML
  └─ 部署到 ACK Serverless

Step 6 — Verify
  └─ 健康检查（Pod Ready + Endpoint 可达）
  └─ 超时 5 分钟未通过 → 回滚到上一应用版本（forward-only）

Step 7 — Notify
  └─ 飞书/钉钉 Webhook 通知
  └─ 记录审计日志
```

**回滚策略**:

- Step 3 失败: SDK 操作幂等，可安全重试
- Step 5-6 失败: Kubernetes rollback 到上一个 Deployment revision
- **数据库迁移**: Forward-only，失败时回滚到上一应用版本，保留 DB 状态（不做 destructive DB rollback）

### 7.4 Observability Bridge 可观测性桥接

**架构**:

```
Encore Runtime (App)
    │
    │ OTLP (gRPC/HTTP)
    ▼
OpenTelemetry Collector (Sidecar / DaemonSet)
    │
    ├─→ 阿里云 ARMS Exporter → ARMS Console
    ├─→ 华为云 AOM Exporter  → AOM Console
    ├─→ 腾讯云 APM Exporter  → APM Console
    └─→ Fallback: Jaeger / Prometheus (自建)
```

---

## 8. Data Model 数据模型

### Organization 组织

```sql
CREATE TABLE organizations (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(100) NOT NULL,
    plan          VARCHAR(20) NOT NULL DEFAULT 'free',  -- free | team
    billing_email VARCHAR(255),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Project 项目

```sql
CREATE TABLE projects (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id           UUID NOT NULL REFERENCES organizations(id),
    name             VARCHAR(100) NOT NULL,
    repo_url         VARCHAR(500),
    default_provider VARCHAR(50) NOT NULL DEFAULT 'alicloud',
    default_region   VARCHAR(50) NOT NULL DEFAULT 'cn-hangzhou',
    encore_version   VARCHAR(20),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Environment 环境

```sql
CREATE TABLE environments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id),
    name            VARCHAR(50) NOT NULL,
    provider        VARCHAR(50) NOT NULL,
    region          VARCHAR(50) NOT NULL,
    status          VARCHAR(20) NOT NULL,       -- planning | provisioning | active | failed | destroying | destroyed
    kubernetes_ns   VARCHAR(100),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Deployment 部署

```sql
CREATE TABLE deployments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    env_id       UUID NOT NULL REFERENCES environments(id),
    git_sha      VARCHAR(40) NOT NULL,
    git_branch   VARCHAR(200),
    git_message  TEXT,
    status       VARCHAR(20) NOT NULL,          -- pending | building | planning | provisioning | deploying | live | failed | rolled_back
    started_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    logs_url     VARCHAR(500),
    error_msg    TEXT
);
```

### CloudCredential 云凭证

```sql
CREATE TABLE cloud_credentials (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                UUID NOT NULL REFERENCES organizations(id),
    provider              VARCHAR(50) NOT NULL,
    name                  VARCHAR(100) NOT NULL,
    access_key_encrypted  BYTEA NOT NULL,          -- AES-256-GCM 加密
    secret_key_encrypted  BYTEA NOT NULL,
    region                VARCHAR(50),
    session_model         JSONB,                    -- 通用角色/会话模型（STS/AssumeRole 等）
    verified_at           TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### InfraResource 基础设施资源

```sql
CREATE TABLE infra_resources (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    env_id               UUID NOT NULL REFERENCES environments(id),
    resource_type        VARCHAR(50) NOT NULL,    -- database | object_storage | cache | compute
    resource_name        VARCHAR(100) NOT NULL,   -- Encore 代码中的资源名
    provider_resource_id VARCHAR(500),            -- 云厂商返回的资源 ID
    provider_console_url VARCHAR(500),            -- 云厂商控制台链接
    status               VARCHAR(20) NOT NULL,
    config_json          JSONB,                   -- 资源配置快照
    config_hash          VARCHAR(64),             -- spec_hash，仅 hash 资源 spec 字段
    state_version        INT NOT NULL DEFAULT 1,  -- 乐观锁，防止并发修改
    cost_per_hour        DECIMAL(10, 4),
    last_synced_at       TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 幂等约束：同一环境下资源名唯一
CREATE UNIQUE INDEX idx_infra_resources_env_name
ON infra_resources(env_id, resource_name);
```

### AuditLog 审计日志

```sql
CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL,
    user_id     UUID,
    action      VARCHAR(50) NOT NULL,
    target_type VARCHAR(50),
    target_id   UUID,
    details     JSONB,
    ip_address  INET,
    user_agent  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 审计日志不可删除，仅追加
```

---

## 9. Business Model 商业模式

### 9.1 经营模式

**SaaS + 用户自带云账号（轻资产）**

| 项目 | 责任方 |
|-----|-------|
| 云资源账单 | 用户阿里云账户直出 |
| 平台服务费 | Infracast 收取（Free/Team 两档） |
| 资源创建/管理 | Infracast 平台代操作 |
| 数据主权 | 用户完全拥有 |

平台不代扣代缴云资源账单，只收平台服务费。后续再评估多云托管代付模型。

### 9.2 定价模型

| 计划 | 月费 | 包含内容 | 目标用户 |
|------|------|---------|----------|
| **Free** | ￥0 | 1 个项目、1 个生产环境、每月 50 次部署、社区支持 | AI 创业工程师、个人开发者 |
| **Team** | ￥299/人/月 | 无限项目、多环境、不限部署、Preview 环境（P1）、邮件支持 | 成长期团队 (5-30 人) |
| **Enterprise** | 定制报价（P2） | 无限项目/环境、私有化部署、SSO、SLA 99.99%、专属支持 | 平台工程师、大企业 |

**计费维度**: 按席位（团队成员数）计费。理由：可预测性好、简单透明、升级路径清晰。

---

## 10. Development Phases 开发阶段

### Phase 1: MVP (Q3 2026, 16 weeks)

**目标**: 实现阿里云的完整 Code-First 部署链路（可复现、可回滚、可观测）

#### Milestone A (Week 1-2): 可行性验证

- [ ] PoC-A: `encore build` + 自托管 config（生成 `infracfg`）可运行
- [ ] PoC-B: 阿里云 SDK 创建 RDS/Redis/OSS + 连接信息回灌
- [ ] 项目脚手架（Go monorepo, CI/CD, 测试框架）
- [ ] Provider Registry + Adapter 插件加载机制
- [ ] `infracast.yaml` 解析与校验（provider/region 必填）
- [ ] `infracfg.json` 生成器 PoC（对齐 Encore 官方 schema）
- [ ] 资源状态表幂等逻辑（spec_hash + 乐观锁）
- [ ] CLI 命令骨架（init/run/deploy）
- [ ] **验收**: 给定 demo app，能生成配置并成功启动最小服务

#### Milestone B (Week 3-6): 核心供应链

- [ ] AlicloudAdapter: database/cache/object_storage SDK Direct 实现
- [ ] STS/AssumeRole 临时凭证支持
- [ ] 幂等资源创建、状态记录、失败可重试
- [ ] **验收**: 资源供应成功率 >= 95%

#### Milestone C (Week 7-10): 部署链路闭环

- [ ] 镜像构建/推送 ACR + ACK Serverless 部署
- [ ] 标准 K8s Deployment + Service YAML 生成
- [ ] 本地一致性: `infracast run` 与云端环境变量/配置对齐
- [ ] 健康检查、失败回滚（应用版本回滚，forward-only）
- [ ] **验收**: 2 个示例 app + 1 条迁移变更可复现上线

#### Milestone D (Week 11-14): 最小产品化

- [ ] CLI 发布: `infracast init / run / deploy / env`
- [ ] 审计日志、基本通知（飞书/钉钉）
- [ ] 部署手册与示例库
- [ ] **验收**: 3 个示例应用在阿里云可重复部署

#### Milestone E (Week 15-16): Gate + 兼容复盘

- [ ] 失败率分析与状态一致性评审
- [ ] 形成"接口冻结清单"（infracast.yaml 字段、infracfg schema、Provider Registry contract、state 幂等规则）
- [ ] 决定 Phase 2 入口（是否进入跨云 + Pub/Sub 原生）

#### MVP 交付物

1. `infracast` CLI 工具（macOS + Linux）
2. AlicloudAdapter 包（Database + ObjectStorage + Cache + Compute）
3. 3 个示例应用 + 部署教程
4. 开发者文档（快速开始 + API 参考）

---

### Phase 2: Multi-Cloud (Q4 2026, 12 weeks)

**目标**: 华为云 + 腾讯云适配 + 原生 Pub/Sub

- [ ] `HuaweiCloudAdapter` 实现（CCE + RDS + OBS + DCS + DMS）
- [ ] `TencentCloudAdapter` 实现（TKE + TDSQL + COS + Redis + TDMQ）
- [ ] 原生 Pub/Sub 适配（阿里云 Kafka/RocketMQ 取其一）
- [ ] OpenTelemetry Collector 配置自动生成
- [ ] ARMS / AOM / APM Exporter 对接
- [ ] Preview 环境系统
- [ ] `infracast.yaml` Override 机制完善
- [ ] **完成度指标**: 3 个示例应用在三个云厂商均可部署，Traces 在 APM 控制台可见

---

### Phase 3: Full Stack (Q1 2027, 12 weeks)

**目标**: 全面云覆盖 + 平台商业化

- [ ] 火山引擎 Adapter（VKE + RDS + TOS + Redis + RocketMQ）
- [ ] 天翼云 Adapter（基础 IaaS 支持）
- [ ] 七牛云 Kodo 作为 ObjectStorage 替代后端
- [ ] 多租户管理平台 Web UI（React/Next.js）
- [ ] 用户注册/认证（OIDC + JWT）
- [ ] 计费系统

---

### Phase 4: Scale (Q2 2027, 12 weeks)

**目标**: 企业级功能 + 社区建设

- [ ] 企业版私有化部署方案（Helm chart + 离线安装包）
- [ ] 合规审计仪表盘
- [ ] 云厂商市场上架（阿里云市场 / 华为云市场）
- [ ] 开发者社区 + 文档站 + 示例应用库
- [ ] 国产 AI 工具集成（通义灵码 / CodeGeeX MCP 支持）

---

## 11. Non-Functional Requirements 非功能需求

| 类别 | 指标 | 要求 |
|------|------|------|
| 性能 | 部署延迟 | 从 git push 到生产环境运行 < 10 分钟（p95，不含首次基础设施创建） |
| 性能 | 本地启动 | `infracast run` 冷启动 < 30 秒（含本地 DB/Redis） |
| 可用性 | 平台 SLA | 管理平台 99.9%（Team 版） |
| 可用性 | 部署回滚 | 失败部署自动回滚 < 2 分钟 |
| 安全 | 凭证管理 | AES-256-GCM 加密存储，运行时优先 STS 临时凭证，不落盘明文 |
| 安全 | 网络隔离 | 每个环境独立 VPC/VSwitch，服务间最小权限网络策略 |
| 安全 | 审计日志 | 所有操作记录不可篡改审计日志（append-only），保留 >= 180 天 |
| 安全 | 供应链 | 镜像签名 + SBOM + 依赖版本锁 |
| 合规 | 数据主权 | 所有用户数据和基础设施状态存储在中国大陆 Region |
| 合规 | 等保合规 | 平台自身满足等保 2.0 三级要求 |
| 可扩展 | Adapter | 添加新云厂商 Adapter 无需修改核心代码（插件化架构） |
| 可扩展 | 资源类型 | 添加新资源类型仅需扩展 ResourceSpec + Adapter 映射 |
| 幂等 | 资源状态 | `(env_id, resource_name)` 唯一键 + `spec_hash` 变更检测 + `state_version` 乐观锁 |

---

## 12. Technology Stack 技术选型

| 领域 | 技术选择 | 选型理由 |
|------|---------|----------|
| 核心语言 | Go 1.22+ | 与 Encore 框架保持一致，性能优异，云生态库丰富 |
| 前端 | Next.js + TypeScript | 管理平台 Web UI（P3） |
| 数据库 | PostgreSQL 16 | 平台自身的元数据存储 |
| 缓存 | Redis 7 | Session、任务队列、实时状态 |
| 消息队列 | NATS JetStream | 内部事件驱动（部署事件、通知等） |
| 容器 | Docker + containerd | 应用镜像构建和运行时 |
| 编排 | ACK Serverless (P0) | 目标云上的工作负载编排，免运维 |
| 资源供应 | 各云 Go SDK Direct (P0) | 直接调用云厂商 API，错误处理更直接 |
| IaC | Terraform / OpenTofu (P1+) | 后续可选，作为 SDK Direct 的补充 |
| 可观测性 | OpenTelemetry Collector | Traces/Metrics 采集和导出 |
| 认证 | OIDC + JWT | 用户认证，支持企业 SSO 对接 |
| 加密 | AES-256-GCM + KMS | 云厂商凭证加密存储 |
| CLI 框架 | Cobra | 支持子命令、flag、help |
| 配置解析 | gopkg.in/yaml.v3 | infracast.yaml 解析 |

---

## 13. Risk Gates 风险门禁

以下条件未通过则不进入下一阶段：

1. **infracfg 与运行时兼容性验证未通过** → 停，返工 PoC
2. **Compute 单一路径（ACK Serverless）导致复杂度失控** → 停
3. **资源创建失败率持续高且缺少重试/补偿** → 停
4. **云账单责任边界/合规边界未明确** → 停
5. **资源状态表冲突（并发、重复创建、更新）未解决** → 停

---

## 14. Risks and Mitigations 风险与应对

| 风险 | 等级 | 影响 | 应对措施 |
|------|------|------|----------|
| Encore 上游破坏性变更 | 高 | Build-Time Patch 可能被上游重构破坏 | 锁定稳定版本；每日 CI 检测上游 HEAD 兼容性；Fallback 到浅 Fork + cherry-pick |
| 云厂商 API 变更/废弃 | 中 | 各厂商 API 向后兼容性不一 | Adapter 层隔离变更；维护 API 版本映射表 |
| Encore IR 不稳定 | 高 | IR 格式是内部实现，无公开 schema | P0 不依赖 IR 作为核心契约；先靠 infracast.yaml + 构建元信息 |
| 数据主权合规风险 | 高 | 平台自身的数据处理需合规 | 技术确保数据不出境；法律顾问审查；等保认证 |
| 人才招聘困难 | 中 | 同时熟悉 Encore + 国内云的人才稀缺 | 内部培训体系；Encore 社区招募；远程团队 |
| 云厂商竞争/复制 | 低 | 大厂可能自己做类似产品 | 先发优势 + 多云中立定位 + 社区绑定 + 示例应用库积累粘性 |

---

## 15. Success Metrics 成功指标

| 时间节点 | 用户指标 | 业务指标 |
|---------|---------|----------|
| MVP 后 3 个月 | 100+ 注册开发者 | 20+ 部署到阿里云的项目 |
| 上线后 6 个月 | 10+ 付费团队 | 3 个云厂商完整适配 |
| 上线后 12 个月 | 1,000+ 注册开发者 | 50+ 付费团队，3+ 云厂商合作 |
| 上线后 18 个月 | 5,000+ 注册开发者 | ARR > 500 万 RMB，企业版客户 5+ |

---

## Appendix A: 项目目录结构

```
infracast/
├── cmd/
│   └── infracast/           # CLI 入口
│       ├── main.go
│       └── commands/
│           ├── init.go
│           ├── run.go
│           └── deploy.go
├── internal/
│   ├── config/              # infracast.yaml 解析与校验
│   │   ├── config.go
│   │   └── validate.go
│   ├── mapper/              # Service Mapper
│   │   ├── mapper.go
│   │   ├── registry.go      # Provider Registry
│   │   └── spec.go          # ResourceSpec 定义
│   ├── state/               # 资源状态管理
│   │   ├── model.go
│   │   └── store.go         # 幂等 CRUD
│   ├── deploy/              # Deploy Pipeline
│   │   ├── pipeline.go
│   │   ├── docker.go
│   │   └── rollback.go
│   └── observe/             # Observability Bridge
│       ├── bridge.go
│       └── exporters/
├── providers/               # Cloud Provider Adapters
│   ├── interface.go         # CloudProviderInterface
│   └── alicloud/
│       ├── adapter.go
│       ├── database.go      # RDS
│       ├── cache.go         # Redis
│       ├── storage.go       # OSS
│       ├── compute.go       # ACK Serverless
│       └── adapter_test.go
├── pkg/
│   └── infragen/            # infracfg.json 生成器
├── examples/                # 示例应用
├── docs/                    # 开发者文档
├── infracast.yaml.example   # 配置文件模板
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Appendix B: CLI 命令参考

```bash
# 初始化项目
infracast init --provider alicloud --region cn-hangzhou

# 本地开发
infracast run                          # 启动本地开发环境
infracast run --verbose                # 详细日志输出

# 部署
infracast deploy                       # 部署到默认环境
infracast deploy --env production      # 部署到指定环境

# 环境管理
infracast env list                     # 列出所有环境
infracast env create staging           # 创建新环境
infracast env destroy preview-42       # 销毁 Preview 环境

# 基础设施
infracast infra plan                   # 预览基础设施变更
infracast infra apply                  # 应用基础设施变更
infracast infra status                 # 查看当前基础设施状态

# 日志和追踪
infracast logs --env production        # 查看生产日志
infracast traces --env production      # 查看追踪数据

# 凭证管理
infracast credentials add alicloud     # 添加云厂商凭证
infracast credentials verify           # 验证凭证有效性

# 版本信息
infracast version                      # 显示版本信息
```

---

*— End of Document —*

*Infracast PRD v1.1 (Frozen) | Confidential*
