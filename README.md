# FileFeed (MyGo / Go 版)

基于 **MyGo** 的 Windows 桌面版「新增文件看板」。无需 Python，安装包附带会话读取与归档组件 `ff.exe`。

## 成品

```
build\windows-amd64\FileFeed.exe                      ← 双击即用
build\windows-amd64\ff.exe                            ← Agent 会话归档 CLI
build\windows-amd64\filefeed-setup-0.1.0-amd64.exe    ← 安装包
```

首次运行会把内置的 `es.exe` 释放到 `%LOCALAPPDATA%\FileFeed\es.exe`，
配置存到 `%LOCALAPPDATA%\FileFeed\config.json`。

## 依赖

- **Everything 1.4+** 必须在运行（程序通过它的 `es.exe` 拿全盘索引）
- WebView2 运行时（Win10/11 一般自带）

## 功能

- 类型多选：任意扩展名，可一键切换到「文档 / 代码 / 图片 / 视频 / 音频 / 压缩包」分类。各分类包含的扩展名可在设置里勾选，也可以添加自己的扩展名。
- 图片可按比例、像素分辨率和文件大小筛选，视频可按文件大小和时长筛选。需要读取属性时，结果栏会显示缺少相应属性的文件数。
- 多格式结果：自适应瀑布流，可跳转到任意格式并独立收起卡片。展开的卡片内可滚动浏览该格式的结果。
- 时间筛选：过去 N 分钟 / 小时 / 天 / 周，或自定义区间（精确到分钟）
- 时间依据：按创建时间 或 按修改时间
- 关键词：匹配文件名与路径
- 排序：时间 / 大小 / 名称
- 每行操作：**打开 / 位置 / 路径 / 文件 / 删除**
  - 路径 = 复制路径文本
  - 文件 = 复制文件本身（CF_HDROP，可直接粘贴到文件夹）
- 顶部「刷新」按钮 + Ctrl+R
- 筛选方案：在类型筛选下方直接点方案名切换，通过「管理方案」保存当前条件或删除方案
- 自动刷新：默认每 60 秒，可在设置中选择 5、10、15、30 或 60 秒

## Agent 会话

打开顶部「Agent 会话」，可查看自动发现的本地记录，按 Agent 或标题、目录、话题、会话 ID 筛选，展开正文并导出 Markdown。读取器沿用 Magpie 的格式支持范围。部分来源仅提供会话元数据，页面会显示正文是否可读。会话由本机读取，归档保存在本机。

勾选「对话分类」后，Cindy 的 Telegram 群会按「群 → 话题 → 会话」展开成树。话题 ID 从会话记录和 Cindy 的运行时目录读取；话题名称只在记录里出现时自动取得。其余名称可写入 `%LOCALAPPDATA%\FileFeed\AgentTopics.json`，键为 `<chat_id>#<话题 ID>`，例如 `"-1003901021780#215": "日本影视作品"`。

每个分组右侧的「导出」可一次导出该组全部会话：Cindy 群会包含组内所有话题，其他 Agent 会包含该工作目录下的所有对话。勾选「纯净导出」只保留问答正文；勾选「导出为 zip」会把多个 Markdown 打成压缩包，并按话题建立子目录。未勾选 zip 时选择导出目录，文件同样按话题分目录保存。

列表左上角的「全部折叠 / 全部展开」可一键收起或展开所有分组。详情右上角的「纯净显示」是独立的阅读模式：把正文显示成左右气泡，只保留用户和 Agent 的文字，引用内容照常显示，思考、工具调用和工具结果隐藏；它不影响 Markdown 导出。

导出单条会话时，勾选「纯净显示」可按问答导出正文，省略思考、工具调用、工具结果和附件占位。未勾选时导出完整 Markdown。
默认文件名使用 Agent 名称与本地导出时间（精确到分钟），例如 `pi_202611211523_filefeed_clean.md` 或 `claudecode_202611110021_filefeed.md`。

在「归档日期」选一天，再点「保存当天会话」，也可以点「复制命令」取得带完整路径的 PowerShell 命令。在 `ff.exe` 所在目录可直接运行：

```powershell
.\ff.exe ff-cindy-20261122
.\ff.exe save all 20261122 --output 'D:\AgentArchives'
```

日期按 Windows 本地时区计算。归档选择当天开始、最后活动或有消息及调用记录的会话，保存这些会话的完整 Markdown 以保留上下文，另外写入 `manifest.json` 记录匹配结果和读取失败原因。默认路径为 `%LOCALAPPDATA%\FileFeed\AgentArchives\ff-<agent>-<YYYYMMDD>.zip`。重复保存会在新归档写完后更新同名文件。CLI 部分导出时保留 ZIP 并返回非零退出码。

## 开发

```bash
npm ci
npx mygo dev      # 开发预览（热更新）
npx mygo build    # 打包
```

改 Go 结构体后要跑 `npx mygo generate` 重新生成 `src/mygo.ts`。

运行 `node tools/ff/build.mjs --test` 可验收会话归档及纯净导出。

## 性能要点

全盘 64 万个候选文件，直接 stat 要 40 秒。做法是：

1. 先用 Everything 的 **`rc:`（最近变更，走 USN 索引）** 粗筛，64 万 → 几千条，0.1 秒
2. 再在 Go 里精确过滤创建/修改时间

实测 **180–220 ms**（2119 条结果）。

## Windows 高 DPI

MyGo 的 WebView2 采用物理像素作为窗口边界，直接使用 `GetClientRect` 返回的尺寸。旧版的额外 DPI 换算会缩小 WebView2 的覆盖区域，已恢复 MyGo v0.10.0 的实现。

## 第三方组件

`assets/es.exe` 来自 voidtools 的 [Everything 命令行工具 ES 1.1.0.27](https://www.voidtools.com/ES-1.1.0.27.x86.zip)。许可文本见 [ES-LICENSE.txt](resources/ES-LICENSE.txt)。

媒体属性由随程序打包的 [MediaInfo 26.10](https://mediaarea.net/en/MediaInfo/Download/Windows) 在本地读取。许可文本见 [MediaInfo-LICENSE.html](resources/MediaInfo-LICENSE.html)。

会话发现与解析使用 [Magpie v0.1.4662](https://github.com/yetone/magpie/tree/v0.1.4662)。MIT 许可文本见 [Magpie-LICENSE.txt](resources/Magpie-LICENSE.txt)。
