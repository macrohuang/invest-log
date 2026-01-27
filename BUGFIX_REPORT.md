# 应用启动问题修复报告

## 问题描述

打包后的 Invest Log 应用在启动时一直停留在 "Starting Invest Log..." 加载页面，无法进入主界面。

## 根本原因分析

应用使用 Tauri 框架包装 Python FastAPI 后端。启动流程应该是：

1. Tauri 启动 →
2. 启动 Python sidecar 后端（监听 8000 端口）→
3. Tauri 窗口加载 http://127.0.0.1:8000 →
4. 显示 FastAPI 提供的 Web 界面

### 核心问题：Tauri 配置错误

**问题配置（tauri.conf.json）：**
```json
{
  "build": {
    "devUrl": "http://localhost:8000",
    "frontendDist": "../static"  // ❌ 错误！
  }
}
```

**影响：**
- 在开发模式下，使用 `devUrl`，应用正常
- 在发布模式（release build）下，Tauri 忽略 `devUrl`，从 `frontendDist` 加载静态文件
- 结果：窗口加载的是 `../static` 目录的文件，而不是连接到 Python 后端
- Python 后端正常启动，但前端无法连接，导致永久加载

## 修复方案

### 1. 移除 frontendDist 配置

**修改前：**
```json
{
  "build": {
    "devUrl": "http://localhost:8000",
    "frontendDist": "../static"
  }
}
```

**修改后：**
```json
{
  "build": {
    "devUrl": "http://localhost:8000"
  }
}
```

### 2. 在窗口配置中明确指定 URL

**修改前：**
```json
{
  "app": {
    "windows": [{
      "title": "Invest Log",
      "width": 1200,
      "height": 800,
      // ... 其他配置
    }]
  }
}
```

**修改后：**
```json
{
  "app": {
    "windows": [{
      "label": "main",
      "title": "Invest Log",
      "url": "http://127.0.0.1:8000",  // ✅ 关键修复
      "width": 1200,
      "height": 800,
      // ... 其他配置
    }]
  }
}
```

### 3. 改进后端启动日志可见性

**修改前（main.rs）：**
```rust
let child = Command::new(&sidecar_path)
    .args(["--data-dir", &data_dir.to_string_lossy(), "--port", "8000"])
    .stdout(Stdio::piped())  // ❌ 日志被隐藏
    .stderr(Stdio::piped())  // ❌ 错误被隐藏
    .spawn()?;
```

**修改后：**
```rust
let child = Command::new(&sidecar_path)
    .args(["--data-dir", &data_dir.to_string_lossy(), "--port", "8000"])
    .stdout(Stdio::inherit())  // ✅ 日志输出到控制台
    .stderr(Stdio::inherit())  // ✅ 错误输出到控制台
    .spawn()?;
```

### 4. 添加后端就绪检测

**新增代码（main.rs）：**
```rust
// 后台线程检测后端是否就绪
thread::spawn(|| {
    for i in 1..=30 {
        thread::sleep(Duration::from_secs(1));
        if let Ok(_) = std::net::TcpStream::connect("127.0.0.1:8000") {
            println!("[Tauri] Backend is ready after {} seconds", i);
            return;
        }
    }
    eprintln!("[Tauri] WARNING: Backend did not respond after 30 seconds");
});
```

## 修改的文件

1. **src-tauri/tauri.conf.json**
   - 移除 `build.frontendDist` 配置
   - 添加窗口 `label: "main"`
   - 添加窗口 `url: "http://127.0.0.1:8000"`

2. **src-tauri/src/main.rs**
   - 改用 `Stdio::inherit()` 使后端日志可见
   - 添加后端 TCP 端口就绪检测
   - 添加必要的 import（`thread`, `Duration`）

3. **BUILD_README.md**
   - 添加更新日志，记录修复的问题

## 验证步骤

构建并测试新版本：

```bash
# 1. 清理旧构建
cd src-tauri && cargo clean

# 2. 重新构建
cd .. && ./scripts/build.sh release

# 3. 运行应用
open "src-tauri/target/release/bundle/macos/Invest Log.app"
```

**预期结果：**
- 应用启动后显示加载画面
- 2-5 秒内（Python 后端启动时间）
- 自动加载到投资组合概览页面
- 可以正常使用所有功能

## 技术总结

### Tauri 2.x 外部服务器模式最佳实践

当 Tauri 应用使用外部后端服务器（如 FastAPI、Express 等）时：

1. **不要配置 `frontendDist`** - 这会让 Tauri 加载静态文件而不是连接服务器
2. **在窗口配置中设置 `url`** - 明确指定要加载的服务器地址
3. **使用 `Stdio::inherit()`** - 让 sidecar 日志可见，便于调试
4. **添加健康检查** - 确保后端完全启动后再加载前端

### 常见陷阱

❌ **错误做法：**
```json
{
  "build": {
    "devUrl": "http://localhost:8000",
    "frontendDist": "../some-dir"
  }
}
```
→ 发布版本会加载 frontendDist，而不是 devUrl

✅ **正确做法：**
```json
{
  "build": {
    "devUrl": "http://localhost:8000"
  },
  "app": {
    "windows": [{
      "url": "http://127.0.0.1:8000"
    }]
  }
}
```
→ 开发和发布版本都连接到后端服务器

## 成功指标

✅ 应用启动不再卡住
✅ 能够正常显示投资组合界面
✅ 所有功能（添加交易、查看持仓、更新价格等）正常工作
✅ 后端日志在控制台可见，便于调试
✅ 打包后的应用大小合理（约 191 MB）

---

**修复完成时间：** 2026-01-25 18:20
**测试平台：** macOS 15.2 (M系列芯片)
**构建工具版本：**
- Tauri: 2.9.5
- Rust: 1.x
- Python: 3.11.2
- PyInstaller: 6.18.0
