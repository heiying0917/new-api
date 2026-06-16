# Tokenki TKE 部署

> 本目录存放 Tokenki 部署到腾讯云 TKE 集群（`cls-tokensolo-tke-japan`）的全部产物。

## 架构概览

```
         Internet
            │
            ▼
   腾讯云 CLB (lb-5j4mtrgo) ───┐
            │                   │ 共用
            ▼                   ▼
   ingress-nginx-controller (集群级，tokensolo 部署)
            │
            ├── tokenki.com / www.tokenki.com  ──► tokenki-api (Service, ns=tokenki)
            ├── tokensolo.com / www...         ──► tokensolo-api (ns=tokensolo)
            └── admincrs.tokensolo.com         ──► tokensolo-crs (ns=tokensolo)

   ┌─────────────────────────────────┐
   │ Tokenki Pods (ns: tokenki)      │
   │  ├── tokenki-api-master (×1)    │  → 后台任务：批量更新、统计、定时同步
   │  └── tokenki-api-slave  (×2)    │  → API 流量（Service selector 只匹配 slave）
   └─────────────────────────────────┘
            │                    │
            ▼                    ▼
   腾讯云 PG (实例 = tokensolo)   腾讯云 Redis (实例 = tokensolo)
     database: tokenki              db: 2 (db=0 给 crs, db=1 给 tokensolo)
     user:     tokenki_app
```

## 目录结构

```
deploy/
├── README.md                          # 本文档
└── k8s/
    ├── manifests/
    │   ├── namespace.yaml
    │   ├── ghcr-secret.example.yaml   # 镜像拉取凭证模板（真实文件 git-ignored）
    │   └── prod/
    │       ├── tokenki/
    │       │   ├── configmap.yaml         # 运行参数（DB/Redis 连接池、超时等）
    │       │   ├── secret.example.yaml    # 敏感配置模板（真实文件 git-ignored）
    │       │   ├── deployment-master.yaml # ×1，后台任务节点
    │       │   ├── deployment-slave.yaml  # ×2，API 流量节点
    │       │   └── service.yaml           # ClusterIP，selector 只匹配 slave
    │       └── ingress/
    │           └── ingress-all.yaml       # tokenki.com + www.tokenki.com
    └── scripts/
        ├── bootstrap.sh             # 一键部署到当前 kubectl context
        └── create-pg-db.sql         # PG 建库建用户脚本
```

## 首次部署（人工 + 脚本）

### 1. 云资源准备（人工）

- **PG**：腾讯云控制台连到 tokensolo 用的 PG 实例 → 用 root 账号跑 `scripts/create-pg-db.sql`，建库建用户
- **Redis**：确认实例 `db=2` 未被占用（db=0 = crs，db=1 = tokensolo）
- **CLS 日志主题**：在腾讯云 CLS 控制台 `ap-tokyo` 新建主题 `tke_tokenki`，绑定 TKE 集群日志采集规则（采 `ns=tokenki` 容器 stdout）。记下 TopicId，后续填进 `.claude/skills/tke-release/SKILL.md`。
- **GHCR**：在 GitHub repo `heiying0917/new-api` 的 Settings → Actions 确认 `GITHUB_TOKEN` 有 `packages: write` 权限（默认开启即可）
- **kubeconfig**：复用 tokensolo 的 `~/.kube/tokensolo.config`

### 2. 生成 secret 实文件（本地，不提交）

```bash
cd deploy/k8s/manifests

# 镜像拉取凭证（PAT 需 read:packages 权限）
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=heiying0917 \
  --docker-password=<GHCR_PAT> \
  --docker-email=xyang0917@gmail.com \
  -n tokenki \
  --dry-run=client -o yaml > ghcr-secret.yaml

# 应用敏感配置
cp prod/tokenki/secret.example.yaml prod/tokenki/secret.yaml
# 编辑 secret.yaml，填入：
#   - SQL_DSN（PG_USER/PG_PASS/PG_HOST + database=tokenki）
#   - REDIS_CONN_STRING（REDIS_PASS/REDIS_HOST + /2）
#   - SESSION_SECRET（openssl rand -hex 32）
#   - CRYPTO_SECRET（openssl rand -hex 32）
```

### 3. 触发首次镜像构建

```bash
git tag v2026.06.16.1
git push origin v2026.06.16.1
# GitHub Actions 跑 .github/workflows/ghcr-publish.yml
# 完成后镜像在 ghcr.io/heiying0917/tokenki:v2026.06.16.1
```

### 4. 替换 deployment 中的 placeholder 镜像 tag

```bash
sed -i '' 's/tokenki:placeholder/tokenki:v2026.06.16.1/g' \
  deploy/k8s/manifests/prod/tokenki/deployment-master.yaml \
  deploy/k8s/manifests/prod/tokenki/deployment-slave.yaml
```

### 5. 跑 bootstrap

```bash
export KUBECONFIG=~/.kube/tokensolo.config
bash deploy/k8s/scripts/bootstrap.sh --dry-run    # 先 dry-run 看一遍
bash deploy/k8s/scripts/bootstrap.sh              # 真正部署
```

脚本输出末尾会打印 CLB 公网 IP。

### 6. DNS 切换

在 DNS 厂商面板（Cloudflare/Godaddy/…）把 `tokenki.com` 和 `www.tokenki.com` 的 A 记录指到 CLB IP。
cert-manager 会自动签 letsencrypt 证书（1-2 分钟）。

### 7. 验证

```bash
curl -fsS https://tokenki.com/api/status      # 200，返回 system_name=Tokenki
kubectl get pods -n tokenki                   # master 1/1，slave 2/2
kubectl logs -n tokenki deploy/tokenki-api-master --tail=20
```

## 后续发版

主路径走 Claude Code 本地 `/tke-release` skill（含 6 Phase 流程 + 冒烟测试 + 监控）。

紧急备用走 `.github/workflows/tke-manual-release.yml` 手动触发（无冒烟）。

详见 `.claude/skills/tke-release/SKILL.md`。

## 与 tokensolo 共存的注意事项

| 资源 | 共用方式 | 风险 / 缓解 |
|---|---|---|
| TKE 集群 | 同集群，不同 namespace | kubectl 命令**必须**带 `-n tokenki`，禁止 `--all-namespaces` |
| CLB / ingress-nginx | 同 CLB，同 controller | 流量爆发需监控 CLB 带宽 |
| ClusterIssuer letsencrypt-prod | 共用 | tokenki 加几个域名不会触阈值；不要频繁 delete-recreate cert |
| PG 实例 | 同实例，独立 database + user | 监控 PG 连接数 / 慢日志 |
| Redis 实例 | 同实例，独立 db | tokenki=db2、tokensolo=db1、crs=db0；PoolSize 100 限制单服务上限 |
| KUBECONFIG | 同一份（tokensolo.config） | 误操作风险——所有命令带 `-n tokenki` |
