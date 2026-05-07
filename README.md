# log-uploader

将本地日志文件上传到 CloudFlare R2 对象存储的 CLI 工具。

## 功能

- 递归扫描指定目录中的日志文件
- 未压缩文件自动 tar.gz 压缩后上传
- 已压缩文件（.zip/.tar/.tar.gz/.gz）直接上传
- 通过 HeadObject 去重，跳过已上传的文件
- 上传失败自动重试（最多 4 次，指数退避）
- 上传后自动清理临时压缩文件
- 按主机名组织上传路径
- 支持按文件修改时间过滤（忽略超过 N 天的文件）
- 上传失败记录到 `/tmp/S3_backup_log_update_fail.log`

## 安装

从 [Releases](../../releases) 下载对应平台的二进制文件，或从源码编译：

```bash
go build -o log-uploader ./cmd/log-uploader
```

## 环境变量

| 变量 | 必填 | 说明 |
|------|------|------|
| `AK` | 是 | R2 Access Key ID |
| `SK` | 是 | R2 Secret Access Key |
| `ENDPOINT` | 是 | R2 端点地址，格式 `https://{account_id}.r2.cloudflarestorage.com/{bucket_name}` |
| `BASE_DIR` | 是 | 上传路径前缀 |
| `LOG_DICT` | 是 | 日志目录列表，分号分隔 |
| `IGNORE_DAYS` | 否 | 忽略超过 N 天未修改的文件，默认 0（不忽略） |
| `WORKERS` | 否 | 并发上传线程数，默认 1（串行） |

## 使用方式

创建一个 bash 脚本（如 `s3_run_backup.sh`）：

```bash
#!/bin/bash

# ========== 配置区 ==========
export AK="your-access-key-id"
export SK="your-secret-access-key"
export ENDPOINT="https://abc123def.r2.cloudflarestorage.com/log-backup"
export BASE_DIR="logs"
export LOG_DICT="/var/log/nginx;/var/log/app;/var/log/syslog"
export IGNORE_DAYS="30"
export WORKERS="8"

# ========== 自动下载并运行 ==========
REPO="jbtt-2025/s3_backup_log_uploader"
BIN="/tmp/log-uploader"

# 检测系统和架构
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  i386|i686) ARCH="i386" ;;
esac

SUFFIX="${OS}-${ARCH}"
TARBALL="log-uploader-${SUFFIX}.tar.gz"

# 获取最新 release 下载地址
DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${TARBALL}"

# 下载、解压、重命名、清理
echo "下载 ${DOWNLOAD_URL} ..."
curl -fSL -o "/tmp/${TARBALL}" "$DOWNLOAD_URL" || { echo "下载失败"; exit 1; }
tar -xzf "/tmp/${TARBALL}" -C /tmp
rm -f "/tmp/${TARBALL}"
mv "/tmp/log-uploader-${SUFFIX}" "$BIN"
chmod +x "$BIN"

# 运行
"$BIN"
```

```bash
chmod +x s3_run_backup.sh
./s3_run_backup.sh
```

### 配合 cron 定时执行

```bash
# 每天凌晨 3 点执行
0 3 * * * /opt/run_backup.sh >> /var/log/log-uploader.log 2>&1
```

## 上传路径格式

```
{BASE_DIR}/{HOSTNAME}/{目录名}/{子目录相对路径}/{文件名}
```

示例：
- 源文件：`/var/log/nginx/access.log`
- 上传路径：`logs/web-server-01/nginx/access.log.tar.gz`

带子目录：
- 源文件：`/var/log/nginx/2024/01/error.log`
- 上传路径：`logs/web-server-01/nginx/2024/01/error.log.tar.gz`

已压缩文件不再额外压缩：
- 源文件：`/var/log/nginx/access.log.gz`
- 上传路径：`logs/web-server-01/nginx/access.log.gz`

## 退出码

- `0`：至少一个目录成功处理
- `非零`：配置错误或所有目录均不可访问

## License

MIT
