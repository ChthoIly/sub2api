# Sub2API Fork 完整部署文档

本文档适用于当前这个 fork：

- Fork：`ChthoIly/sub2api`
- 上游：`Wei-Shaw/sub2api`
- 镜像：`ghcr.io/chthoilly/sub2api`
- 当前默认部署方式：**Docker Compose + GHCR 镜像**

整体目标：

```text
本机修改代码
   ↓
git push custom
   ↓
GitHub Actions
   ├─ 同步 upstream/main
   ├─ rebase custom
   ├─ 构建 Docker 镜像
   └─ 推送到 GHCR
   ↓
本机 / 生产服务器
   ├─ docker compose pull
   └─ docker compose up -d
```

这样你不需要每次在本机编译，也不需要生产服务器安装 Go/Node。

---

## 1. 分支模型

| 引用 | 用途 | 是否放个人提交 |
| --- | --- | --- |
| `upstream/main` | 上游原版 | 否 |
| `origin/main` | 上游镜像分支 | 否 |
| `origin/custom` | 你的功能分支、镜像构建分支 | 是 |

重要：

- 个人改动只放在 `custom`。
- `main` 只用来同步 upstream。
- GitHub 仓库默认分支设置为 `custom`，因为定时 workflow 只从默认分支读取。

---

## 2. 环境要求

### 本机

当前本机是 Apple Silicon Mac，`arm64`。

需要：

- Docker Desktop 或 Colima
- Docker Compose v2+
- Git
- 浏览器

检查：

```bash
docker version
docker compose version
uname -m
```

本机当前应看到：

```text
linux/arm64
```

因此 workflow 需要构建 `linux/arm64` 镜像。

### 生产服务器（如果以后部署到 Linux）

如果生产服务器是 x86_64：

```text
linux/amd64
```

需要修改 `.github/workflows/fork-sync.yml`：

```yaml
platforms: linux/amd64
```

或者同时支持本地 Mac 和 x86 服务器：

```yaml
platforms: linux/amd64,linux/arm64
```

多架构构建会慢一些，但同一个镜像 tag 可以同时支持 amd64 和 arm64。

---

## 3. 首次初始化分支

以下命令在本地主仓库执行。执行前先确认 `main` 没有需要保留的个人提交。

```bash
git remote add upstream https://github.com/Wei-Shaw/sub2api.git 2>/dev/null || \
  git remote set-url upstream https://github.com/Wei-Shaw/sub2api.git

git fetch upstream
git fetch origin

# main 只做 upstream 镜像
git switch main
git reset --hard upstream/main
git push origin main

# 创建个人长期分支
git switch -c custom
```

把自动同步文件提交到 `custom`：

```bash
git add .github/workflows/fork-sync.yml FORK_SYNC_CN.md DEPLOYMENT_CN.md
git commit -m "chore: add fork sync and deployment docs"
git push -u origin custom
```

然后到 GitHub 仓库：

1. `Settings → General → Default branch` 改为 `custom`。
2. `Settings → Actions → General → Workflow permissions` 选择允许读写。
3. 确认 Actions 可以推送 `main` 和 `custom`。
4. 不要给 `custom` 开启禁止 force push 的保护，因为 rebase 会改写 SHA。

---

## 4. GitHub Actions 自动同步与构建

workflow 文件：

```text
.github/workflows/fork-sync.yml
```

触发时机：

- 每天 03:17 UTC 自动运行；
- 手动 `Run workflow`；
- 向 `custom` push 时运行。

工作内容：

1. 拉取 `upstream/main`；
2. 将 `origin/main` fast-forward 到 upstream；
3. 在 `custom` 上执行 `git rebase upstream/main`；
4. 使用 `--force-with-lease` 推送 rebase 后的 `custom`；
5. 构建 `linux/arm64` Docker 镜像；
6. 推送到 GHCR。

镜像 tag：

```text
ghcr.io/chthoilly/sub2api:custom
ghcr.io/chthoilly/sub2api:custom-<短SHA>
```

第一次构建完成后，到 GitHub 的 `Packages` 页面确认 package 已生成。

如果服务器要匿名拉取镜像，把 package 设为 public；否则服务器需要登录 GHCR。

---

## 5. 部署目录准备

不要在源码仓库里保存运行数据。单独创建部署目录：

### 本机 macOS

```bash
mkdir -p ~/sub2api-deploy
cd ~/sub2api-deploy
```

### Linux 生产服务器

```bash
sudo mkdir -p /opt/sub2api-deploy
sudo chown "$USER":"$USER" /opt/sub2api-deploy
cd /opt/sub2api-deploy
```

从仓库复制 compose 文件：

```bash
cp /path/to/sub2api/deploy/docker-compose.local.yml .
cp /path/to/sub2api/deploy/.env.example .env
chmod 600 .env
mkdir -p data postgres_data redis_data
```

如果使用本机，路径可以写成：

```bash
cp /Users/chtholly/sub2api/deploy/docker-compose.local.yml .
cp /Users/chtholly/sub2api/deploy/.env.example .env
```

---

## 6. 配置 `.env`

生成密钥：

```bash
openssl rand -hex 32
openssl rand -hex 32
openssl rand -hex 32
```

在 `.env` 中至少配置：

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

- `.env` 不要提交到 Git。
- `.env` 权限保持 `600`。
- 不要使用示例密码。
- 修改 `.env` 后需要重新执行 `docker compose up -d`。

如果 `ADMIN_EMAIL` / `ADMIN_PASSWORD` 留空，首次启动日志会生成随机管理员账号：

```bash
docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  logs sub2api | grep "Generated admin"
```

---

## 7. 指定你自己的镜像

在部署目录新增 `docker-compose.override.yml`：

```yaml
services:
  sub2api:
    image: ghcr.io/chthoilly/sub2api:custom
    pull_policy: always
```

完整启动命令统一使用：

```bash
docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  up -d
```

---

## 8. GHCR 登录

如果 GHCR package 是公开的，可以跳过。

私有 package 先登录一次：

```bash
echo "$GHCR_TOKEN" | docker login ghcr.io -u ChthoIly --password-stdin
```

`GHCR_TOKEN` 建议使用 fine-grained PAT，只需要：

```text
read:packages
```

不要给服务器长期使用拥有写权限的 token。

---

## 9. 本机 macOS 部署

### 9.1 启动

```bash
cd ~/sub2api-deploy

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  pull sub2api

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  up -d --wait
```

第一次启动会拉取 PostgreSQL、Redis 和 Sub2API 镜像，并自动创建数据库、执行迁移。

### 9.2 访问

```text
http://localhost:8080
```

### 9.3 查看状态

```bash
docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  ps

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  logs --tail=100 sub2api
```

### 9.4 数据目录

```text
~/sub2api-deploy/data
~/sub2api-deploy/postgres_data
~/sub2api-deploy/redis_data
```

升级和重启不会删除这些目录。

---

## 10. Linux 生产服务器部署

生产服务器建议使用 `BIND_HOST=127.0.0.1`，然后由 Caddy/Nginx 提供 HTTPS。

```bash
cd /opt/sub2api-deploy

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  pull sub2api

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  up -d --wait
```

健康检查：

```bash
curl -fsS http://127.0.0.1:8080/health
```

### 关于服务中断

当前方案是单实例 Compose。`docker compose up -d --no-deps --wait sub2api` 在更新镜像时会有短暂中断：

- 旧容器停止；
- 新容器创建；
- 应用启动；
- 执行数据库迁移；
- 健康检查通过。

`--wait` 只是等待新容器健康，不会让旧容器继续接流量。

如果生产环境不能接受中断，需要额外做：

- Caddy / Nginx + 两个应用实例的蓝绿部署；
- Docker Swarm / Kubernetes 滚动更新；
- 连接排空和健康检查。

在生产环境建议：

- 不要开启无审核的自动更新；
- 在低峰期手动更新；
- 大版本更新前备份数据库；
- 保留上一个镜像 SHA tag 用于回滚。

---

## 11. 本机 macOS 自动更新

创建脚本：

```bash
mkdir -p ~/bin
cat > ~/bin/sub2api-update <<'EOF'
#!/bin/bash
set -euo pipefail

export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin"
cd /Users/chtholly/sub2api-deploy

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  pull sub2api

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  up -d --no-deps --wait sub2api
EOF

chmod +x ~/bin/sub2api-update
```

创建 launchd 配置：

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
    <string>/Users/chtholly/bin/sub2api-update</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>StartInterval</key>
  <integer>600</integer>
  <key>StandardOutPath</key>
  <string>/tmp/sub2api-update.log</string>
  <key>StandardErrorPath</key>
  <string>/tmp/sub2api-update.err.log</string>
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

日志：

```bash
tail -f /tmp/sub2api-update.log /tmp/sub2api-update.err.log
```

注意：Docker Desktop 或 Colima 必须处于运行状态。Mac 睡眠时服务也会暂停。

---

## 12. Linux 生产服务器自动更新（可选）

创建 `/usr/local/bin/sub2api-update`：

```bash
#!/usr/bin/env bash
set -euo pipefail
cd /opt/sub2api-deploy

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  pull sub2api

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  up -d --no-deps --wait sub2api

docker image prune -f
```

```bash
chmod +x /usr/local/bin/sub2api-update
```

创建 `/etc/systemd/system/sub2api-update.service`：

```ini
[Unit]
Description=Pull and update Sub2API
Wants=network-online.target
After=network-online.target docker.service

[Service]
Type=oneshot
ExecStart=/usr/local/bin/sub2api-update
```

创建 `/etc/systemd/system/sub2api-update.timer`：

```ini
[Unit]
Description=Update Sub2API every 10 minutes

[Timer]
OnBootSec=5min
OnUnitActiveSec=10min
Persistent=true

[Install]
WantedBy=timers.target
```

启用：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now sub2api-update.timer
```

再次提醒：该方案每次真正更新镜像时会造成短暂服务中断。生产环境更推荐手动发布或蓝绿部署。

---

## 13. 5 小时订阅限额功能

本次功能是**订阅分组级**的 5h 限额。

### 13.1 设置限额

管理后台：

```text
分组管理 → 编辑订阅分组 → 5 小时限额（USD）
```

语义：

```text
空 / 0：不限制
> 0：5 小时 USD 上限
```

注意：5h 限额只对**订阅模式分组**生效。普通余额模式使用的是“用户 × 平台限额”，两者不是同一套数据。

### 13.2 计算方式

- 每个用户单独计算自己的 5h usage；
- 5h 窗口从该用户的首次请求开始；
- 满 5 小时后自动归零并重新开窗；
- 同一个分组内的用户共享同一个 5h limit，但各自独立计数。

### 13.3 查看用量

用户可以查看：

- 订阅页面中的 5h 进度条；
- 顶栏订阅进度；
- API Key 用量页中的 5h 窗口；
- 套餐卡片中的 5h 限额。

管理员可以查看：

- 分组列表中的 5h 限额；
- 订阅列表中的 5h usage 和重置时间；
- 订阅管理中的 5h 数据。

### 13.4 重置 5h 窗口

管理员在订阅管理中重置配额时，会同时重置：

```text
five_hour
daily
weekly
monthly
```

批量重置也支持 5h 窗口。

手动重置 5h 后，窗口会被清空，下一次请求重新开始计时。

### 13.5 API Key 行为

用户仍然可以：

- 创建 API Key；
- 删除/管理 API Key；
- 登录网页；
- 查看用量。

这些操作不会被 5h 限额拦截。5h 限额只在网关调用模型的计费检查中生效。

---

## 14. 升级流程

### 14.1 日常开发发布

```bash
git fetch origin
git switch custom
git pull --rebase origin custom

# 修改代码、测试
git add ...
git commit -m "feat: my change"
git push origin custom
```

push 后 GitHub Actions 自动：

1. 同步 upstream；
2. rebase；
3. 构建镜像；
4. 推送 GHCR。

### 14.2 服务器更新

```bash
cd ~/sub2api-deploy

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  pull sub2api

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  up -d --no-deps --wait sub2api
```

---

## 15. 数据库备份

大版本更新前必须备份。

```bash
cd ~/sub2api-deploy

docker compose \
  -f docker-compose.local.yml \
  exec -T postgres \
  pg_dump -U sub2api -d sub2api \
  | gzip > "backup-$(date +%Y%m%d-%H%M%S).sql.gz"
```

如果修改过默认数据库用户或库名，替换：

```text
-U sub2api
-d sub2api
```

恢复：

```bash
gunzip -c backup-YYYYMMDD-HHMMSS.sql.gz | \
  docker compose \
    -f docker-compose.local.yml \
    exec -T postgres \
    psql -U sub2api -d sub2api
```

同时建议定期备份整个 `postgres_data` 目录或整台机器。

---

## 16. 回滚

镜像同时保留 SHA tag：

```text
ghcr.io/chthoilly/sub2api:custom-<短SHA>
```

回滚时修改 `docker-compose.override.yml`：

```yaml
services:
  sub2api:
    image: ghcr.io/chthoilly/sub2api:custom-<上一个短SHA>
    pull_policy: always
```

然后：

```bash
docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  pull sub2api

docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  up -d --no-deps --wait sub2api
```

注意：

- 数据库迁移可能在启动时自动执行；
- 如果新版本执行了不兼容迁移，仅回滚镜像可能不够；
- 必要时使用数据库备份恢复。

---

## 17. 常用运维命令

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

# 查看资源占用
docker stats
```

---

## 18. 故障排查

### 镜像拉取失败

```text
denied: requested access to the resource is denied
```

处理：

```bash
echo "$GHCR_TOKEN" | docker login ghcr.io -u ChthoIly --password-stdin
```

确认 token 有 `read:packages`，或者把 GHCR package 设为 public。

### `exec format error`

原因：镜像架构和机器架构不一致。

本机 Apple Silicon 需要：

```text
linux/arm64
```

x86 服务器需要：

```text
linux/amd64
```

修改 workflow 的 `platforms` 后重新构建。

### GitHub Actions 没有推送镜像

检查：

- 是否推送到 `custom` 分支；
- 仓库 Actions 是否允许写权限；
- GHCR package 是否被组织策略限制；
- workflow 是否因 rebase 冲突失败。

### rebase 冲突

workflow 会失败并停止，不会推送新镜像。

本地解决：

```bash
git fetch upstream
git fetch origin
git switch custom
git rebase upstream/main

# 解决冲突
git add <冲突文件>
git rebase --continue

git push --force-with-lease origin custom
```

### `origin/main` 与 upstream 分叉

通常是有人误把个人提交放到了 `main`。

处理：

```bash
git fetch upstream
git switch main
git reset --hard upstream/main
git push --force-with-lease origin main
```

个人提交应放在 `custom`，不要放 `main`。

### 5h 限额没有生效

确认：

1. 该 API Key 所在分组是 `subscription` 类型；
2. 该用户有该分组的有效订阅；
3. 分组已配置 `five_hour_limit_usd > 0`；
4. 当前运行的是包含 5h 功能的新镜像；
5. 数据库已执行迁移 `250_subscription_five_hour_limit.sql`。

查看当前镜像：

```bash
docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  images sub2api
```

### 5h 限额为 0

当前实现和现有日/周/月分组限额保持一致：

```text
0 = 不限制
```

如果要禁止使用，应设置一个非常小的正数，或使用其他访问控制。

---

## 19. 安全检查清单

- [ ] `.env` 已设置 `chmod 600`
- [ ] 数据库密码、JWT 密钥、TOTP 密钥都是随机值
- [ ] 管理员密码是强密码
- [ ] GHCR token 只有 `read:packages` 权限
- [ ] PostgreSQL / Redis 未直接暴露公网
- [ ] 生产环境通过 Caddy/Nginx 提供 HTTPS
- [ ] 防火墙只开放 80/443 和必要的 SSH
- [ ] 定期备份数据库
- [ ] 生产自动更新前先评估停机窗口
- [ ] 大版本升级前在 staging 验证

---

## 20. 最短命令清单

```bash
# 首次启动
docker compose -f docker-compose.local.yml -f docker-compose.override.yml pull sub2api
docker compose -f docker-compose.local.yml -f docker-compose.override.yml up -d --wait

# 状态
docker compose -f docker-compose.local.yml -f docker-compose.override.yml ps

# 日志
docker compose -f docker-compose.local.yml -f docker-compose.override.yml logs -f sub2api

# 备份
docker compose -f docker-compose.local.yml exec -T postgres pg_dump -U sub2api -d sub2api | gzip > backup.sql.gz

# 更新
docker compose -f docker-compose.local.yml -f docker-compose.override.yml pull sub2api
docker compose -f docker-compose.local.yml -f docker-compose.override.yml up -d --no-deps --wait sub2api

# 健康检查
curl -fsS http://localhost:8080/health
```
