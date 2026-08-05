<p align="center">
  <img src="frontend/src/assets/omp-logo.svg" width="160" alt="OMP Switch">
</p>

<h1 align="center">OMP Switch</h1>

<p align="center">
  <a href="https://github.com/ADBC123456/omp-switch/releases"><img src="https://img.shields.io/github/v/release/ADBC123456/omp-switch?style=flat-square" alt="Version"></a>
  <a href="https://github.com/ADBC123456/omp-switch/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License"></a>
  <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.22-00ADD8?style=flat-square&logo=go" alt="Go"></a>
  <a href="https://wails.io"><img src="https://img.shields.io/badge/Wails-v2-FF6B6B?style=flat-square" alt="Wails"></a>
  <a href="https://github.com/ADBC123456/omp-switch"><img src="https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-lightgrey?style=flat-square" alt="Platform"></a>
</p>

## 介绍

OMP Switch 是一个跨平台 OMP Provider、模型与模型角色配置工具，基于 Wails 构建。支持管理多个 Provider、识别上游模型、编辑十个模型角色，并一键启动 OMP。

## 致谢

- [Oh My Pi](https://github.com/can1357/oh-my-pi) - OMP 项目
- [Pi Switch](https://github.com/Wing900/Pi-switch) - 特别感谢 Wing900 的开源项目，本项目的界面与 Provider 管理实现受到其启发
- [Linux.do](https://linux.do) - 社区支持

## 功能特性

- 多 Provider 管理（预设：DeepSeek / OpenAI / Anthropic），支持任意 OpenAI 兼容服务
- Anthropic 原生 API 适配：`anthropic-messages` 模式，通过 `/v1/models` + `x-api-key` 拉取模型
- 上游模型识别与审查 / 手动模型管理 / 十个模型角色配置 / 一键启动 OMP

## 开发

待完善中

## Linux 运行依赖

Linux 版本依赖 GTK3 和 WebKitGTK。发布页同时提供两种 WebKitGTK ABI 构建产物，请按系统选择：

| 系统 | ABI | 发布文件后缀 | 运行依赖 |
| --- | --- | --- | --- |
| Debian 12/13+、Ubuntu 22.04+ | 4.1 | `linux-amd64-webkit2-41` | `sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0` |
| 已安装 WebKitGTK 4.0 的 Linux 环境 | 4.0 | `linux-amd64-webkit2-40` | `sudo apt install libgtk-3-0 libwebkit2gtk-4.0-37` |

例如 Debian 13 请选择 `OMP-Switch-<version>-linux-amd64-webkit2-41.tar.gz`。
4.0 构建基于 Ubuntu 22.04，旧发行版还需满足对应的 `glibc` 运行时版本要求。
