# 发布说明 — Infracast vX.Y.Z

> **Date:** YYYY-MM-DD
> **Commit:** `<short-sha>`
> **Platforms:** darwin/amd64, darwin/arm64, linux/amd64

---

## 新特性

- [ ] 功能或改进（一句话摘要）
- [ ] 功能或改进（一句话摘要）

## 问题修复

- [ ] 修复说明（如适用，请引用错误码，例如 EDEPLOY050）

## 破坏性变更

- 无 / 在此列出任何破坏性变更

## 兼容性

| 组件 | 最低版本 |
|-----------|----------------|
| Go | 1.22+ |
| Encore CLI | latest |
| Docker | 20.10+ |
| kubectl | 1.28+ |
| Alicloud Provider | 已在 cn-hangzhou 测试 |

## 已知环境前置条件

这些是基础设施前置条件，而非代码缺陷：

- **ACK 集群**：部署操作需要一个运行中的 Alibaba Container Service（ACK）集群。如果集群未预配或账户余额不足，部署将失败并返回 `EPROV003` 或 `NotEnoughBalance`。
- **KUBECONFIG**：必须已设置并指向有效的 ACK 集群。缺少配置将产生 `EDEPLOY011`。
- **ACR 凭证**：必须配置容器镜像仓库认证才能推送镜像。认证失败将产生 `EDEPLOY072`。
- **计费**：按量付费资源（RDS、Redis、ACK 节点）需要充足的账户余额。

## 安装方式

### 下载二进制文件

```bash
# Example for darwin/arm64 (Apple Silicon)
curl -LO https://github.com/DaviRain-Su/Infracast/releases/download/vX.Y.Z/infracast_vX.Y.Z_darwin_arm64.tar.gz
curl -LO https://github.com/DaviRain-Su/Infracast/releases/download/vX.Y.Z/checksums.txt

# Verify checksum
shasum -a 256 -c checksums.txt --ignore-missing

# Extract and install
tar xzf infracast_vX.Y.Z_darwin_arm64.tar.gz
sudo mv infracast_vX.Y.Z_darwin_arm64/infracast /usr/local/bin/
```

### 验证

```bash
infracast version
# Expected:
# infracast version vX.Y.Z
#   commit: <short-sha>
#   build time: YYYY-MM-DD_HH:MM:SS
```

### 从源码构建

```bash
git clone https://github.com/DaviRain-Su/Infracast.git
cd Infracast
make build
./bin/infracast version
```

## 回滚

如果此版本引发问题：

1. 从 [Releases](https://github.com/DaviRain-Su/Infracast/releases) 下载上一个版本
2. 替换二进制文件：`sudo mv infracast /usr/local/bin/`
3. 验证：`infracast version` 显示上一个版本
4. 状态数据库（`.infra/state.db`）向前兼容；数据无需回滚

## 完整变更日志

`git log <prev-tag>..vX.Y.Z --oneline`
