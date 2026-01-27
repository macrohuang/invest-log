# Invest Log - macOS 应用打包说明

## 构建成功！

恭喜！你的 Invest Log 应用已成功打包为 macOS 应用程序。

## 输出文件

构建完成后，你可以在以下位置找到打包好的文件：

### 1. macOS 应用包（.app）
**路径：** `src-tauri/target/release/bundle/macos/Invest Log.app`
- **大小：** 约 191 MB
- **用途：** 可直接双击运行的 macOS 应用
- **说明：** 包含 Tauri 前端（4.3MB）和 Python 后端（187MB）

### 2. DMG 安装文件
**路径：** `src-tauri/target/release/bundle/dmg/Invest Log_0.1.0_aarch64.dmg`
- **大小：** 约 188 MB
- **用途：** 可分发给其他用户的安装文件
- **说明：** 双击 DMG 文件，将应用拖动到 Applications 文件夹即可安装

## 使用方法

### 方式一：直接运行 .app 文件
1. 打开 Finder，导航到 `src-tauri/target/release/bundle/macos/`
2. 双击 `Invest Log.app`
3. 如果遇到"无法打开，因为来自身份不明的开发者"提示：
   - 右键点击应用 → 选择"打开"
   - 或在系统偏好设置 → 安全性与隐私中点击"仍要打开"

### 方式二：使用 DMG 安装
1. 双击 `Invest Log_0.1.0_aarch64.dmg`
2. 将 `Invest Log` 拖动到 `Applications` 文件夹
3. 从启动台或应用程序文件夹中打开应用

## 首次运行

首次运行时，应用会提示你选择数据存储位置：

### 选项 1：iCloud Drive（推荐）
- 数据自动同步到其他 Apple 设备
- 路径：`~/Library/Mobile Documents/com~apple~CloudDocs/InvestLog/`

### 选项 2：自定义文件夹
- 完全控制数据位置
- 适合需要手动备份的场景

### 选项 3：本地应用数据文件夹
- 数据存储在本地
- 路径：`~/Library/Application Support/InvestLog/`

## 应用功能

应用启动后会自动：
1. 启动 Python 后端服务（端口 8000）
2. 初始化 SQLite 数据库
3. 打开 Web 界面

### 主要功能
- 📊 投资组合概览
- 💰 交易记录管理（支持买入、卖出、分红、拆分等）
- 📈 持仓详情和价格更新
- 🎯 资产配置监控
- 💱 多币种支持（CNY/USD/HKD）
- 🏦 多资产类型（股票/债券/贵金属/现金）

## 技术细节

### 构建配置
- **平台：** macOS (Apple Silicon / ARM64)
- **前端：** Tauri 2.x + Rust
- **后端：** Python 3.11 + FastAPI
- **打包工具：** PyInstaller

### 包含的组件
1. **Tauri 应用**（4.3 MB）
   - Rust 编译的原生 macOS 应用
   - 负责管理窗口和启动 Python 后端

2. **Python 后端**（187 MB）
   - FastAPI Web 服务器
   - SQLite 数据库操作
   - 价格获取服务（AKShare、Yahoo Finance等）
   - 所有 Python 依赖已打包进单个可执行文件

### 数据存储
- **数据库：** SQLite (`transactions.db`)
- **日志：** 自动按天轮转，保留 7 天
- **配置：** JSON 格式，存储在应用数据目录

## 重新构建

如果需要重新构建应用：

```bash
# 构建 Python sidecar（仅后端）
./scripts/build.sh sidecar

# 完整构建（包括 Tauri 应用）
./scripts/build.sh release

# 开发模式（实时重载）
./scripts/build.sh dev
```

## 故障排查

### 应用无法启动
1. 检查是否允许了"来自身份不明的开发者"的应用
2. 查看系统日志：Console.app → 搜索 "Invest Log"
3. 确认 Python 后端是否正常启动

### 数据库相关问题
1. 检查数据目录权限
2. 如果使用 iCloud，确认 iCloud Drive 已启用
3. 尝试使用自定义文件夹重新初始化

### 价格更新失败
1. 检查网络连接
2. 某些数据源可能需要特殊网络访问
3. 查看应用内的操作日志

## 分发给其他用户

如果要分发给其他 Mac 用户（M系列芯片）：
1. 使用 DMG 文件进行分发
2. 告知用户首次打开时需要右键 → 打开
3. 提供本说明文件作为参考

## 技术支持

如遇到问题，请检查：
- 应用日志（在数据目录的 `logs/` 文件夹中）
- 控制台输出（如果从终端启动应用）
- 系统要求：macOS 10.15+ (Catalina or later)

## 更新日志

### 2026-01-25 v0.1.0 - 修复版本

**修复的问题：**
1. ✅ 修复了应用启动卡在"Starting Invest Log..."页面的问题
2. ✅ 移除了错误的 `frontendDist` 配置
3. ✅ 正确配置窗口 URL 为 `http://127.0.0.1:8000`
4. ✅ 改进了后端日志输出（使用 Stdio::inherit）
5. ✅ 添加了后端就绪检测机制

**技术改进：**
- Tauri 窗口现在正确加载 Python FastAPI 后端
- 后端日志直接输出到控制台，便于调试
- 添加了 TCP 端口检测，确保后端完全启动

---

**构建时间：** 2026-01-25 18:20
**应用版本：** 0.1.0
**构建平台：** macOS ARM64 (Apple Silicon)
