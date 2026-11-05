# Sub2API Fork macOS 完整部署文档

本文档只覆盖 **macOS 本机部署**。

当前部署形态：

```text
macOS
  ├─ Docker CLI（Homebrew）
  ├─ Colima Linux VM
  ├─ Docker Compose
  └─ Sub2API 容器
```

镜像完全保留在本机 Colima 的 Docker 存储中，**不需要 GHCR、Docker Hub 或其他镜像仓库**。

---

## 1. 总体架构

```text
源码目录
  /Users/chtholly/sub2api
        │
        │ docker buildx build --load
        ▼
本地 Docker 镜像
  sub2api:custom
  sub2api:custom-<短SHA>
        │
        │ docker compose up
        ▼
部署目录
  /Users/chtholly/sub2api-deploy
  ├─ docker-compose.local.yml
  ├─ docker-compose.override.yml
  ├─ .env
  ├─ data/
  ├─ postgres_data/
  └─ redis_data/
```

本地镜像的生命周期：

- 构建后保存在 Colima VM 的 Docker 存储里；
- `colima stop` / `colima start` 不会丢失；
- `colima delete` 会丢失镜像；
- `docker image prune -a` / `docker system prune -a` 可能删除镜像；
- 重要镜像可以用 `docker save` 导出成 tar 文件。

---

## 2. 环境检查

### 2.1 检查 Colima

```bash
colima status
colima list
```

当前环境应类似：

```text
PROFILE    STATUS     ARCH       CPUS    MEMORY    DISK
default    Running    aarch64    2       2GiB      100GiB
```

Sub2API 前端构建需要较多内存，建议至少调整为：

```text
CPUS: 4
MEMORY: 8GiB
```

修改方法：

```bash
colima stop
colima start --cpu 4 --memory 8
```

确认：

```bash
colima list
docker info --format 'MemTotal={{.MemTotal}} NCPU={{.NCPU}}'
```

### 2.2 检查 Docker

```bash
docker version
docker compose version
```

Docker daemon 应该连接 Colima：

```bash
docker context ls
```

应该看到：

```text
colima *   colima   unix:///Users/chtholly/.colima/default/docker.sock
```

### 2.3 检查 buildx

当前 Homebrew 安装的 Docker CLI 可能没有 buildx 插件：

```bash
docker buildx version
```

如果提示：

```text
docker: unknown command: docker buildx
```

安装 buildx：

```bash
mkdir -p ~/.docker/cli-plugins

curl -fL \
  https://github.com/docker/buildx/releases/download/v0.38.2/buildx-v0.38.2.darwin-arm64 \
  -o ~/.docker/cli-plugins/docker-buildx

chmod +x ~/.docker/cli-plugins/docker-buildx

docker buildx version
```

如果权限不足：

```bash
sudo chown -R "$(id -u):$(id -g)" ~/.docker
```

---

## 3. 部署目录

创建独立的部署目录：

```bash
mkdir -p ~/sub2api-deploy
cd ~/sub2api-deploy
```

从源码仓库复制 compose 文件：

```bash
cp /Users/chtholly/sub2api/deploy/docker-compose.local.yml .
cp /Users/chtholly/sub2api/deploy/.env.example .env
chmod 600 .env
mkdir -p data postgres_data redis_data
```

目录结构：

```text
~/sub2api-deploy/
├── docker-compose.local.yml
├── docker-compose.override.yml
├── .env
├── data/
├── postgres_data/
└── redis_data/
```

---

## 4. 配置 `.env`

生成随机密钥：

```bash
openssl rand -hex 32
openssl rand -hex 32
openssl rand -hex 32
```

编辑 `.env`：

```bash
nano .env
```

至少配置：

```bash
POSTGRES_PASSWORD=替换成随机密码
JWT_SECRET=替换成随机密钥
TOTP_ENCRYPTION_KEY=替换成随机密钥

SERVER_PORT=8080

# 建议首次部署时直接设置管理员账号
ADMIN_EMAIL=你的管理员邮箱
ADMIN_PASSWORD=强密码

# 如果前面有 Caddy/Nginx，建议只监听本机
# BIND_HOST=127.0.0.1
```

注意：

- `.env` 不要提交到 Git；
- `.env` 保持 `600` 权限；
- 不要使用示例密码；
- 修改 `.env` 后需要重新创建容器。

如果没有设置管理员账号，可以在首次启动后从日志中查找：

```bash
docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  logs sub2api | grep "Generated admin"
```

---

## 5. 配置本地镜像

创建 `docker-compose.override.yml`：

```yaml
services:
  sub2api:
    image: sub2api:custom
    pull_policy: never
```

说明：

- `image: sub2api:custom` 使用本机镜像；
- `pull_policy: never` 禁止 Compose 尝试从远程仓库拉取；
- 不需要 GHCR；
- 不需要 Docker Hub。

---

## 6. 构建本地镜像

进入源码目录：

```bash
cd /Users/chtholly/sub2api
```

构建：

```bash
docker buildx build \
  --load \
  --platform linux/arm64 \
  --build-arg VERSION=custom-local \
  --build-arg COMMIT="$(git rev-parse --short HEAD)" \
  -t sub2api:custom \
  -t "sub2api:custom-$(git rev-parse --short HEAD)" \
  .
```

说明：

- `--platform linux/arm64`：适配 Apple Silicon；
- `--load`：把构建结果加载到本机 Docker 镜像库；
- `VERSION` 和 `COMMIT` 会写入应用版本信息；
- `sub2api:custom` 是部署标签；
- `sub2api:custom-<短SHA>` 用于回滚。

构建完成后确认：

```bash
docker image ls | grep sub2api
```

应该看到：

```text
sub2api:custom
sub2api:custom-a4dd19b
```

---

## 7. 首次启动

```bash
cd ~/sub2api-deploy

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  up -d --wait
```

第一次启动会：

1. 拉取 PostgreSQL 和 Redis 镜像；
2. 创建数据库和缓存容器；
3. 启动 Sub2API；
4. 自动执行数据库迁移。

访问：

```text
http://localhost:8080
```

健康检查：

```bash
curl -fsS http://localhost:8080/health
```

查看状态：

```bash
docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  ps
```

查看日志：

```bash
docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  logs -f sub2api
```

---

## 8. 更新镜像并重启

重新构建：

```bash
cd /Users/chtholly/sub2api

docker buildx build \
  --load \
  --platform linux/arm64 \
  --build-arg VERSION=custom-local \
  --build-arg COMMIT="$(git rev-parse --short HEAD)" \
  -t sub2api:custom \
  -t "sub2api:custom-$(git rev-parse --short HEAD)" \
  .
```

重新创建容器：

```bash
cd ~/sub2api-deploy

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  up -d --no-deps --force-recreate --wait sub2api
```

单实例 Compose 更新会有短暂中断：

- 旧容器停止；
- 新容器创建；
- 应用启动；
- 数据库迁移；
- 健康检查通过。

本机使用可以接受这段短暂中断。

---

## 9. 源码更新方式

如果使用 GitHub Actions 自动同步 `custom` 分支，本机部署拉取最新代码：

```bash
cd /Users/chtholly/sub2api

# 确认没有未提交改动
git status --short

git fetch origin
git switch custom
git reset --hard origin/custom
```

如果本机有未提交改动：

- 先提交；
- 先 stash；
- 或使用单独的部署 clone。

不要在有未提交改动时直接执行 `git reset --hard`。

然后重新执行第 8 节的构建和重启命令。

---

## 10. macOS 自动更新

建议使用一个**独立的部署源码目录**，避免自动更新覆盖开发中的代码。

例如：

```text
开发目录：/Users/chtholly/sub2api
部署源码：/Users/chtholly/sub2api-prod
部署配置：/Users/chtholly/sub2api-deploy
```

自动更新脚本只操作部署源码目录。

### 10.1 创建更新脚本

```bash
mkdir -p ~/bin

cat > ~/bin/sub2api-local-update <<'EOF'
#!/bin/bash
set -euo pipefail

export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin"

SOURCE_DIR="/Users/chtholly/sub2api-prod"
DEPLOY_DIR="/Users/chtholly/sub2api-deploy"
LOCK_DIR="/tmp/sub2api-local-update.lock"
LAST_SHA_FILE="$DEPLOY_DIR/.last-built-sha"

# 防止定时任务重叠执行
if ! mkdir "$LOCK_DIR" 2>/dev/null; then
  exit 0
fi
trap 'rmdir "$LOCK_DIR"' EXIT

cd "$SOURCE_DIR"

# 有未提交改动时跳过，避免覆盖本地工作
if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "source tree is dirty, skip update" >&2
  exit 1
fi

git fetch origin custom
git switch custom
git reset --hard origin/custom

NEW_SHA="$(git rev-parse HEAD)"

if [ -f "$LAST_SHA_FILE" ] && [ "$(cat "$LAST_SHA_FILE")" = "$NEW_SHA" ]; then
  exit 0
fi

docker buildx build \
  --load \
  --platform linux/arm64 \
  --build-arg VERSION=custom-local \
  --build-arg COMMIT="$NEW_SHA" \
  -t sub2api:custom \
  -t "sub2api:custom-$(git rev-parse --short HEAD)" \
  .

cd "$DEPLOY_DIR"

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  up -d --no-deps --force-recreate --wait sub2api

echo "$NEW_SHA" > "$LAST_SHA_FILE"
EOF

chmod +x ~/bin/sub2api-local-update
```

### 10.2 手动测试脚本

```bash
~/bin/sub2api-local-update
```

查看日志：

```bash
tail -f /tmp/sub2api-local-update.log
tail -f /tmp/sub2api-local-update.err.log
```

### 10.3 配置 launchd

```bash
cat > ~/Library/LaunchAgents/com.chthoilly.sub2api-update.plist <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.chthoilly.sub2api-update</string>
  <key>ProgramArguments</key>
  <array>
    <string>/Users/chtholly/bin/sub2api-local-update</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>StartInterval</key>
  <integer>1800</integer>
  <key>StandardOutPath</key>
  <string>/tmp/sub2api-local-update.log</string>
  <key>StandardErrorPath</key>
  <string>/tmp/sub2api-local-update.err.log</string>
</dict>
</plist>
EOF
```

加载：

```bash
launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/com.chthoilly.sub2api-update.plist 2>/dev/null || true
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.chthoilly.sub2api-update.plist
launchctl kickstart -k gui/$(id -u)/com.chthoilly.sub2api-update
```

注意：

- 执行自动更新前，Colima 必须已经启动；
- `colima stop` 期间服务会停止；
- 如果希望“新镜像构建成功才部署”，脚本已经按这个顺序执行；
- 如果 `git reset --hard origin/custom` 后没有新 SHA，不会重复构建。

---

## 11. 5 小时订阅限额

当前实现是**订阅分组级**的 5 小时限额。

### 11.1 设置

管理后台：

```text
分组管理 → 新建/编辑订阅分组 → 5 小时限额（USD）
```

语义：

```text
空 / 0：不限制
> 0：5 小时 USD 上限
```

### 11.2 行为

- 只对订阅模式分组生效；
- 每个用户独立计算自己的 5h usage；
- 5h 窗口从该用户首次请求开始；
- 满 5 小时后自动清零并重新开窗；
- 用户仍可正常创建 API Key；
- 用户仍可正常登录网页和查看用量；
- 5h 限额只在调用模型 API 时拦截。

### 11.3 查看和重置

用户可以查看：

- 订阅页面的 5h 进度；
- 顶栏订阅进度；
- API Key 用量页。

管理员可以：

- 在订阅列表中查看 5h usage；
- 在重置配额时同时重置 5h；
- 在批量重置中勾选 5h。

---

## 12. 数据备份

数据库备份：

```bash
cd ~/sub2api-deploy

docker compose \
  -f docker-compose.local.yml \
  exec -T postgres \
  pg_dump -U sub2api -d sub2api \
  | gzip > "backup-$(date +%Y%m%d-%H%M%S).sql.gz"
```

恢复：

```bash
gunzip -c backup-YYYYMMDD-HHMMSS.sql.gz | \
  docker compose \
    -f docker-compose.local.yml \
    exec -T postgres \
    psql -U sub2api -d sub2api
```

同时建议备份：

```text
~/sub2api-deploy/data
~/sub2api-deploy/postgres_data
~/sub2api-deploy/redis_data
```

---

## 13. 镜像备份和回滚

### 13.1 导出本地镜像

```bash
docker save sub2api:custom-<短SHA> | gzip > sub2api-custom-<短SHA>.tar.gz
```

恢复：

```bash
gunzip -c sub2api-custom-<短SHA>.tar.gz | docker load
```

### 13.2 回滚

修改 `docker-compose.override.yml`：

```yaml
services:
  sub2api:
    image: sub2api:custom-<上一个短SHA>
    pull_policy: never
```

然后：

```bash
cd ~/sub2api-deploy

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  up -d --no-deps --force-recreate --wait sub2api
```

如果新版本已经执行了不兼容的数据库迁移，仅回滚镜像可能不够，需要使用数据库备份恢复。

---

## 14. 常用命令

```bash
cd ~/sub2api-deploy

# 状态
docker compose -f docker-compose.local.yml -f docker-compose.override.yml ps

# 应用日志
docker compose -f docker-compose.local.yml -f docker-compose.override.yml logs -f sub2api

# PostgreSQL 日志
docker compose -f docker-compose.local.yml -f docker-compose.override.yml logs -f postgres

# Redis 日志
docker compose -f docker-compose.local.yml -f docker-compose.override.yml logs -f redis

# 重启应用
docker compose -f docker-compose.local.yml -f docker-compose.override.yml restart sub2api

# 健康检查
curl -fsS http://localhost:8080/health

# 查看镜像
docker image ls | grep sub2api

# 查看资源占用
docker stats
```

---

## 15. 故障排查

### 15.1 `docker: unknown command: docker buildx`

安装 buildx：

```bash
mkdir -p ~/.docker/cli-plugins

curl -fL \
  https://github.com/docker/buildx/releases/download/v0.38.2/buildx-v0.38.2.darwin-arm64 \
  -o ~/.docker/cli-plugins/docker-buildx

chmod +x ~/.docker/cli-plugins/docker-buildx

docker buildx version
```

### 15.2 `failed to parse platform : "" is an invalid OS component`

使用了 legacy builder，而不是 BuildKit/buildx。

不要执行：

```bash
docker build ...
```

改为：

```bash
docker buildx build --load --platform linux/arm64 ...
```

### 15.3 `JavaScript heap out of memory`

前端构建 OOM，通常是 Colima 只有 2 GiB 内存。

```bash
colima stop
colima start --cpu 4 --memory 8
```

确认：

```bash
colima list
docker info --format 'MemTotal={{.MemTotal}} NCPU={{.NCPU}}'
```

如果仍然不够，可以在 `Dockerfile` 的 `frontend-builder` 阶段增加：

```dockerfile
ARG NODE_OPTIONS=--max-old-space-size=4096
ENV NODE_OPTIONS=${NODE_OPTIONS}
```

然后构建时传入：

```bash
docker buildx build \
  --load \
  --platform linux/arm64 \
  --build-arg NODE_OPTIONS=--max-old-space-size=4096 \
  -t sub2api:custom \
  .
```

### 15.4 `exec format error`

镜像架构和机器架构不一致。

Apple Silicon 构建时必须使用：

```bash
--platform linux/arm64
```

### 15.5 容器启动失败

```bash
docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  logs --tail=200 sub2api
```

常见原因：

- `.env` 缺少必需变量；
- 端口 8080 被占用；
- PostgreSQL 密码不一致；
- 数据库迁移失败；
- 本地镜像没有真正构建成功。

### 15.6 5h 限额没有生效

确认：

1. 分组是 `subscription` 类型；
2. 用户有该分组的有效订阅；
3. 分组 `five_hour_limit_usd > 0`；
4. 运行的是最新本地镜像；
5. 数据库迁移 `250_subscription_five_hour_limit.sql` 已执行。

查看当前镜像：

```bash
docker image ls | grep sub2api
```

---

## 16. 安全检查清单

- [ ] `.env` 权限为 `600`
- [ ] 数据库密码、JWT 密钥、TOTP 密钥都是随机值
- [ ] 管理员密码是强密码
- [ ] 不需要远程访问时，端口只绑定 `127.0.0.1`
- [ ] 定期备份 PostgreSQL
- [ ] 定期导出重要镜像
- [ ] 不要执行 `docker system prune -a`
- [ ] 不要执行 `colima delete`，除非确认镜像可以重新构建且数据已经备份

---

## 17. 最短命令清单

```bash
# 检查 Colima
colima list
docker info --format 'MemTotal={{.MemTotal}} NCPU={{.NCPU}}'

# 检查 buildx
docker buildx version

# 构建本地镜像
cd /Users/chtholly/sub2api
docker buildx build \
  --load \
  --platform linux/arm64 \
  --build-arg VERSION=custom-local \
  --build-arg COMMIT="$(git rev-parse --short HEAD)" \
  -t sub2api:custom \
  -t "sub2api:custom-$(git rev-parse --short HEAD)" \
  .

# 启动
cd ~/sub2api-deploy
docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  up -d --wait

# 状态
docker compose -f docker-compose.local.yml -f docker-compose.override.yml ps

# 日志
docker compose -f docker-compose.local.yml -f docker-compose.override.yml logs -f sub2api

# 备份
docker compose -f docker-compose.local.yml exec -T postgres pg_dump -U sub2api -d sub2api | gzip > backup.sql.gz

# 更新
docker buildx build --load --platform linux/arm64 -t sub2api:custom .
docker compose -f docker-compose.local.yml -f docker-compose.override.yml up -d --no-deps --force-recreate --wait sub2api

# 健康检查
curl -fsS http://localhost:8080/health
```

---

## 18. 可选：通过 Cloudflare Tunnel 暴露到公网

可以只用 Cloudflare Tunnel，不需要 Nginx，也不需要路由器端口转发。

Cloudflare Tunnel 的流量方向是：

```text
公网用户
  ↓ HTTPS
Cloudflare Edge
  ↓ Tunnel
cloudflared（本机）
  ↓ HTTP
Sub2API 容器
```

### 18.1 cloudflared 运行在 macOS 宿主机

这是最简单的方式。

先在 `.env` 中限制 Sub2API 只监听本机：

```bash
BIND_HOST=127.0.0.1
SERVER_PORT=8080
```

然后启动 Sub2API：

```bash
cd ~/sub2api-deploy

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  up -d --wait
```

确认本机可以访问：

```bash
curl -fsS http://127.0.0.1:8080/health
```

此时 Cloudflare Tunnel 的 ingress 应指向：

```text
http://127.0.0.1:8080
```

不要写：

```text
http://sub2api:8080
```

原因：

- `sub2api` 是 Docker Compose 网络内部的服务名；
- macOS 宿主机默认不能解析这个 Docker DNS 名称；
- 宿主机应使用映射到本机的端口，即 `127.0.0.1:8080`。

cloudflared 配置示例：

```yaml
tunnel: <TUNNEL_UUID>
credentials-file: /Users/chtholly/.cloudflared/<TUNNEL_UUID>.json

ingress:
  - hostname: sub2api.example.com
    service: http://127.0.0.1:8080
  - service: http_status:404
```

启动：

```bash
cloudflared tunnel run <TUNNEL_NAME>
```

Cloudflare 会负责公网 HTTPS，不需要 Nginx 再做 TLS。

### 18.2 cloudflared 运行在 Docker Compose 中

如果 cloudflared 容器和 `sub2api` 容器在同一个 Compose 网络，那么可以使用 Docker 服务名：

```text
http://sub2api:8080
```

示例 `docker-compose.override.yml`：

```yaml
services:
  sub2api:
    image: sub2api:custom
    pull_policy: never

  cloudflared:
    image: cloudflare/cloudflared:latest
    restart: unless-stopped
    command: tunnel --no-autoupdate run --token ${CLOUDFLARE_TUNNEL_TOKEN}
    depends_on:
      - sub2api
```

在 Cloudflare Dashboard 中把 Public Hostname 的 Origin 设置为：

```text
http://sub2api:8080
```

条件是：

- cloudflared 和 sub2api 在同一个 Docker Compose project / network；
- 否则 `sub2api` 这个主机名无法解析。

如果 cloudflared 是单独启动的容器，可以使用：

```text
http://host.docker.internal:8080
```

### 18.3 是否需要暴露 8080 端口？

#### cloudflared 在宿主机上运行

需要本机端口映射，但建议只绑定本机：

```bash
BIND_HOST=127.0.0.1
```

这样 8080 不会暴露到局域网或公网，只有本机 cloudflared 可以访问。

#### cloudflared 和 sub2api 在同一个 Docker 网络

可以完全不把 8080 发布到宿主机；cloudflared 直接访问 Docker 网络内的 `sub2api:8080`。

如果使用当前 compose 文件，保留 `BIND_HOST=127.0.0.1` 也是最简单、最安全的折中。

### 18.4 Cloudflare 侧建议

- 给管理后台配置 Cloudflare Access，只允许你的邮箱或 IP 访问；
- 开启 WAF 和速率限制；
- 不要直接把 `/admin` 无保护暴露给公网；
- 长连接和流式响应需要关注 Cloudflare 的请求超时和套餐限制；
- 确保 `server.trusted_proxies` 或 `security.trust_forwarded_ip_for_api_key_acl` 的配置与代理链一致；
- 如果启用了兼容模式 `trust_forwarded_ip_for_api_key_acl=true`，必须保证源站不能绕过 Cloudflare 直接访问。

### 18.5 结论

```text
cloudflared 在 macOS 宿主机：
  Tunnel Origin = http://127.0.0.1:8080

cloudflared 在同一个 Docker Compose 网络：
  Tunnel Origin = http://sub2api:8080

cloudflared 是独立容器：
  Tunnel Origin = http://host.docker.internal:8080
```

Cloudflare Tunnel 已经承担 HTTPS 终止和公网入口，所以简单的 Sub2API 公网暴露不需要额外 Nginx。

### 18.6 更换宿主机映射端口

可以只修改 `.env` 中的宿主机端口：

```bash
BIND_HOST=127.0.0.1
SERVER_PORT=18080
```

修改后：

```bash
cd ~/sub2api-deploy

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  up -d --no-deps --force-recreate --wait sub2api
```

注意：

- 容器内部 Sub2API 仍然监听 `8080`；
- `SERVER_PORT` 只改变 Colima/macOS 宿主机映射出去的端口；
- 如果 cloudflared 在 macOS 宿主机上运行，Tunnel Origin 要同步改成：

```text
http://127.0.0.1:18080
```

- 如果 cloudflared 和 sub2api 在同一个 Docker 网络里，Tunnel Origin 仍然是：

```text
http://sub2api:8080
```

此时宿主机映射端口怎么改都不影响 cloudflared 容器访问 Sub2API。

### 18.7 访问范围对照

| 配置 | 本机访问 | 局域网访问 | 公网访问 |
| --- | --- | --- | --- |
| `BIND_HOST=127.0.0.1` | 可以 | 不可以 | 只能通过 Cloudflare Tunnel |
| `BIND_HOST=0.0.0.0` | 可以 | 可以 | 需要路由器端口转发才会开放 |
| cloudflared 与 sub2api 同 Docker 网络 | 容器内可访问 | 宿主机/LAN 不可直接访问 | 只能通过 Cloudflare Tunnel |

补充说明：

- `http://sub2api:8080` 只在 Docker 网络内部有效，宿主机和局域网都不能直接访问；
- Cloudflare Tunnel 不等于自动访问控制。域名公开后，默认是“知道域名的人都能访问”。
- 如果只允许你自己的账号访问，应在 Cloudflare 侧配置 Access 或 WAF 规则。
- 如果云主机/路由器没有开放 18080 到公网，那么外部只能通过 Tunnel 域名进入。

---

## 19. cloudflared 跑在 Docker 还是 macOS 宿主机

两种方式都可以，区别主要在 **网络、生命周期和运维复杂度**。

### 19.1 方式对比

| 项目 | cloudflared 在 Docker Compose | cloudflared 在 macOS 宿主机 |
| --- | --- | --- |
| 访问 Sub2API | `http://sub2api:8080` | `http://127.0.0.1:8080` |
| 是否需要宿主机端口映射 | 同一网络时不需要 | 需要，建议只绑定 `127.0.0.1` |
| 网络隔离 | 更好，只走 Docker 内部网络 | 通过宿主机 loopback 访问 |
| 生命周期 | 跟 compose 一起管理 | 独立于 Docker，通常由 launchd/brew services 管理 |
| Docker/Colima 重启时 | Tunnel 可能随容器重启 | Tunnel 不受 Docker 重启影响 |
| 配置和日志 | `docker compose logs cloudflared` | 由 launchd/系统日志管理 |
| 多服务复用 | 适合容器化服务 | 适合宿主机上多个非 Docker 服务 |
| 故障边界 | 少一个宿主机进程，但多一层 Docker 网络 | Docker 出问题时 tunnel 仍然存活，但返回 502 |

### 19.2 推荐选择

#### 推荐用 Docker 版

如果满足：

- Sub2API 和 cloudflared 都在 Docker；
- 两者在同一个 Compose 项目或同一个 Docker network；
- 只暴露这一套服务；
- 希望 `docker compose up` 一键启动全部组件。

配置：

```yaml
services:
  sub2api:
    image: sub2api:custom
    pull_policy: never

  cloudflared:
    image: cloudflare/cloudflared:latest
    restart: unless-stopped
    command: tunnel --no-autoupdate run --token ${CLOUDFLARE_TUNNEL_TOKEN}
    depends_on:
      - sub2api
```

Cloudflare 的 Origin 使用：

```text
http://sub2api:8080
```

这种方式可以把宿主机端口映射去掉：

```yaml
# 不需要时可以让它不发布到宿主机
# ports:
#   - "127.0.0.1:8080:8080"
```

如果暂时还需要本机浏览器直接访问后台，可以保留：

```yaml
ports:
  - "127.0.0.1:8080:8080"
```

#### 推荐用宿主机版

如果满足：

- cloudflared 已经通过 Homebrew 或 launchd 稳定运行；
- 希望 Tunnel 不依赖 Docker/Colima 的生命周期；
- 宿主机上还有其他非 Docker 服务也需要暴露；
- 不想把 Cloudflare token/配置放进 Docker Compose。

配置：

```text
http://127.0.0.1:8080
```

并保持：

```bash
BIND_HOST=127.0.0.1
```

### 19.3 选择建议

对于当前这种“一台 Mac、一个 Sub2API、使用 Colima + Docker Compose”的部署：

> 优先推荐 cloudflared 和 Sub2API 放在同一个 Docker Compose 网络里。

原因是：

- 不需要宿主机暴露 8080；
- 不需要额外维护第二个 launchd 服务；
- 网络隔离更好；
- 配置和日志集中在一个 compose 项目里；
- 可以用 `http://sub2api:8080`，不会受宿主端口变化影响。

但如果你发现 cloudflared 容器和 Sub2API 不在同一个 Docker network，不要直接写 `http://sub2api:8080`，应二选一：

```text
加入同一个 network：http://sub2api:8080
独立容器访问宿主机：http://host.docker.internal:8080
```

---

## 20. Cloudflare Tunnel 安全加固

### 20.1 两种方式的本质区别

cloudflared 无论是 Docker 还是宿主机进程，公网入口都走 Cloudflare Tunnel：

```text
公网 → Cloudflare Edge → 加密 Tunnel → cloudflared → Sub2API
```

它不会像传统端口映射那样把 8080 直接开放到公网。  
但两者在**本机网络隔离**上有区别。

| 项目 | Docker 版 cloudflared | 宿主机版 cloudflared |
| --- | --- | --- |
| 是否需要发布 8080 到宿主机 | 同网络时不需要 | 需要 `127.0.0.1:8080` |
| 局域网/本机其他进程访问 Sub2API | 更难 | 本机进程可以直接访问 loopback |
| 被攻击后的横向移动范围 | 取决于 Docker network | 取决于本机 loopback 上的服务 |
| 管理复杂度 | 更集中 | 独立进程 |
| 启动/重启影响 | 随 Docker/Colima | Tunnel 独立 |

单从“端口暴露面”看：

> Docker 版 + 同网络 + 不发布宿主机端口，通常比宿主机版更安全。

但 Docker 版如果让 cloudflared 和 PostgreSQL、Redis 处在同一个网络里，cloudflared 容器被攻破后可以横向访问数据库和缓存。更安全的做法是拆分 Docker network。

### 20.2 推荐网络拓扑

```text
Cloudflare Edge
      │
      ▼
cloudflared
      │  edge network
      ▼
   sub2api
   │      │
   │      └── backend network（internal）
   │                ├── postgres
   │                └── redis
   │
   └── egress network（访问上游 API）
```

原则：

- cloudflared 只加入 `edge` 网络，只访问 sub2api；
- postgres、redis 只加入 `backend` 网络；
- `backend` 设置为 `internal: true`，不允许直接访问公网；
- sub2api 同时加入 `edge`、`backend` 和用于访问上游的 `egress` 网络；
- 不要给 cloudflared 挂载 Docker socket；
- 不要使用 `privileged: true`；
- 不要使用 `network_mode: host`。

### 20.3 cloudflared Docker 加固

建议：

```yaml
services:
  cloudflared:
    image: cloudflare/cloudflared:latest
    restart: unless-stopped
    command: tunnel --no-autoupdate run --token ${CLOUDFLARE_TUNNEL_TOKEN}
    read_only: true
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
    depends_on:
      - sub2api
```

注意：

- Tunnel token 相当于运行该 Tunnel 的凭证；
- token 不要提交到 Git；
- `.env` 保持 `600`；
- 不要给 cloudflared 挂载 `/var/run/docker.sock`；
- 不要给 cloudflared 额外挂载宿主机目录；
- 定期更新 `cloudflare/cloudflared` 镜像。

### 20.4 Cloudflare 侧加固

建议至少配置：

- Cloudflare 账号开启 2FA；
- 使用 Cloudflare Access 保护管理后台；
- 对 `/admin`、登录页和敏感接口配置 WAF 规则；
- 开启 Rate Limiting；
- 开启 Bot Fight Mode 或等价策略；
- 只允许必要的 HTTP 方法；
- 限制请求体大小；
- 定期查看 Cloudflare Security Events；
- 不要把 Tunnel token 发到聊天工具或公开仓库。

如果整个站点都要 Access 保护，注意 API 客户端可能需要配置 Service Token；  
如果只保护 `/admin`，公开 API 仍然可以用 Sub2API 自己的 API Key 鉴权。

### 20.5 Sub2API 侧加固

- 管理员使用强密码；
- 开启管理员 TOTP；
- 不要把管理员账号和普通用户混用；
- `BIND_HOST=127.0.0.1`；
- 不要发布 PostgreSQL/Redis 端口；
- 定期备份数据库和密钥；
- 只从可信网络或 Cloudflare 访问管理后台；
- 不要在公网直接暴露 `/metrics`、调试端口或数据库端口。

### 20.6 结论

对于单 Mac、单 Sub2API、Colima + Docker Compose 的场景：

```text
cloudflared 放在同一个 Docker Compose
+ 独立 edge/backend 网络
+ 不发布宿主机 8080
+ Cloudflare Access
+ WAF/Rate Limit
```

是安全性和运维便利性比较均衡的方案。

如果 cloudflared 已经稳定运行在宿主机，也可以继续使用，但应确保：

```text
BIND_HOST=127.0.0.1
```

并且本机没有其他不可信进程可以访问 loopback。

---

## 21. Docker 网络隔离能防什么，不能防什么

### 21.1 结论先说

Docker network 隔离可以**显著降低横向移动范围**，但不能单独“保证安全”。

它主要防的是：

- 其他无关容器直接访问 Sub2API；
- Sub2API 直接访问 PostgreSQL/Redis；
- cloudflared 直接访问数据库/缓存；
- 宿主机上未发布到宿主端口的服务被容器直接访问；
- 容器之间的无关通信。

它不能完全防：

- Sub2API 被远程代码执行后，利用允许的出站网络攻击公网目标；
- 通过 NAT 访问宿主机所在局域网中的其他设备；
- Docker daemon、内核、容器逃逸类漏洞；
- 通过 `host.docker.internal` 或宿主机已发布端口访问宿主机服务；
- 应用层漏洞、弱密码、被盗 API Key、Cloudflare 配置错误。

### 21.2 Colima 的额外边界

当前环境是：

```text
macOS host
  └── Colima Linux VM
        └── Docker daemon
              ├── sub2api
              ├── cloudflared
              ├── postgres
              └── redis
```

容器攻击者首先落入的是 **Colima 的 Linux VM**，不是直接进入 macOS 内核。  
这对 macOS 本身是一层明显保护，但攻击者仍可能：

- 在 VM 内进行横向移动；
- 通过网络访问 LAN 内其他设备；
- 访问已经发布到宿主机的服务；
- 利用 Colima/容器运行时漏洞尝试逃逸。

### 21.3 推荐网络拓扑

```text
Cloudflare
    │
    ▼
cloudflared
    │  edge（internal）
    ▼
sub2api
    │  backend（internal）
    ├── postgres
    └── redis
    │
    └── egress（允许访问上游 API）
```

Compose 示例：

```yaml
services:
  cloudflared:
    image: cloudflare/cloudflared:latest
    restart: unless-stopped
    command: tunnel --no-autoupdate run --token ${CLOUDFLARE_TUNNEL_TOKEN}
    networks:
      - edge
      - tunnel
    read_only: true
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL

  sub2api:
    image: sub2api:custom
    pull_policy: never
    networks:
      - edge
      - backend
      - egress

  postgres:
    image: postgres:18-alpine
    networks:
      - backend

  redis:
    image: redis:8-alpine
    networks:
      - backend

networks:
  edge:
    internal: true
  backend:
    internal: true
  egress:
  tunnel:
```

说明：

- `edge` 使用 `internal: true`，cloudflared 和 sub2api 只能在这个内部网络通信；
- `backend` 使用 `internal: true`，数据库和 Redis 没有直接公网出口；
- cloudflared 通过 `tunnel` 网络出站到 Cloudflare；
- sub2api 通过 `egress` 网络访问上游模型 API；
- 不要给任何容器挂载 `/var/run/docker.sock`；
- 不要使用 `privileged: true`；
- 不要使用 `network_mode: host`。

### 21.4 还需要做什么

网络隔离只是其中一层。建议同时做：

1. 宿主机服务尽量只监听 `127.0.0.1`；
2. 不要发布 PostgreSQL/Redis 端口；
3. Cloudflare 管理后台加 Access；
4. 开启 WAF / Rate Limit；
5. 管理员开启 TOTP；
6. 容器使用非 root、只读文件系统、`cap_drop: ALL`；
7. 定期更新镜像；
8. 不挂 Docker socket；
9. 监控异常网络连接和登录日志；
10. 定期备份并验证恢复流程。

### 21.5 真正的“保证”

如果目标是尽可能接近“即使 Sub2API 被攻破，也不能接触 macOS 和 LAN”，需要做**出站访问控制**，而不仅是 Docker network：

- 在 Colima VM 内用 iptables/nftables 限制容器访问 RFC1918、link-local 和宿主机网段；
- 只允许访问模型上游所需的公网地址；
- 或者把 Sub2API 放到独立机器/独立网络；
- 使用 Cloudflare Tunnel，不开放任何入站端口；
- 宿主机服务只绑定 loopback；
- 数据库/Redis 只放在 `internal` 网络。

Docker network 能有效减少攻击面，但不能替代主机防火墙、出站策略、应用安全和 Cloudflare 访问控制。

---

## 22. 普通用户能否通过 Sub2API 攻击 macOS 或本地网络

### 22.1 基于当前代码的结论

基于当前版本代码做静态检查，**普通用户没有直接获得以下能力**：

- 任意指定 URL 让服务器发起请求；
- 在服务端执行任意命令；
- 读取宿主机任意文件；
- 直接访问 PostgreSQL/Redis；
- 直接指定上游目标地址。

普通用户可用的主要接口集中在：

- 个人资料、密码、TOTP、Passkey；
- API Key 管理；
- 用量查询；
- 订阅查询；
- 公告、兑换码；
- 渠道监控只读接口；
- 网关模型调用。

这些接口本身不提供“任意 URL 请求”或“命令执行”的原语。

### 22.2 网关请求的目标由管理员配置

普通用户调用模型时控制的是：

- 模型名；
- 请求内容；
- API Key。

上游账号、Base URL、代理、分组和路由由管理员配置，普通用户不能直接指定：

```text
http://127.0.0.1:...
http://192.168.x.x:...
http://169.254.169.254/...
```

因此不能简单地通过修改请求体把网关变成 SSRF 工具。

### 22.3 代码中的 SSRF 防护

当前代码里可以看到这些防护：

- `security.url_allowlist` 配置；
- `WithHTTPUpstreamPublicHostsOnly` 标记；
- 图片 URL 回填会拒绝 loopback、私网、链路本地地址；
- 渠道监控 URL 有独立 SSRF 防护和 DNS rebinding 防护；
- 代理探测、审计节点等配置属于管理员面；
- 上游 URL、重定向和 DNS 解析存在校验链路。

代码里的危险操作，例如：

- `pg_dump` / `psql` 命令执行；
- 备份导入导出；
- 代理配置；
- 账号 Base URL 配置；
- 插件和审计节点配置；
- 管理设置修改；

都属于管理员权限范围，不属于普通用户权限范围。

### 22.4 仍然不能掉以轻心的地方

“没有直接的 SSRF 原语”不等于“绝对安全”。仍然要考虑：

1. 应用层 RCE 漏洞；
2. 依赖包供应链漏洞；
3. 管理员账号或 Admin API Key 泄露；
4. Tunnel Token 泄露；
5. 管理员配置了恶意的上游地址或代理；
6. 容器逃逸或 Docker/Colima 漏洞；
7. 普通用户通过合法网关请求触发上游返回恶意 URL；
8. 未来版本新增功能引入新的 SSRF 面。

尤其注意：

> 如果攻击者拿到管理员权限，他就可以修改账号、代理、分组和设置。  
> 这时应用层的 URL 校验和网络隔离才是最后的防线。

### 22.5 实际防护建议

建议假设“应用可能被攻破”，继续做：

- cloudflared 和 sub2api 只共享 edge 网络；
- PostgreSQL/Redis 放在 internal 网络；
- 宿主机服务只监听 `127.0.0.1`；
- 不挂 Docker socket；
- 容器非 root、只读、`cap_drop: ALL`；
- 在 Colima VM 内限制容器出站访问 LAN 网段；
- 开启 Cloudflare Access、WAF、Rate Limit；
- 管理员开启 TOTP；
- 定期更新 Sub2API 和基础镜像。

### 22.6 结论

对普通用户来说：

```text
直接通过 Sub2API 的常规用户接口攻击 macOS 或本地网络：
没有发现直接的、低门槛的实现路径。
```

但不能因此认为绝对安全：

```text
应用漏洞 + 容器出站网络 + 管理员权限泄露
仍然可能造成风险。
```

Docker 网络隔离、Cloudflare Access、出站防火墙和最小权限容器仍然必须保留。

---

## 23. 通过 Cloudflare Tunnel 访问时，本地网络会暴露吗

### 23.1 Tunnel 的正常网络路径

```text
用户浏览器 / API 客户端
        │ HTTPS
        ▼
Cloudflare Edge
        │ 已建立的 Tunnel
        ▼
cloudflared（Mac mini / Docker）
        │ HTTP
        ▼
Sub2API
```

cloudflared 是**主动向外**连接 Cloudflare 的，不需要：

- 公网 IPv4；
- 路由器端口转发；
- 防火墙开放 8080；
- 公网可路由的域名指向家里 IP。

### 23.2 普通用户能看到什么

普通用户访问：

```text
https://sub2api.example.com
```

能看到：

- 域名；
- Cloudflare Edge 的 IP；
- 正常的 TLS 证书；
- HTTP 响应头；
- 页面内容。

通常看不到：

- 你的家庭宽带公网 IPv4；
- 你的 Mac mini 的局域网 IPv4，例如 `192.168.x.x`；
- 你的局域网设备；
- Colima VM 的 IP；
- Docker 容器内部 IP；
- 你的路由器管理地址。

DNS 解析通常也是 Cloudflare 的 Anycast IP，而不是你的家庭公网 IP：

```bash
dig +short sub2api.example.com
```

不应该出现你的家庭公网 IPv4。

### 23.3 Cloudflare 自身能看到什么

Cloudflare 作为 Tunnel 的接收端，可以看到：

- cloudflared 连接的公网出口 IP；
- Tunnel ID；
- 请求内容（除非配置了额外的端到端加密）；
- 你的 Cloudflare 账号配置。

这属于 Cloudflare 的信任边界，不等于普通用户能看到你的家庭网络。

### 23.4 本地局域网会不会被“暴露”

Tunnel 本身不会把整个局域网暴露出去。它只会把 ingress 中明确配置的服务暴露出去，例如：

```yaml
ingress:
  - hostname: sub2api.example.com
    service: http://sub2api:8080
  - service: http_status:404
```

只有 `sub2api.example.com` 会路由到 `sub2api:8080`。  
不会自动暴露：

- 路由器；
- NAS；
- 其他电脑；
- PostgreSQL；
- Redis；
- Colima VM；
- Docker 其他容器。

前提是不要在 ingress 中配置通配符或错误的服务地址。

### 23.5 需要真正注意的风险

#### 1. 应用层 RCE / SSRF

如果 Sub2API 或 cloudflared 被攻破，攻击者可能在容器内尝试访问局域网。这不是 Tunnel 自己暴露了 LAN，而是攻击者获得了网络出口。

#### 2. 管理员权限泄露

管理员可以修改上游地址、代理和系统设置，风险高于普通用户。

#### 3. Tunnel Token 泄露

Tunnel Token 泄露后，攻击者可能接管或干扰 Tunnel。应保存在 `.env` 或 secret 中，不提交 Git。

#### 4. DNS 配置错误

如果域名不是 Cloudflare 代理模式，而是直接解析到你的家庭公网 IP，就会暴露公网地址。Tunnel 场景应确保 DNS 记录是 Proxied / 橙色云。

#### 5. 错误绑定 `0.0.0.0`

```bash
BIND_HOST=127.0.0.1
```

只允许本机访问。  
如果写成 `0.0.0.0`，同一局域网内的设备可能直接访问宿主机端口，这与 Tunnel 无关，是本地暴露。

### 23.6 建议的验证方法

```bash
# 解析域名
dig +short sub2api.example.com

# 检查响应头
curl -I https://sub2api.example.com

# 检查本机监听
lsof -nP -iTCP -sTCP:LISTEN | grep 8080
```

预期：

- DNS 返回 Cloudflare IP；
- 响应头不包含你的家庭 IP 或局域网 IP；
- 8080 只监听 `127.0.0.1` 或只在 Docker 内部使用。

### 23.7 结论

对于普通用户来说：

```text
域名 → Cloudflare → Tunnel → 本机 Sub2API
```

正常配置下：

- 不会暴露你的家庭公网 IPv4；
- 不会暴露局域网 IPv4；
- 不会暴露整个 LAN；
- 不会暴露路由器或其他设备；
- 不需要开放入站端口。

但如果你主动把 `BIND_HOST` 设为 `0.0.0.0`、错误配置 DNS、泄露 Tunnel Token，或者应用本身被攻破，风险会变化。Tunnel 是入口保护，不是应用层和出站网络的完整安全保证。

---

## 24. 从 macOS launchd cloudflared 迁移到 Docker 版

### 24.1 这样做是否更安全

可以更安全，但前提是**网络设计正确**。

关键不是“cloudflared 放进 Docker”本身，而是：

- cloudflared 不再依赖宿主机 `127.0.0.1:8080`；
- Sub2API 不再需要把 8080 发布到宿主机；
- cloudflared 只加入 edge 网络，不能直接访问 PostgreSQL/Redis；
- cloudflared 没有 Docker socket、没有 privileged、没有 host network。

如果直接把 cloudflared 加入所有服务共用的 `sub2api-network`，而数据库和 Redis 也在同一个网络里，那么安全性提升有限，因为 cloudflared 被攻破后仍能访问数据库和缓存。

### 24.2 推荐网络结构

```text
Cloudflare Edge
      │ Tunnel
      ▼
cloudflared
      │ edge（internal）
      ▼
sub2api
      │ backend（internal）
      ├── postgres
      └── redis
      │
      └── egress（访问上游 API）

cloudflared 另接 tunnel network，用于连接 Cloudflare
```

Compose 关键部分：

```yaml
services:
  cloudflared:
    image: cloudflare/cloudflared:2026.7.3
    restart: unless-stopped
    command: tunnel --no-autoupdate run --token ${CLOUDFLARE_TUNNEL_TOKEN:?CLOUDFLARE_TUNNEL_TOKEN is required}
    depends_on:
      sub2api:
        condition: service_healthy
    networks:
      - edge
      - tunnel
    mem_limit: 128m
    cpus: 0.5
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL

  sub2api:
    networks:
      - edge
      - backend
      - egress

  postgres:
    networks:
      - backend

  redis:
    networks:
      - backend

networks:
  edge:
    internal: true
  backend:
    internal: true
  tunnel:
  egress:
```

`cloudflared` 镜像版本请替换为实际可用的固定版本，不建议长期使用 `latest`。

### 24.3 Cloudflare Dashboard 修改

原来宿主机 cloudflared 通常指向：

```text
http://127.0.0.1:8080
```

改成 Docker 网络内的服务名：

```text
http://sub2api:8080
```

前提：

- cloudflared 和 sub2api 都在 `edge` 网络；
- `sub2api` 服务名能被 Docker DNS 解析。

如果 Cloudflare 使用本地配置文件而不是 dashboard 托管，则修改 `ingress.service`：

```yaml
ingress:
  - hostname: sub2api.example.com
    service: http://sub2api:8080
  - service: http_status:404
```

### 24.4 迁移步骤

1. 先停止宿主机 cloudflared：

```bash
# brew 安装的情况
brew services stop cloudflared

# 或 launchd 自定义 plist 的情况
launchctl list | grep -i cloudflare
launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/<cloudflared 的 plist>
```

2. 在 Compose 中加入 cloudflared 服务和上面的网络配置。

3. 启动：

```bash
cd ~/sub2api-deploy

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  up -d --wait
```

4. 查看 cloudflared 日志：

```bash
docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  logs -f cloudflared
```

5. 确认 Cloudflare Dashboard 的 Public Hostname Origin 已改为：

```text
http://sub2api:8080
```

6. 从公网验证：

```bash
curl -I https://sub2api.example.com
```

7. 确认宿主机没有不必要的 8080 监听：

```bash
lsof -nP -iTCP -sTCP:LISTEN | grep 8080
```

如果不再需要本机直接访问，建议不要在 compose 中发布 8080。

### 24.5 注意事项

- 不要同时长期运行宿主机 cloudflared 和 Docker cloudflared 两个 connector，除非明确要做高可用；
- 两个 connector 同时连接同一个 Tunnel 时，流量可能被分配到任意一个，排障会变复杂；
- Docker/Colima 停止时 Tunnel 也会停止，但宿主机进程同样无法在源站停止时提供正常服务；
- Tunnel Token 继续放在 `.env`，权限保持 `600`；
- 不要挂载 `/var/run/docker.sock`；
- 不要使用 `network_mode: host`。
