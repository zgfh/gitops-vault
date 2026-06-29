# ArgoCD 集成 gitops-vault

通过 ArgoCD 的 [Config Management Plugin v2（Sidecar）](https://argo-cd.readthedocs.io/en/stable/operator-manual/config-management-plugins/) 机制，在 ArgoCD 拉取仓库后、应用资源前，自动解密 YAML 中的占位符。

## 工作原理

```
Git 仓库                          ArgoCD Repo Server
┌─────────────┐                   ┌──────────────────────────┐
│ .vault/     │─── git pull ───▶  │  CMP Sidecar 容器         │
│   *.age     │                   │  ┌────────────────────┐  │
│ apps/       │                   │  │ 1. 定位 app 源码路径 │  │
│   app1/     │                   │  │ 2. /gitops-vault    │  │
│     secret.yaml                 │  │    decrypt -w .     │  │
│     (含占位符)  │                   │  │    -d ../../.vault  │  │
└─────────────┘                   │  │ 3. cat *.yaml       │  │
                                  │  └────────────────────┘  │
                                  │           │               │
                                  │           ▼               │
                                  │  解密后的 manifests ─────▶│
                                  │  传给 Application Controller
                                  └──────────────────────────┘
```

Sidecar 容器运行 ArgoCD 的 CMP Server 二进制，通过 gRPC 与主 repo-server 通信，共享 `/tmp` 目录获取仓库文件。

## 前置条件

- **AGE 私钥**：用于解密 `.vault/` 中的密文
- **gitops-vault 镜像**：包含 `/gitops-vault` 二进制（Dockerfile 基于 `alpine`）
- **ArgoCD >= 2.6**：支持 CMP v2 Sidecar 模式

## 部署

### 1. 创建 AGE 私钥 Secret

```bash
kubectl -n argocd create secret generic gitops-vault-age-key \
  --from-literal=age-key="$(cat ~/.age/key.txt)"
```

### 2. 创建 Plugin ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gitops-vault-plugin
  namespace: argocd
data:
  plugin.yaml: |
    apiVersion: argoproj.io/v1alpha1
    kind: ConfigManagementPlugin
    metadata:
      name: gitops-vault
    spec:
      version: v1.0
      generate:
        command: [sh, -c]
        args:
          - |
            set -e
            APP_PATH="${ARGOCD_APP_SOURCE_PATH:-.}"
            if [ "$APP_PATH" = "." ]; then
              VAULT_DIR=".vault"
            else
              DEPTH=$(echo "$APP_PATH" | tr -cd '/' | wc -c | tr -d ' ')
              VAULT_DIR=$(printf '../%.0s' $(seq 1 $((DEPTH + 1)))).vault
            fi
            /gitops-vault decrypt -w . -d "$VAULT_DIR" >&2
            cat *.yaml 2>/dev/null || true
```

**关键说明：**

| 要点 | 说明 |
|------|------|
| `set -e` | 确保 `decrypt` 失败时立即报错，而非静默跳过 |
| VAULT_DIR 自动定位 | CMP 工作目录为 `<workspace>/<app-source-path>`，`.vault/` 在仓库根。通过计算 `ARGOCD_APP_SOURCE_PATH` 的层级深度，自动生成 `../../.vault` 等相对路径 |
| `decrypt -w .` | 原地修改文件，ArgoCD 通过 stdout 读取 manifest |
| `-d "$VAULT_DIR"` | 指定密文目录，指向仓库根目录 |
| `/gitops-vault` | Docker 镜像将二进制放在 `/`，不在 `PATH` 中，须用绝对路径 |

### 3. 配置 argocd-repo-server Sidecar

在 `argocd-repo-server` Deployment 中添加 sidecar 容器和共享卷：

```yaml
spec:
  template:
    spec:
      # === 共享卷 ===
      volumes:
        - name: var-files
          emptyDir: {}
        - name: plugins
          emptyDir: {}
        - name: cmp-plugin-config
          configMap:
            name: gitops-vault-plugin

      # === Init 容器：拷贝 CMP Server 二进制 ===
      initContainers:
        - name: copyutil
          image: quay.io/argoproj/argocd:v2.14.2
          command: [/bin/cp, -n, /usr/local/bin/argocd, /var/run/argocd/argocd-cmp-server]
          securityContext:
            runAsNonRoot: true
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
            readOnlyRootFilesystem: true
            seccompProfile:
              type: RuntimeDefault
          volumeMounts:
            - mountPath: /var/run/argocd
              name: var-files

      containers:
        # === 主容器额外挂载 ===
        - name: argocd-repo-server
          volumeMounts:
            - mountPath: /tmp
              name: tmp          # 与 sidecar 共享临时目录
            - mountPath: /home/argocd/cmp-server/plugins
              name: plugins

        # === gitops-vault Sidecar ===
        - name: cmp-gitops-vault
          image: ghcr.io/zgfh/gitops-vault:latest
          command: [/var/run/argocd/argocd-cmp-server]
          securityContext:
            runAsNonRoot: true
            runAsUser: 999
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
            readOnlyRootFilesystem: true
            seccompProfile:
              type: RuntimeDefault
          env:
            - name: AGE_KEY
              valueFrom:
                secretKeyRef:
                  name: gitops-vault-age-key
                  key: age-key
          volumeMounts:
            - name: var-files
              mountPath: /var/run/argocd
            - name: plugins
              mountPath: /home/argocd/cmp-server/plugins
            - name: cmp-plugin-config
              mountPath: /home/argocd/cmp-server/config/plugin.yaml
              subPath: plugin.yaml
            - name: tmp
              mountPath: /tmp
```

**数据流：**

1. Init 容器将 ArgoCD 二进制拷贝到共享卷 `var-files`，作为 CMP Server
2. Sidecar 启动 CMP Server，读取 `plugin.yaml` 注册插件，通过 `plugins` 目录的 Unix socket 向主容器暴露
3. 主容器将仓库文件写入共享的 `tmp`，通过 gRPC 调用 Sidecar
4. Sidecar 在仓库副本的临时目录中执行插件脚本，解密后的 YAML 输出到 stdout

### 4. 创建使用插件的 Application

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
spec:
  source:
    repoURL: https://github.com/zgfh/gitops.git
    path: apps/my-app
    plugin:
      name: gitops-vault-v1.0
```

`plugin.name` 格式为 `<metadata.name>-v<spec.version>`，即 `gitops-vault-v1.0`。

### 5. 目录结构约定

```
gitops-repo/
├── .vault/                   # 密文存储（必须在仓库根目录）
│   └── *.age
├── apps/
│   ├── app1/
│   │   └── secret.yaml       # 含占位符 VAULT_XXX_1234567890
│   └── app2/
│       └── deploy.yaml
```

- `.vault/` **必须**在仓库根目录
- 每个 Application 的 `path` 可以是仓库下任意子目录
- 插件自动计算从 app 目录到仓库根的相对路径

## 排查

```bash
# 查看 Sidecar 日志（含 decrypt stderr）
kubectl -n argocd logs deploy/argocd-repo-server -c cmp-gitops-vault --tail=50

# 正常解密输出示例
# stderr="  stringData.DB_PASS = VAULT_XXX -> ***\nsecret.yaml: 1 value(s) decrypted\n"

# 强制刷新（绕过缓存）
argocd app diff my-app --hard-refresh

# 清理缓存
kubectl -n argocd rollout restart deploy/argocd-repo-server
kubectl -n argocd delete pod -l app.kubernetes.io/name=argocd-application-controller
```

### 常见错误

| 错误信息 | 原因 | 解决 |
|----------|------|------|
| `exec: "sh": executable file not found` | 镜像无 shell | 使用 alpine 基础镜像 |
| `gitops-vault: not found` | 二进制不在 PATH | 使用绝对路径 `/gitops-vault` |
| `unknown flag: --force` | 新版去掉了此 flag | 去掉 `--force` 参数 |
| `vault entry not found: VAULT_XXX` | 密文目录路径不对，找不到 `.vault/` | 检查 `-d` 参数是否正确指向仓库根目录 |
| `Path "apps/my-app" does not exist` | CMP 已 cd 到 app 目录，脚本又 cd 了一次导致路径重复 | 不要 `cd $APP_PATH`，CWD 已经是正确目录 |
| `(cached)` 持续出现 | 旧错误被 Redis/内存缓存 | 重启 repo-server 和 application-controller |
| `CheckPluginConfiguration failed` | ConfigMap 未挂载或格式错误 | 检查 `plugin.yaml` YAML 格式，确认 `subPath: plugin.yaml` |
