# Invest Log - 桌面应用打包说明（Electron）

## 构建输出

构建完成后，输出目录由 electron-builder 决定（默认 `dist/`）。

- macOS：通常会生成 `.app` 与 `.dmg`
- Windows：通常会生成安装包或可执行文件
- Linux：通常会生成 AppImage 或其他格式

具体文件名以 electron-builder 实际输出为准。

## 构建步骤

```bash
# 仅构建后端（Python sidecar）
./scripts/build.sh sidecar

# 开发模式（Electron）
./scripts/build.sh dev

# 发布构建（Electron）
./scripts/build.sh release
```

说明：
- 后端二进制由 PyInstaller 生成，输出在 `dist/` 下
- Electron 打包会将 `dist/` 作为额外资源打入应用（见 `package.json` 的 build.extraResources）

## 常见问题

### 应用启动后一直停在加载页
- 确认后端二进制已生成：`dist/invest-log-backend-*`
- 开发模式请先运行：`./scripts/build.sh sidecar`
- 检查是否被防火墙/系统权限拦截本地服务

### 需要清理重建
```bash
rm -rf build dist
```

## 技术栈

- 前端壳：Electron
- 后端：Python 3 + FastAPI
- 打包：PyInstaller（后端）+ electron-builder（桌面应用）
