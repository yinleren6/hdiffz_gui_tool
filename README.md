~~问 AI 写的 BUG 巨多，难道是因为 AI 自己写的吗？~~

用命令行拉扯文件路径总有点不方便，所以写了个 GUI 工具。直接拖进来就完事了。

[HDiffPatch](https://github.com/sisong/HDiffPatch) 是一个C\C++库和命令行工具，用于在二进制文件或文件夹之间执行 diff(创建补丁) 和 patch(打补丁)；跨平台、运行速度快、创建的补丁小、支持巨大的文件并且diff和patch时都可以控制内存占用量。