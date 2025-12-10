package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

type FileType int

const (
	FileTypeUnknown FileType = iota
	FileTypeFile
	FileTypeDirectory
)

type PatchTab struct {
	TabPage            *walk.TabPage
	OldPathEdit        *walk.LineEdit
	NewPathEdit        *walk.LineEdit
	PatchPathEdit      *walk.LineEdit
	CreatePatchBtn     *walk.PushButton
	VerifyPatchBtn     *walk.PushButton
	OverwriteCheck     *walk.CheckBox
	CompressCheck      *walk.CheckBox
	SkipVerifyCheck    *walk.CheckBox
	LogTextEdit        *walk.TextEdit
	SelectOldBtn       *walk.PushButton
	SelectOldFolderBtn *walk.PushButton
	SelectNewBtn       *walk.PushButton
	SelectNewFolderBtn *walk.PushButton
	SelectPatchBtn     *walk.PushButton
	OldPathLabel       *walk.Label
	NewPathLabel       *walk.Label
	PatchPathLabel     *walk.Label
	OldPathType        FileType
	NewPathType        FileType
	AutoPatchName      string
}

type ApplyTab struct {
	TabPage            *walk.TabPage
	OldPathEdit        *walk.LineEdit
	PatchPathEdit      *walk.LineEdit
	NewPathEdit        *walk.LineEdit
	ApplyPatchBtn      *walk.PushButton
	VerifyApplyBtn     *walk.PushButton
	OverwriteCheck     *walk.CheckBox
	SkipVerifyCheck    *walk.CheckBox
	LogTextEdit        *walk.TextEdit
	SelectOldBtn       *walk.PushButton
	SelectOldFolderBtn *walk.PushButton
	SelectPatchBtn     *walk.PushButton
	SelectNewBtn       *walk.PushButton
	SelectNewFolderBtn *walk.PushButton
	OldPathLabel       *walk.Label
	PatchPathLabel     *walk.Label
	NewPathLabel       *walk.Label
	OldPathType        FileType
	PatchPathType      FileType
	AutoPatchName      string
}

type AppMainWindow struct {
	*walk.MainWindow
	TabWidget *walk.TabWidget
	PatchTab  *PatchTab
	ApplyTab  *ApplyTab
	LogMutex  sync.Mutex
}

var HdiffzPath string
var Cp uintptr

func (mw *AppMainWindow) log(text string) {
	mw.LogMutex.Lock()
	defer mw.LogMutex.Unlock()

	now := time.Now().Format("11:45:14")
	// 使用 Windows 风格换行符并添加空行，保证在 TextEdit 中正确显示
	logLine := fmt.Sprintf("[%s] %s\r\n", now, text)

	// UI 更新必须在主线程执行
	mw.Synchronize(func() {
		var logEdit *walk.TextEdit
		if mw.TabWidget.CurrentIndex() == 0 {
			logEdit = mw.PatchTab.LogTextEdit
		} else {
			logEdit = mw.ApplyTab.LogTextEdit
		}

		if logEdit != nil {
			currentText := logEdit.Text()
			logEdit.SetText(currentText + logLine)
		}
	})
}

// GBK -> UTF-8
func GbkToUtf8(s []byte) ([]byte, error) {
	if Cp == 936 {
		return s, nil
	}
	reader := transform.NewReader(bytes.NewReader(s), simplifiedchinese.GBK.NewDecoder())
	all, err := io.ReadAll(reader)
	if err != nil {
		return all, err
	}
	return all, nil
}

// // UTF-8 -> GBK
// func Utf8ToGbk(s []byte) ([]byte, error) {
// 	reader := transform.NewReader(bytes.NewReader(s), simplifiedchinese.GBK.NewEncoder())
// 	all, err := io.ReadAll(reader)
// 	if err != nil {
// 		return all, err
// 	}
// 	return all, nil
// }

func (mw *AppMainWindow) executeCommand(args []string) {
	// 确定 hdiffz 可执行文件路径：收集候选路径并记录检查结果，便于排查
	toolPath := ""
	checked := []string{}
	// 将工作目录切换到可执行文件所在目录，保证双击启动时能找到同目录的 hdiffz.exe
	if exe, err := os.Executable(); err == nil {
		if dir := filepath.Dir(exe); dir != "" {
			fmt.Println("work_dir: " + dir)
			_ = os.Chdir(dir)
		}
		// 预缓存同目录的 hdiffz.exe（如果存在），避免首次运行时提示
		toolPath = filepath.Join(filepath.Dir(exe), "hdiffz.exe")
		fmt.Println("toolPath: ", toolPath)
		if _, err_file_stat := os.Stat(toolPath); err_file_stat == nil {
			// hdiffz.exe 路径
			HdiffzPath = toolPath
			fmt.Println("hdiffz.exe_path: " + HdiffzPath)
		} else if os.IsNotExist(err_file_stat) {
			fmt.Println("error 错误:", err_file_stat)
			//mw.log("错误: 未找到 hdiffz.exe 工具")
		} else {
			fmt.Println("error:", err_file_stat)
			//mw.log("错误: 未找到 hdiffz.exe 工具")
			//return
		}
	}

	// 另外把检查结果写入工作目录下的文件，方便在终端/文件管理器中查看
	go func() {
		var outDir string
		if wd, err := os.Getwd(); err == nil {
			outDir = wd
		} else {
			outDir = os.TempDir()
		}
		logPath := filepath.Join(outDir, "hdiffz_check.log")
		data := strings.Join(checked, "\r\n") + "\r\n"

		// 如果 checked 为空，追加更多诊断信息：工作目录、目录列表、可执行所在目录和 PATH
		if len(checked) == 0 {
			var b strings.Builder
			if wd, err := os.Getwd(); err == nil {
				b.WriteString("WorkingDir: ")
				b.WriteString(wd)
				b.WriteString("\r\n")
				if entries, err := os.ReadDir(wd); err == nil {
					b.WriteString("Dir listing:\r\n")
					for _, e := range entries {
						b.WriteString("  ")
						b.WriteString(e.Name())
						if e.IsDir() {
							b.WriteString("/\r\n")
						} else {
							b.WriteString("\r\n")
						}
					}
				}
			}
			if exe, err := os.Executable(); err == nil {
				b.WriteString("ExecutableDir: ")
				b.WriteString(filepath.Dir(exe))
				b.WriteString("\r\n")
			}

			data = data + "\r\nDIAGNOSTICS:\r\n" + b.String()
		}

		// 追加模式
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			f.WriteString(time.Now().Format("2006-01-02 15:04:05") + "\r\n")
			f.WriteString(data)
			f.WriteString("----\r\n")
		}
	}()

	go func() {
		cmd := exec.Command(HdiffzPath, args...)

		if HdiffzPath == "" {
			mw.log("错误: 当前目录下未找到 hdiffz.exe 可执行文件")
			return
		}

		mw.log(fmt.Sprintf("执行命令: %s %s", HdiffzPath, strings.Join(args, " ")))
		mw.log(fmt.Sprintln("Processing..."))

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			mw.log(fmt.Sprintf("错误: 创建输出管道失败 - %v", err))
			return
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			mw.log(fmt.Sprintf("错误: 创建错误管道失败 - %v", err))
			return
		}

		if err := cmd.Start(); err != nil {
			mw.log(fmt.Sprintf("错误: 启动进程失败 - %v", err))
			return
		}

		// 读取输出
		output, _ := io.ReadAll(stdout)
		errorOutput, _ := io.ReadAll(stderr)

		if err := cmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					mw.log(fmt.Sprintf("进程退出，返回码: %d", status.ExitStatus()))
				}
			}
		}

		// 显示输出
		if len(output) > 0 {

			output, _ := GbkToUtf8(output)
			mw.log("\r\n>>>>>\r\n" + strings.TrimSpace(string(output)) + "\r\n<<<<<")
		}
		if len(errorOutput) > 0 {
			errorOutput, _ := GbkToUtf8(errorOutput)
			mw.log("错误输出: " + strings.TrimSpace(string(errorOutput)))
		}

		//TODO: 重命名输出文件，备份原文件
	}()

}

func (mw *AppMainWindow) getPathType(path string) FileType {
	if path == "" {
		return FileTypeUnknown
	}

	info, err := os.Stat(path)
	if err != nil {
		return FileTypeUnknown
	}

	if info.IsDir() {
		return FileTypeDirectory
	}
	return FileTypeFile
}

func (mw *AppMainWindow) updatePatchName() {
	if mw.PatchTab.OldPathEdit.Text() == "" || mw.PatchTab.NewPathEdit.Text() == "" {
		return
	}

	oldPath := mw.PatchTab.OldPathEdit.Text()
	oldName := filepath.Base(oldPath)
	ext := filepath.Ext(oldName)
	baseName := strings.TrimSuffix(oldName, ext)

	patchName := baseName + "_patch.diff"
	mw.PatchTab.AutoPatchName = patchName

	// 如果当前没有手动修改过补丁路径，则自动设置
	currentPatch := mw.PatchTab.PatchPathEdit.Text()
	if currentPatch == "" || currentPatch == mw.PatchTab.AutoPatchName {
		dir := filepath.Dir(oldPath)
		mw.PatchTab.PatchPathEdit.SetText(filepath.Join(dir, patchName))
	}
}

func (mw *AppMainWindow) updateApplyName() {
	if mw.ApplyTab.OldPathEdit.Text() == "" || mw.ApplyTab.PatchPathEdit.Text() == "" {
		return
	}

	oldPath := mw.ApplyTab.OldPathEdit.Text()
	oldName := filepath.Base(oldPath)
	ext := filepath.Ext(oldName)
	baseName := strings.TrimSuffix(oldName, ext)

	newName := baseName + "_new" + ext
	mw.ApplyTab.AutoPatchName = newName

	// 如果当前没有手动修改过新文件路径，则自动设置
	currentNew := mw.ApplyTab.NewPathEdit.Text()
	if currentNew == "" || currentNew == mw.ApplyTab.AutoPatchName {
		dir := filepath.Dir(oldPath)
		mw.ApplyTab.NewPathEdit.SetText(filepath.Join(dir, newName))
	}
}

func (mw *AppMainWindow) createPatch() {

	// 检查路径
	if mw.PatchTab.NewPathEdit.Text() == "" {
		mw.log("错误: 请选择新文件/文件夹路径")
		return
	}

	oldPath := mw.PatchTab.OldPathEdit.Text()
	newPath := mw.PatchTab.NewPathEdit.Text()
	patchPath := mw.PatchTab.PatchPathEdit.Text()

	if patchPath == "" {
		mw.log("错误: 请指定补丁文件输出路径")
		return
	}

	// 检查路径类型一致性
	oldType := mw.getPathType(oldPath)
	newType := mw.getPathType(newPath)

	if oldType != FileTypeUnknown && newType != FileTypeUnknown && oldType != newType {
		mw.log("错误: 旧路径和新路径必须是相同的类型（都是文件或都是文件夹）")
		return
	}

	// 构建参数
	args := []string{}
	if mw.PatchTab.CompressCheck.Checked() {
		args = append(args, "-c-zstd-21-24")
	}
	if mw.PatchTab.OverwriteCheck.Checked() {
		args = append(args, "-f")
	}
	if mw.PatchTab.SkipVerifyCheck.Checked() {
		args = append(args, "-d")
	}

	// 添加路径参数
	if oldPath != "" {
		args = append(args, oldPath)
	}
	args = append(args, newPath, patchPath)

	mw.executeCommand(args)
	for _, rec := range args {
		mw.log("args: " + rec)
	}

}

func (mw *AppMainWindow) verifyPatch() {

	oldPath := mw.PatchTab.OldPathEdit.Text()
	newPath := mw.PatchTab.NewPathEdit.Text()
	patchPath := mw.PatchTab.PatchPathEdit.Text()

	if oldPath == "" || newPath == "" || patchPath == "" {
		mw.log("错误: 请填写所有必要的路径")
		return
	}

	args := []string{"-t", oldPath, newPath, patchPath}

	mw.executeCommand(args)
}

func (mw *AppMainWindow) applyPatch() {

	// 检查路径
	if mw.ApplyTab.OldPathEdit.Text() == "" || mw.ApplyTab.PatchPathEdit.Text() == "" {
		mw.log("错误: 请选择旧文件和补丁文件路径")
		return
	}

	oldPath := mw.ApplyTab.OldPathEdit.Text()
	patchPath := mw.ApplyTab.PatchPathEdit.Text()
	newPath := mw.ApplyTab.NewPathEdit.Text()

	if newPath == "" {
		mw.log("错误: 请指定新文件输出路径")
		return
	}

	// 构建参数
	args := []string{}
	args = append(args, "--patch")
	if mw.ApplyTab.OverwriteCheck.Checked() {
		args = append(args, "-f")
	}

	// 添加路径参数
	args = append(args, oldPath, patchPath, newPath)

	mw.executeCommand(args)
}

func (mw *AppMainWindow) selectFile(edit *walk.LineEdit, title, filter string) {

	dlg := new(walk.FileDialog)
	dlg.Title = title
	dlg.Filter = filter

	if ok, _ := dlg.ShowOpen(mw); ok {
		if dlg.FilePath != "" && edit != nil {
			edit.SetText(dlg.FilePath)
		}
	}
}

func (mw *AppMainWindow) selectFolder(edit *walk.LineEdit, title string) {

	// 使用 Win32 SHBrowseForFolderW + SHGetPathFromIDListW 实现文件夹选择
	shell32 := syscall.NewLazyDLL("shell32.dll")
	procSHBrowseForFolder := shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDList := shell32.NewProc("SHGetPathFromIDListW")
	ole32 := syscall.NewLazyDLL("ole32.dll")
	procCoTaskMemFree := ole32.NewProc("CoTaskMemFree")

	// prepare BROWSEINFO struct
	type browseInfo struct {
		hwndOwner      uintptr
		pidlRoot       uintptr
		pszDisplayName uintptr
		lpszTitle      uintptr
		ulFlags        uint32
		lpfn           uintptr
		lParam         uintptr
		iImage         int32
	}

	titlePtr, _ := syscall.UTF16PtrFromString(title)
	var display [syscall.MAX_PATH]uint16
	bi := browseInfo{
		hwndOwner:      uintptr(mw.MainWindow.Handle()),
		pidlRoot:       0,
		pszDisplayName: uintptr(unsafe.Pointer(&display[0])),
		lpszTitle:      uintptr(unsafe.Pointer(titlePtr)),
		ulFlags:        0x00000001 | 0x00000040, // BIF_RETURNONLYFSDIRS | BIF_NEWDIALOGSTYLE
		lpfn:           0,
		lParam:         0,
	}

	ret, _, _ := procSHBrowseForFolder.Call(uintptr(unsafe.Pointer(&bi)))
	if ret == 0 {
		return
	}
	pidl := ret

	// prepare buffer for path
	var pathBuf [syscall.MAX_PATH]uint16
	ok, _, _ := procSHGetPathFromIDList.Call(pidl, uintptr(unsafe.Pointer(&pathBuf[0])))
	if ok == 0 {
		// free pidl
		procCoTaskMemFree.Call(pidl)
		mw.log("错误: 无法从 IDList 获取路径")
		return
	}

	// free pidl
	procCoTaskMemFree.Call(pidl)

	path := syscall.UTF16ToString(pathBuf[:])
	if path != "" && edit != nil {
		edit.SetText(path)
	}
}

func (mw *AppMainWindow) selectSaveFile(edit *walk.LineEdit, title, filter string) {

	dlg := new(walk.FileDialog)
	dlg.Title = title
	dlg.Filter = filter

	if ok, _ := dlg.ShowSave(mw); ok {
		if dlg.FilePath != "" && edit != nil {
			edit.SetText(dlg.FilePath)
		}
	}
}

func (mw *AppMainWindow) handleDropFiles(files []string) {

	if len(files) == 0 {
		return
	}

	// 获取当前激活的Tab
	currentIndex := mw.TabWidget.CurrentIndex()
	path := files[0]
	// 使用 GetCursorPos 获取鼠标位置，然后对每个已知 LineEdit 使用 GetWindowRect
	// 来判断鼠标是否位于该控件之上（更可靠，避免 WindowFromPoint 在复杂控件层次中失效）。
	user32 := syscall.NewLazyDLL("user32.dll")
	procGetCursorPos := user32.NewProc("GetCursorPos")
	procGetWindowRect := user32.NewProc("GetWindowRect")

	var pt struct{ X, Y int32 }
	if r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt))); r != 0 {
		// RECT 定义
		type rect struct{ Left, Top, Right, Bottom int32 }

		isPointInWindow := func(target walk.Window) bool {
			if target == nil {
				return false
			}
			hwnd := uintptr(target.Handle())
			var r rect
			ret, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
			if ret == 0 {
				return false
			}
			return pt.X >= r.Left && pt.X <= r.Right && pt.Y >= r.Top && pt.Y <= r.Bottom
		}

		// 检查生成补丁 Tab 的控件
		if currentIndex == 0 {
			if isPointInWindow(mw.PatchTab.OldPathEdit) {
				mw.Synchronize(func() { mw.PatchTab.OldPathEdit.SetText(path) })
				mw.log(fmt.Sprintf("拖放文件: %s -> 旧路径", path))
				return
			}
			if isPointInWindow(mw.PatchTab.NewPathEdit) {
				mw.Synchronize(func() { mw.PatchTab.NewPathEdit.SetText(path) })
				mw.log(fmt.Sprintf("拖放文件: %s -> 新路径", path))
				return
			}
			if isPointInWindow(mw.PatchTab.PatchPathEdit) {
				mw.Synchronize(func() { mw.PatchTab.PatchPathEdit.SetText(path) })
				mw.log(fmt.Sprintf("拖放文件: %s -> 补丁路径", path))
				return
			}
		} else {
			// 应用补丁 Tab
			if isPointInWindow(mw.ApplyTab.OldPathEdit) {
				mw.Synchronize(func() { mw.ApplyTab.OldPathEdit.SetText(path) })
				mw.log(fmt.Sprintf("拖放文件: %s -> 旧路径", path))
				return
			}
			if isPointInWindow(mw.ApplyTab.PatchPathEdit) {
				mw.Synchronize(func() { mw.ApplyTab.PatchPathEdit.SetText(path) })
				mw.log(fmt.Sprintf("拖放文件: %s -> 补丁路径", path))
				return
			}
			if isPointInWindow(mw.ApplyTab.NewPathEdit) {
				mw.Synchronize(func() { mw.ApplyTab.NewPathEdit.SetText(path) })
				mw.log(fmt.Sprintf("拖放文件: %s -> 新路径", path))
				return
			}
		}
	}

}

func main() {

	mw := &AppMainWindow{}
	mw.PatchTab = &PatchTab{}
	mw.ApplyTab = &ApplyTab{}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleCP := kernel32.NewProc("GetConsoleOutputCP")
	Cp, _, _ = getConsoleCP.Call()
	fmt.Println("console_cp:", Cp)
	// 创建主窗口
	w := MainWindow{
		AssignTo: &mw.MainWindow,
		Title:    "HDiffz GUI 工具",
		MinSize:  Size{Width: 800, Height: 600},
		Size:     Size{Width: 800, Height: 600},
		Layout:   VBox{},
		Children: []Widget{
			TabWidget{
				AssignTo: &mw.TabWidget,
				Pages: []TabPage{
					// 第一个Tab：生成补丁
					TabPage{
						Title:  "生成补丁",
						Layout: VBox{},
						DataBinder: DataBinder{
							DataSource: mw,
						},
						Children: []Widget{
							Composite{
								Layout: Grid{Columns: 4, Spacing: 10},
								Children: []Widget{
									Label{
										Text: "旧文件/文件夹:",
									},
									LineEdit{
										AssignTo: &mw.PatchTab.OldPathEdit,
										OnTextChanged: func() {
											mw.updatePatchName()
										},
									},
									Composite{
										Layout: HBox{},
										Children: []Widget{
											PushButton{
												AssignTo: &mw.PatchTab.SelectOldBtn,
												Text:     "文件...",
												OnClicked: func() {
													mw.selectFile(mw.PatchTab.OldPathEdit, "选择旧文件", "所有文件 (*.*)|*.*")
												},
											},
											PushButton{
												AssignTo: &mw.PatchTab.SelectOldFolderBtn,
												Text:     "文件夹...",
												OnClicked: func() {
													mw.selectFolder(mw.PatchTab.OldPathEdit, "选择旧文件夹")
												},
											},
										},
									},
									Label{
										AssignTo: &mw.PatchTab.OldPathLabel,
										Text:     "",
									},

									Label{
										Text: "新文件/文件夹:",
									},
									LineEdit{
										AssignTo: &mw.PatchTab.NewPathEdit,
										OnTextChanged: func() {
											mw.updatePatchName()
										},
									},
									Composite{
										Layout: HBox{},
										Children: []Widget{
											PushButton{
												AssignTo: &mw.PatchTab.SelectNewBtn,
												Text:     "文件...",
												OnClicked: func() {
													mw.selectFile(mw.PatchTab.NewPathEdit, "选择新文件", "所有文件 (*.*)|*.*")
												},
											},
											PushButton{
												AssignTo: &mw.PatchTab.SelectNewFolderBtn,
												Text:     "文件夹...",
												OnClicked: func() {
													mw.selectFolder(mw.PatchTab.NewPathEdit, "选择新文件夹")
												},
											},
										},
									},
									Label{
										AssignTo: &mw.PatchTab.NewPathLabel,
										Text:     "",
									},

									Label{
										Text: "补丁文件:",
									},
									LineEdit{
										AssignTo: &mw.PatchTab.PatchPathEdit,
									},
									PushButton{
										AssignTo: &mw.PatchTab.SelectPatchBtn,
										Text:     "选择...",
										OnClicked: func() {
											mw.selectSaveFile(mw.PatchTab.PatchPathEdit, "选择补丁文件", "补丁文件 (*.diff)|*.diff")
										},
									},
									Label{
										AssignTo: &mw.PatchTab.PatchPathLabel,
										Text:     "",
									},
								},
							},

							Composite{
								Layout: HBox{},
								Children: []Widget{
									CheckBox{
										AssignTo: &mw.PatchTab.OverwriteCheck,
										Text:     "覆盖同名文件 (-f)",
										Checked:  true,
									},
									CheckBox{
										AssignTo: &mw.PatchTab.CompressCheck,
										Text:     "压缩 (-c-zstd-21-24)",
										Checked:  true,
									},
									CheckBox{
										AssignTo: &mw.PatchTab.SkipVerifyCheck,
										Text:     "不要执行patch检查 (-d)",
									},
								},
							},

							Composite{
								Layout: HBox{},
								Children: []Widget{
									PushButton{
										AssignTo: &mw.PatchTab.CreatePatchBtn,
										Text:     "生成补丁",
										OnClicked: func() {
											mw.createPatch()
										},
									},
									PushButton{
										AssignTo: &mw.PatchTab.VerifyPatchBtn,
										Text:     "验证",
										OnClicked: func() {
											mw.verifyPatch()
										},
									},
								},
							},

							TextEdit{
								AssignTo: &mw.PatchTab.LogTextEdit,
								ReadOnly: true,
								HScroll:  true,
								VScroll:  true,
								OnTextChanged: func() {
									mw.PatchTab.LogTextEdit.SendMessage(0x0115, 7, 0)
								},
							},
						},
					},

					// 第二个Tab：应用补丁
					TabPage{
						Title:  "应用补丁",
						Layout: VBox{},
						Children: []Widget{
							Composite{
								Layout: Grid{Columns: 4, Spacing: 10},
								Children: []Widget{
									Label{
										Text: "旧文件/文件夹:",
									},
									LineEdit{
										AssignTo: &mw.ApplyTab.OldPathEdit,
										OnTextChanged: func() {
											mw.updateApplyName()
										},
									},
									Composite{
										Layout: HBox{},
										Children: []Widget{
											PushButton{
												AssignTo: &mw.ApplyTab.SelectOldBtn,
												Text:     "文件...",
												OnClicked: func() {
													mw.selectFile(mw.ApplyTab.OldPathEdit, "选择旧文件", "所有文件 (*.*)|*.*")
												},
											},
											PushButton{
												AssignTo: &mw.ApplyTab.SelectOldFolderBtn,
												Text:     "文件夹...",
												OnClicked: func() {
													mw.selectFolder(mw.ApplyTab.OldPathEdit, "选择旧文件夹")
												},
											},
										},
									},
									Label{
										AssignTo: &mw.ApplyTab.OldPathLabel,
										Text:     "",
									},

									Label{
										Text: "补丁文件:",
									},
									LineEdit{
										AssignTo: &mw.ApplyTab.PatchPathEdit,
										OnTextChanged: func() {
											mw.updateApplyName()
										},
									},
									PushButton{
										AssignTo: &mw.ApplyTab.SelectPatchBtn,
										Text:     "选择...",
										OnClicked: func() {
											mw.selectFile(mw.ApplyTab.PatchPathEdit, "选择补丁文件", "补丁文件 (*.diff)|*.diff")
										},
									},
									Label{
										AssignTo: &mw.ApplyTab.PatchPathLabel,
										Text:     "",
									},

									Label{
										Text: "新文件/文件夹:",
									},
									LineEdit{
										AssignTo: &mw.ApplyTab.NewPathEdit,
									},
									Composite{
										Layout: HBox{},
										Children: []Widget{
											PushButton{
												AssignTo: &mw.ApplyTab.SelectNewBtn,
												Text:     "文件...",
												OnClicked: func() {
													mw.selectFile(mw.ApplyTab.NewPathEdit, "选择新文件输出", "所有文件 (*.*)|*.*")
												},
											},
											PushButton{
												AssignTo: &mw.ApplyTab.SelectNewFolderBtn,
												Text:     "文件夹...",
												OnClicked: func() {
													mw.selectFolder(mw.ApplyTab.NewPathEdit, "选择新文件夹输出")
												},
											},
										},
									},
									Label{
										AssignTo: &mw.ApplyTab.NewPathLabel,
										Text:     "",
									},
								},
							},

							Composite{
								Layout: HBox{},
								Children: []Widget{
									CheckBox{
										AssignTo: &mw.ApplyTab.OverwriteCheck,
										Text:     "覆盖同名文件 (-f)",
										Checked:  false,
									},
								},
							},

							Composite{
								Layout: HBox{},
								Children: []Widget{
									PushButton{
										AssignTo: &mw.ApplyTab.ApplyPatchBtn,
										Text:     "应用补丁",
										OnClicked: func() {
											mw.applyPatch()
										},
									},
								},
							},

							TextEdit{
								AssignTo: &mw.ApplyTab.LogTextEdit,
								ReadOnly: true,
								HScroll:  true,
								VScroll:  true,
								OnTextChanged: func() {
									mw.ApplyTab.LogTextEdit.SendMessage(0x0115, 7, 0)
								},
							},
						},
					},
				},
			},
		},
		OnDropFiles: func(files []string) {
			mw.handleDropFiles(files)
		},
	}

	fmt.Println("Starting Run()")
	ret, err := w.Run()
	fmt.Println("Run() returned code:", ret, "error:", err)
}
