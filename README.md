# FileFeed (MyGo / Go 版)

基于 **MyGo** 的 Windows 桌面版「新增文件看板」。单文件 exe，无需 Python。

## 成品

```
build\windows-amd64\FileFeed.exe                      ← 双击即用
build\windows-amd64\filefeed-setup-0.1.0-amd64.exe    ← 安装包
```

首次运行会把内置的 `es.exe` 释放到 `%LOCALAPPDATA%\FileFeed\es.exe`，
配置存到 `%LOCALAPPDATA%\FileFeed\config.json`。

## 依赖

- **Everything 1.4+** 必须在运行（程序通过它的 `es.exe` 拿全盘索引）
- WebView2 运行时（Win10/11 一般自带）

## 功能

- 类型多选：任意扩展名，可一键切换到「文档 / 代码 / 图片 / 视频 / 音频 / 压缩包」分类
- 多格式结果：自适应瀑布流，可跳转到任意格式并独立收起卡片。展开的卡片内可滚动浏览该格式的结果。
- 时间筛选：过去 N 分钟 / 小时 / 天 / 周，或自定义区间（精确到分钟）
- 时间依据：按创建时间 或 按修改时间
- 关键词：匹配文件名与路径
- 排序：时间 / 大小 / 名称
- 每行操作：**打开 / 位置 / 路径 / 文件 / 删除**
  - 路径 = 复制路径文本
  - 文件 = 复制文件本身（CF_HDROP，可直接粘贴到文件夹）
- 顶部「刷新」按钮 + Ctrl+R
- 筛选方案：把当前条件存成方案，一键切换
- 自动刷新：默认每 60 秒，可在设置中选择 5、10、15、30 或 60 秒

## 开发

```bash
npm ci
npx mygo dev      # 开发预览（热更新）
npx mygo build    # 打包
```

改 Go 结构体后要跑 `npx mygo generate` 重新生成 `src/mygo.ts`。

## 性能要点

全盘 64 万个候选文件，直接 stat 要 40 秒。做法是：

1. 先用 Everything 的 **`rc:`（最近变更，走 USN 索引）** 粗筛，64 万 → 几千条，0.1 秒
2. 再在 Go 里精确过滤创建/修改时间

实测 **180–220 ms**（2119 条结果）。

## Windows 高 DPI

MyGo 的 WebView2 采用物理像素作为窗口边界，直接使用 `GetClientRect` 返回的尺寸。旧版的额外 DPI 换算会缩小 WebView2 的覆盖区域，已恢复 MyGo v0.10.0 的实现。

## 第三方组件

`assets/es.exe` 来自 voidtools 的 [Everything 命令行工具 ES 1.1.0.27](https://www.voidtools.com/ES-1.1.0.27.x86.zip)。许可文本见 [ES-LICENSE.txt](resources/ES-LICENSE.txt)。
