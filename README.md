# gitops-vault

> GitOps 仓库密钥脱敏工具 — 把明文密钥锁进保险箱，放心提交到 Git。

## 解决什么问题？

你的 GitOps 仓库里是不是到处散落着数据库密码、API Token、证书私钥？`.gitignore` + `sops` 太繁琐，Base64 编码更是自欺欺人。

**gitops-vault** 一键扫描 YAML 文件中的所有敏感值，用 `age` 加密后替换为占位符，让你可以**安全地把整个仓库提交到 Git**。解密只需要一把密钥。

```
# 加密前                                   # 加密后
apiVersion: v1                            apiVersion: v1
kind: Secret                               kind: Secret
data:                                      data:
  password: my-secret-password              password: VAULT_DATA_PASSWORD_1719000000
```

## 快速开始

### 安装

```bash
# macOS / Linux
brew install zzg/tap/gitops-vault

# 或直接下载二进制
# https://github.com/zzg/gitops-vault/releases
```

### 初始化（推荐）

```bash
# 一键初始化：生成密钥对 + 创建配置文件
gitops-vault init --generate-key
```

执行后会自动生成：
- `.gitops-vault.yml` — 项目配置（可提交到 Git）
- `~/.age/vault.pub` — 公钥（可提交到 Git）
- `~/.age/key.txt` — 私钥（**绝不能提交！**）

### 加密

```bash
# 无需任何参数，自动读取配置中的公钥
gitops-vault encrypt ./manifests

# 试运行：只看看有哪些敏感值，不做修改
gitops-vault scan ./manifests
```

`.vault/` 目录下会保存加密后的密文，可以安全提交到 Git。

### 解密（部署时）

```bash
# 设置私钥环境变量
export AGE_KEY="$(cat ~/.age/key.txt)"

# 解密还原
gitops-vault decrypt ./manifests
```

解密只需设置 `AGE_KEY` 环境变量即可，无需其他配置。

## 原理

```
明文 YAML ──[扫描敏感值]──> 提取明文 ──[age 加密]──> .vault/ 密文存储
     │
     └──[替换为占位符]──> 安全 YAML（可提交 Git）
```

- **扫描引擎**：内置常见敏感字段匹配（password, token, secret, key, apikey 等），支持正则自定义
- **加密算法**：[age](https://age-encryption.org/)（filippo.io/age），现代、简洁、替代 GPG
- **占位符格式**：`VAULT_KEYNAME_TIMESTAMP`，防止被误匹配，方便定位

## 命令一览

| 命令 | 作用 |
|------|------|
| `gitops-vault init` | 一键初始化：生成密钥 + 配置文件 |
| `gitops-vault encrypt [path]` | 加密敏感值，原地替换为占位符 |
| `gitops-vault decrypt [path]` | 将占位符恢复为原始值 |
| `gitops-vault scan [path]` | 只报告，不做修改 |
| `gitops-vault encrypt-value --key NAME VAL` | 加密单个值，输出占位符（手动粘贴到 YAML） |

### 手动加密单个值

不想全自动扫描？手动加密一个值，拿到占位符后自己粘贴到 YAML：

```bash
$ gitops-vault encrypt-value --key db_password "my-secret-password"
VAULT_DB_PASSWORD_1750000000

# 然后手动编辑 YAML：
# password: VAULT_DB_PASSWORD_1750000000
```

## 配置

### .gitops-vault.yml

```yaml
# 公钥（值或文件路径），init --generate-key 会自动填充
public_key: "age1..."

# 私钥（值或文件路径），仅解密时需要
# 注意：不要将此文件提交到 Git！
private_key: "AGE-SECRET-KEY-1..."

# 密文存储目录，默认 .vault
secret_dir: ".vault"

# 额外的敏感字段名（内置默认已覆盖常见字段）
sensitive_keys:
  - db_url
  - connection_string

# 跳过扫描的目录
exclude:
  - "vendor/"
  - "node_modules/"
```

配置文件从当前目录向上查找（类似 `.gitignore` 的发现机制），命令行参数优先级高于配置文件。

## 为什么选择 gitops-vault？

- **一键初始化**：`gitops-vault init --generate-key` 三秒搞定全部配置
- **安全可靠**：age 加密，密钥永不出现在明文中
- **Git 友好**：加密后的文件 diff 清晰可读
- **CI/CD 就绪**：解密只需一行命令，集成到部署流程只需 3 分钟

## License

MIT
