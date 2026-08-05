<p align="center">
  <img src="frontend/src/assets/omp-logo.svg" width="160" alt="OMP Switch">
</p>

<h1 align="center">OMP Switch</h1>

<p align="center">
  <a href="https://github.com/ADBC123456/omp-switch/releases"><img src="https://img.shields.io/github/v/release/ADBC123456/omp-switch?style=flat-square" alt="Version"></a>
  <a href="https://github.com/ADBC123456/omp-switch/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License"></a>
  <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.24-00ADD8?style=flat-square&logo=go" alt="Go"></a>
  <a href="https://wails.io"><img src="https://img.shields.io/badge/Wails-v2-FF6B6B?style=flat-square" alt="Wails"></a>
  <a href="https://github.com/ADBC123456/omp-switch"><img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-lightgrey?style=flat-square" alt="Platform"></a>
</p>

## 介绍

OMP Switch 是 [Oh My Pi（OMP）](https://github.com/can1357/oh-my-pi) 的桌面配置与启动工具。它直接管理 OMP 的 Provider、模型和模型角色配置，并提供模型测试、会话管理、全局 Skill 管理及一键启动入口。

OMP Switch 不包含 OMP 本体。使用前必须先安装 OMP，并确保 `omp` 命令可用；也可在应用设置中填写 OMP 可执行文件的完整路径。

## 功能

| 模块 | 功能 |
| --- | --- |
| Provider | 新建、编辑、切换和删除多个 Provider；内置 Anthropic、DeepSeek、OpenAI 与自定义模板；API Key 保存后不回显 |
| API 协议 | `openai-completions`、`openai-responses`、`anthropic-messages`、`google-generative-ai` |
| 模型 | 从上游 `/models` 端点识别模型，区分新增、已存在、未识别；支持搜索、批量选择导入及取消请求 |
| 手动模型管理 | 添加、编辑和删除模型；可设置显示名称、独立 API 协议、推理能力、上下文窗口和最大输出 Token |
| 模型测试 | 按所选协议向上游发送短消息 `Hi`，显示 HTTP 状态和耗时 |
| 模型角色 | 管理 `default`、`smol`、`slow`、`plan`、`commit`、`vision`、`designer`、`task`、`advisor`、`tiny` 十个角色及 thinking level |
| OMP 启动 | 选择工作目录后，以所选 `provider/model` 启动 OMP；下次目录选择默认定位到上次目录 |
| 会话 | 读取 OMP 会话，按最近使用时间排序；可在原工作目录继续会话或永久删除会话及附件 |
| 全局 Skill | 查看 `~/.agents/skills` 中有效 Skill 的名称、说明、路径和登记状态；可同步删除目录及锁文件登记 |
| 配置安全 | 写入前生成备份；多文件配置采用事务式替换，失败时回滚；删除 Provider/模型前显示受影响角色 |
| 桌面体验 | 浅色、深色、跟随系统主题；配置文件目录入口；Windows 自动更新和手动检查更新 |

## 安装

### 1. 安装 OMP

请以 [OMP 官方安装说明](https://github.com/can1357/oh-my-pi#install) 为准。

Windows PowerShell：

```powershell
irm https://omp.sh/install.ps1 | iex
omp --version
```

macOS / Linux：

```sh
curl -fsSL https://omp.sh/install | sh
omp --version
```

### 2. 安装 OMP Switch

从 [Releases](https://github.com/ADBC123456/omp-switch/releases) 下载与系统匹配的文件：

| 系统 | 架构 | 发布文件 | 使用方式 |
| --- | --- | --- | --- |
| Windows 10/11 | amd64 | `OMP-Switch-<tag>-windows-amd64.exe` | 便携版，直接运行；系统需有 WebView2 Runtime |
| macOS Intel | amd64 | `OMP-Switch-<tag>-macos-amd64.zip` | 解压后运行 `.app` |
| macOS Apple Silicon | arm64 | `OMP-Switch-<tag>-macos-arm64.zip` | 解压后运行 `.app` |
| Linux 桌面 | amd64 | `OMP-Switch-<tag>-linux-amd64-webkit2-41.tar.gz` | 推荐，适用于 WebKitGTK 4.1 环境 |
| Linux 桌面 | amd64 | `OMP-Switch-<tag>-linux-amd64-webkit2-40.tar.gz` | 仅用于仍采用 WebKitGTK 4.0 ABI 的环境 |

Linux 解压与启动示例：

```sh
tar -xzf OMP-Switch-<tag>-linux-amd64-webkit2-41.tar.gz
chmod +x OMP-Switch-<tag>-linux-amd64-webkit2-41
./OMP-Switch-<tag>-linux-amd64-webkit2-41
```

## Linux 运行要求

Linux 构建面向 amd64 图形桌面，不支持纯终端或无显示服务器的环境。除 GTK3 和对应的 WebKitGTK ABI 外，一键启动 OMP 还需要系统提供 `x-terminal-emulator`。

Debian / Ubuntu 常用依赖：

```sh
# WebKitGTK 4.1 构建（推荐）
sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0 gnome-terminal

# WebKitGTK 4.0 构建
sudo apt install libgtk-3-0 libwebkit2gtk-4.0-37 gnome-terminal
```

`x-terminal-emulator` 通常由系统终端包和 Debian alternatives 提供，上例使用 GNOME Terminal。如果启动 OMP 时提示找不到该命令，请安装适合桌面环境的终端模拟器并配置该 alternative。4.0 发布包基于 Ubuntu 22.04 构建，更旧发行版还必须满足对应的 `glibc` 运行时要求。

## 快速开始

### 配置 Provider

1. 点击“添加 Provider”，选择 Anthropic、DeepSeek、OpenAI 或自定义模板。
2. 填写 API 基础地址、API Key、接口协议和 Provider ID。
3. 保存 Provider。

字段规则：

- **Provider ID** 同时作为 OMP Provider 名称，不能为空，且不能包含 `/` 或控制字符；重名时应用会生成可用 ID。
- **API 基础地址** 必须是无 fragment 的绝对 HTTP(S) URL，例如 `https://api.example.com/v1`。
- **API Key** 可填写密钥本身，也可填写环境变量名；若同名环境变量存在，模型识别和测试会读取其值。以 `!` 开头的命令形式密钥不会由 OMP Switch 执行。
- 编辑已有 Provider 时，API Key 输入框留空会保留原值。后端只向界面返回“是否已配置”，不会回显密钥正文。

Provider 和模型变更会同步写入 OMP 的 `models.yml`。首次运行且 OMP Switch 自身配置不存在时，应用会导入已有 `models.yml` 和受管理的模型角色。

### 获取或手动添加模型

保存 Provider 后点击“获取模型”。应用最多请求 20 页，每页超时 12 秒，总操作超时 60 秒；上游必须提供兼容的 `/models` 响应。识别结果只会导入你勾选的新增模型，不会自动删除本地已有模型。

如果上游不提供兼容的模型列表、使用命令形式密钥，或需要 OMP Switch 未配置的特殊鉴权，请使用“添加模型”手工录入。模型级接口协议留空时继承 Provider 协议。

### 测试并启动 OMP

1. 在概览页选择 Provider 和模型。
2. 点击“测试模型”，确认上游请求成功。
3. 点击“启动 OMP”，选择项目工作目录。

实际执行等价于：

```sh
omp --model <provider-id>/<model-id>
```

取消目录选择不会启动进程，也不会修改上次目录。若提示找不到 OMP 命令，请打开“设置”，将“OMP 命令”改为可执行文件完整路径。Windows 还会自动尝试 `%LOCALAPPDATA%\omp\omp.exe`。

### 配置模型角色

“角色模型映射”只写入本次明确修改的角色，不会覆盖 `config.yml` 的其他字段。可为角色选择 `provider/model`，并设置以下 thinking level：

`off`、`minimal`、`low`、`medium`、`high`、`xhigh`、`max`、`auto`

重命名 Provider 或模型时，受管理角色会自动迁移；删除时会先列出受影响角色，确认后清除对应映射。

### 管理会话与 Skill

- **会话管理**读取 `~/.omp/agent/sessions` 下的 OMP JSONL 会话。继续会话会在会话原工作目录执行 `omp --resume <session-file>`。
- 删除会话会永久删除 JSONL 文件及其同名附件目录。
- **OMP Skill**只列出包含 `SKILL.md` 的全局 Skill 目录。删除会同时移除 Skill 目录和 `~/.agents/.skill-lock.json` 中的对应登记，且不可恢复。

## 配置与数据文件

| 路径 | 用途 |
| --- | --- |
| `~/.ompswitch/config.json` | OMP Switch 自身状态、Provider、当前选择和设置 |
| `~/.omp/agent/models.yml` | OMP Provider 与模型配置 |
| `~/.omp/agent/config.yml` | OMP 配置；OMP Switch 只合并十个受管理模型角色 |
| `~/.omp/agent/sessions/` | OMP 会话及附件 |
| `~/.agents/skills/` | 全局 Skill 目录 |
| `~/.agents/.skill-lock.json` | 全局 Skill 登记文件 |
| `~/.ompswitch/backups/` | 配置写入前的备份，最多保留最近 20 个文件 |

API Key 会写入本机配置和 OMP `models.yml`；使用环境变量名可避免直接保存密钥正文。请保护上述配置文件及备份目录。

## 更新与版本

- 应用启动时最多每 7 天静默检查一次 GitHub Release，也可在“设置”中手动检查。
- Windows 支持下载发布页中的 `-windows-amd64.exe` 并延迟替换当前程序。
- macOS 和 Linux 暂不支持应用内自动替换，请从 Releases 手动下载新版本。
- 发布 tag 使用 `vMAJOR.MINOR.PATCH`，例如 `v1.0.0`。GitHub Actions 会把 tag 版本写入应用界面和平台文件元数据。

## 本地开发

要求：Go 1.24、Node.js 20、Wails CLI 2.10.2。

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.2
cd frontend
npm ci
npm test
cd ..
go test ./...
wails build -clean
```

Linux 构建前还需安装 `libgtk-3-dev` 和目标 ABI 的 `libwebkit2gtk-4.1-dev` 或 `libwebkit2gtk-4.0-dev`。推送 `v*` tag 后，发布工作流会构建 Windows amd64、macOS amd64/arm64，以及 Linux amd64 的 WebKitGTK 4.1/4.0 两种产物。

## 致谢

- [Oh My Pi](https://github.com/can1357/oh-my-pi) - OMP 项目
- [Pi Switch](https://github.com/Wing900/Pi-switch) - 特别感谢 Wing900 的开源项目，本项目的界面与 Provider 管理实现受到其启发
- [Linux.do](https://linux.do) - 社区支持
