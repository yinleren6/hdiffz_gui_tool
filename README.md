# HDiffPatch GUI Tool

## 介绍

HDiffPatch GUI Tool 是使用 Go 语言编写的一个 GUI 工具。能方便的调用 HDiffPatch 工具，支持拖拽文件路径。
用命令行拉扯文件路径总有点不方便，所以写了个 GUI 工具。直接拖进来就完事了。支持 Windows 10/11 64位系统。

A GUI tool written in Go language. It can call HDiffPatch tool easily, and supports dragging file paths.

~~问 AI 写的 BUG 巨多~~

## 使用方法

- 使用以下命令编译：
  `go install github.com/lxn/walk@latest`
  `go install github.com/akavel/rsrc@latest`
  `rsrc -manifest main.manifest -o rsrc.syso`
  `go build -ldflags "-H=windowsgui"`
- 或者直接下载编译好的可执行文件 hdiffz-gui.exe 放到 hdiffz.exe (下载地址: [HDiffPatch](https://github.com/sisong/HDiffPatch)) 同一目录下，双击运行 hdiffz-gui.exe 即可。

## 鸣谢

使用到以下项目的源码或者二进制文件，感谢开源社区的力量，排名不分先后：

- [go](https://github.com/golang/go)
- [walk](https://github.com/lxn/walk)
- [rsrc](https://github.com/akavel/rsrc)
- [HDiffPatch](https://github.com/sisong/HDiffPatch)

---

## 更新日志 (Changelog)

v0.5

- 修复UI界面控件对齐问题
- 编译: 移除 DWARF 信息(-w)和符号表(-s), 缩小了编译体积

v0.4

- 添加了任务进度条动画,避免界面卡住无响应
- 修复子进程管道 gbk 乱码问题
- bug 修复

v0.3

- 新增检查文件MD5值功能
- bug 修复

v0.2

- 修复中文路径乱码问题
- bug 修复

v0.1

- 初版发布
