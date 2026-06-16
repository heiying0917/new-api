#!/usr/bin/env bash
# Tokenki 集群初始化部署：把整套 K8s 资源部署到 cls-tokensolo-tke-japan / ns tokenki
#
# 用法：
#   bash bootstrap.sh [--dry-run]
#     --dry-run / -n : 只打印将执行的写操作，不修改集群
#
# 前提（脚本外人工准备）：
#   - kubectl 已指向集群 cls-tokensolo-tke-japan（脚本会打印 context 让你二次确认）
#   - 腾讯云 PG 已执行 scripts/create-pg-db.sql 建库建用户
#   - 腾讯云 Redis 实例 db=2 可用
#   - manifests/ghcr-secret.yaml 已生成（kubectl create secret docker-registry，见 ghcr-secret.example.yaml）
#   - manifests/prod/tokenki/secret.yaml 已填好 PG/Redis 凭证 + SESSION/CRYPTO secret
#   - tokenki.com / www.tokenki.com DNS TTL 已调低（部署完后再切 A 记录到 CLB IP）
#
# 只负责 K8s 层。集群、CLB、ingress-nginx、cert-manager、ClusterIssuer 都已由 tokensolo 部署完毕。

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
M="$(cd "$SCRIPT_DIR/../manifests" && pwd)"
NS=tokenki
EXPECTED_CONTEXT_KEYWORD="tokensolo"  # 集群 context 应包含此关键字（cls-tokensolo-tke-japan）

DRY=0
for a in "$@"; do
  case "$a" in
    --dry-run|-n) DRY=1 ;;
    *) echo "用法: bash bootstrap.sh [--dry-run]"; exit 1 ;;
  esac
done

run() { if [ "$DRY" = 1 ]; then echo "  [dry-run] $*"; else "$@"; fi; }

[ "$DRY" = 1 ] && echo "*** DRY-RUN：以下写操作只打印不执行 ***"

# 1. 集群 context 校验
CTX=$(kubectl config current-context)
echo "目标集群 context : $CTX"
echo "目标 namespace   : $NS"
if [[ "$CTX" != *"$EXPECTED_CONTEXT_KEYWORD"* ]]; then
  echo "⚠️  当前 context 不含 '$EXPECTED_CONTEXT_KEYWORD'，可能不是预期集群"
fi
if [ "$DRY" != 1 ]; then
  read -r -p "确认部署到此集群? (yes/no): " ans
  [ "$ans" = "yes" ] || { echo "已取消"; exit 1; }
fi

# 2. 前置检查：实际 secret 文件必须已就位（git-ignored，新集群不自带）
for s in "$M/ghcr-secret.yaml" "$M/prod/tokenki/secret.yaml"; do
  [ -f "$s" ] || { echo "✗ 缺少 ${s}（git-ignored，需先填好再部署）"; exit 1; }
done

# 3. 集群级组件（仅校验，不动）
echo "==> 校验集群级依赖（应由 tokensolo 已部署）"
kubectl get ds ingress-nginx-controller -n ingress-nginx >/dev/null 2>&1 \
  || { echo "✗ 集群未装 ingress-nginx，请先在 tokensolo 项目跑 install-ingress.sh"; exit 1; }
kubectl get clusterissuer letsencrypt-prod >/dev/null 2>&1 \
  || { echo "✗ 集群未装 letsencrypt-prod ClusterIssuer"; exit 1; }
echo "    ingress-nginx + letsencrypt-prod 已就绪"

# 4. namespace + 镜像凭证
echo "==> 创建 namespace + ghcr-secret"
run kubectl apply -f "$M/namespace.yaml"
run kubectl apply -f "$M/ghcr-secret.yaml"

# 5. 部署应用
echo "==> 部署 tokenki-api（master×1 + slave×2）"
run kubectl apply -f "$M/prod/tokenki/configmap.yaml"
run kubectl apply -f "$M/prod/tokenki/secret.yaml"
run kubectl apply -f "$M/prod/tokenki/service.yaml"
run kubectl apply -f "$M/prod/tokenki/deployment-master.yaml"
run kubectl apply -f "$M/prod/tokenki/deployment-slave.yaml"

# 6. 等就绪（首次镜像 tag 必须是真实存在的 tag，不是 placeholder——bootstrap 前需要先 set image）
if [ "$DRY" != 1 ]; then
  echo "==> 等待 deployment 就绪（5 分钟）"
  for d in tokenki-api-master tokenki-api-slave; do
    kubectl rollout status deploy/"$d" -n "$NS" --timeout=300s
  done
fi

# 7. ingress
echo "==> 部署 ingress（tokenki.com / www.tokenki.com）"
run kubectl apply -f "$M/prod/ingress/ingress-all.yaml"

# 8. 收尾
if [ "$DRY" = 1 ]; then
  echo ""
  echo "*** DRY-RUN 结束 ***"
  exit 0
fi
echo ""
echo "✓ 部署完成。CLB 入口 IP："
kubectl get svc -n ingress-nginx ingress-nginx-controller \
  -o jsonpath='{.status.loadBalancer.ingress[0].ip}{"\n"}'
echo ""
echo "后续："
echo "  1. 在 DNS 厂商（Cloudflare/Godaddy/...）把 tokenki.com / www.tokenki.com 的 A 记录指到上方 CLB IP"
echo "  2. cert-manager 会自动签 letsencrypt 证书（首次 1-2 分钟）"
echo "  3. 验证：curl -fsS https://tokenki.com/api/status"
