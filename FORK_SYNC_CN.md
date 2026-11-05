# Fork 自动同步与部署

目标：`upstream/main`（原版）更新后，自动把个人改动 rebase 到最新上游之上，并构建只有你自己的镜像。

## 1. 分支模型

推荐使用三个引用：

| 引用 | 用途 | 是否接受个人提交 |
| --- | --- | --- |
| `upstream/main` | 原版仓库 | 否 |
| `origin/main` | 原版镜像，只做 fast-forward | 否 |
| `origin/custom` | 你的功能和部署分支 | 是 |

> 把 GitHub 仓库的默认分支改成 `custom`（Settings → Branches → Default branch）。  
> GitHub 的定时 workflow 只会从默认分支读取，所以这一步不能省。


## 镜像在哪里构建

生产部署使用的是预先构建好的 Docker 镜像，不是在生产服务器上编译源码，也不是在本机编译。

完整链路是：

```text
本机写代码并 git push
        |
        v
GitHub Actions（Ubuntu runner）
  - 同步 upstream
  - rebase custom
  - 在 runner 上用 Dockerfile 构建前端和 Go 后端
  - 推送镜像到 GHCR
        |
        v
生产服务器
  - docker compose pull
  - docker compose up -d
```

因此：

- 本机只需要 `git push`，不需要编译、也不需要 `docker push`。
- 生产服务器也不需要拉源码或安装 Go/Node，只拉镜像。
- 本机执行 `docker build` 只用于本地调试镜像，不是生产发布链路的一部分。

不要让 `main` 和 `custom` 同时承载个人提交。`main` 只负责跟上游保持一致，`custom` 只负责你的改动。

## 2. 第一次初始化

```bash
# 在本地仓库中执行
git remote add upstream https://github.com/Wei-Shaw/sub2api.git 2>/dev/null || \
  git remote set-url upstream https://github.com/Wei-Shaw/sub2api.git

git fetch upstream
git fetch origin

# main 必须是 upstream/main 的镜像
git switch main
git reset --hard upstream/main
git push origin main

# 从 main 创建你的长期分支
git switch -c custom
```

把仓库里的 `.github/workflows/fork-sync.yml` 保留在 `custom` 分支，然后：

```bash
git add .github/workflows/fork-sync.yml FORK_SYNC_CN.md DEPLOYMENT_CN.md
git commit -m "chore: add automated fork sync"
git push -u origin custom
```

最后到 GitHub 仓库设置里把默认分支改成 `custom`。此后 workflow 会在每天 03:17 UTC 和手动触发时运行，也会在你向 `custom` push 时运行。

## 3. 日常开发

```bash
git fetch origin
git switch custom
git pull --rebase origin custom

# 改代码、测试、提交
git add ...
git commit -m "feat: my feature"
git push origin custom
```

不要执行 `git merge upstream/main`。如果本地需要先同步上游：

```bash
git fetch upstream
git rebase upstream/main
# 解决冲突后：
#   git add <file>
#   git rebase --continue
git push --force-with-lease origin custom
```

## 4. workflow 做了什么

`.github/workflows/fork-sync.yml`：

1. 拉取 `upstream/main`；
2. 把 `origin/main` fast-forward 到 `upstream/main`；
3. 在 `custom` 上执行 `git rebase upstream/main`；
4. 使用 `--force-with-lease` 推送 `custom`，不会覆盖别人刚推的新提交；
5. 构建并推送 GHCR 镜像：
   - `ghcr.io/<你的用户名>/<仓库名>:custom`
   - `ghcr.io/<你的用户名>/<仓库名>:custom-<短 SHA>`

如果 rebase 冲突，workflow 会失败并停止，不会推送或发布新镜像。按第 3 节在本地解决即可。

> 由 `GITHUB_TOKEN` 推送的提交不会再次触发其他 GitHub Actions workflow。因此构建镜像是同一个 workflow 里直接完成的；如果你还希望原仓库的 CI 在同步后运行，需要给 workflow 配置一个有 `workflow` 权限的 PAT，或把测试步骤合并进本 workflow。

## 5. 本机（macOS）部署

当前机器是 Apple Silicon Mac（`arm64`），Docker 运行在本地即可，不需要远程服务器。workflow 已经配置为构建 `linux/arm64` 镜像；如果你以后也要部署到 x86 主机，把 `platforms` 改成 `linux/amd64,linux/arm64`。

确认本机 Docker 正常：

```bash
docker version
docker compose version
```

准备一个稳定的部署目录（镜像已经构建好，这个目录不需要放源码）：

```bash
mkdir -p ~/sub2api-deploy
cd ~/sub2api-deploy
cp /Users/chtholly/sub2api/deploy/docker-compose.local.yml .
cp /Users/chtholly/sub2api/deploy/.env.example .env
mkdir -p data postgres_data redis_data
```

编辑 `.env`，至少配置：

```bash
POSTGRES_PASSWORD=...
JWT_SECRET=...
TOTP_ENCRYPTION_KEY=...
SERVER_PORT=8080
```

新增 `docker-compose.override.yml`：

```yaml
services:
  sub2api:
    image: ghcr.io/chthoilly/sub2api:custom
    pull_policy: always
```

先登录一次 GHCR（如果 package 设为 public，可以跳过）：

```bash
echo "$GHCR_TOKEN" | docker login ghcr.io -u ChthoIly --password-stdin
```

`GHCR_TOKEN` 需要 `read:packages`。然后启动：

```bash
docker compose   -f docker-compose.local.yml   -f docker-compose.override.yml   pull sub2api

docker compose   -f docker-compose.local.yml   -f docker-compose.override.yml   up -d --no-deps --wait sub2api
```

访问：

```text
http://localhost:8080
```

数据保存在 `~/sub2api-deploy/data`、`postgres_data`、`redis_data`，重启和升级不会丢数据。

## 6. 本机自动更新（launchd）

创建 `~/bin/sub2api-update`：

```bash
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
```

```bash
chmod +x ~/bin/sub2api-update
```

创建 `~/Library/LaunchAgents/com.chthoilly.sub2api-update.plist`：

```xml
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
```

加载并立即跑一次：

```bash
launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/com.chthoilly.sub2api-update.plist 2>/dev/null || true
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.chthoilly.sub2api-update.plist
launchctl kickstart -k gui/$(id -u)/com.chthoilly.sub2api-update
```

这样本机每 10 分钟检查一次新镜像。Docker Desktop 需要随登录启动，或者你使用 Colima 并保证 daemon 已启动；否则脚本会失败，下一次 timer 再重试。

查看日志：

```bash
tail -f /tmp/sub2api-update.log /tmp/sub2api-update.err.log
```

镜像同时带 `custom-<短 SHA>` tag。升级出问题时，把 override 里的 tag 改成上一个 SHA，再执行一次更新脚本即可回滚。

## 7. 如果不想用 GHCR：本机直接构建

如果本机网络访问 GHCR 不稳定，也可以完全在本机构建，不依赖镜像仓库。在源码目录执行：

```bash
cd /Users/chtholly/sub2api
docker build --platform linux/arm64 -t sub2api:custom .
```

然后本机覆盖文件改成：

```yaml
services:
  sub2api:
    image: sub2api:custom
    pull_policy: never
```

再执行：

```bash
docker compose \
  -f docker-compose.local.yml \
  -f docker-compose.override.yml \
  up -d --no-deps --wait sub2api
```

注意：这种方式需要本机保持源码和 Docker 环境可用，而且每次 upstream 更新后都要重新构建。它适合本地调试；如果希望“每天自动跟随 upstream”，还是推荐让 GitHub Actions 构建并让本机只负责 pull。

## 8. 注意事项

- 当前本机是 `arm64`，workflow 必须构建 `linux/arm64` 镜像。
- `custom` 分支不要开启阻止 force push 的 branch protection；rebase 必然改写提交 SHA。
- 每次上游更新都会改写你提交的 SHA，这是 rebase 工作流的正常现象。
- 如果上游 force push 或有人误提交到 `main`，同步 job 会主动失败，等待你人工处理，不会静默丢改动。
- 本机自动更新只是 `docker compose pull + up -d`，不会在容器里编译源码。
