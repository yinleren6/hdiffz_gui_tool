package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
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
	now := time.Now().Format("15:04:05")
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
			logEdit.AppendText(logLine)
		}
	})
}
func FastHashCompare(file1, file2 string) (bool, error) {
	// 1. 首先检查是否是同一个文件（路径相同）
	if file1 == file2 {
		return true, nil
	}

	// 2. 快速检查文件大小（避免不必要的哈希计算）
	info1, err := os.Stat(file1)
	if err != nil {

		return false, err
	}

	info2, err := os.Stat(file2)
	if err != nil {
		return false, err
	}

	if info1.Size() != info2.Size() {
		fmt.Println("大小不相同 ")
		return false, nil
	} else {
		fmt.Println("大小相同 ")
	}
	fmt.Println("继续 ")
	// 3. 使用MD5哈希（最快的主流哈希算法）
	hash1, err := fastMD5(file1)
	if err != nil {
		return false, err
	}

	hash2, err := fastMD5(file2)
	if err != nil {
		return false, err
	}
	fmt.Printf("哈希值1: %s\r\n哈希值2: %s", hash1, hash2)

	return hash1 == hash2, nil
}
func fastMD5(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 使用较大缓冲区（1MB）提高读取速度
	buf := make([]byte, 1024*1024)
	hash := md5.New()

	for {
		n, err := file.Read(buf)
		if n > 0 {
			hash.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
func (mw *AppMainWindow) BenchmarkCompare(file1, file2 string) {
	start := time.Now()
	same, err := FastHashCompare(file1, file2)

	if err != nil {
		fmt.Printf("比较出错: %v\n", err)
		return
	}

	if same {

		mw.log(fmt.Sprintf("\r\n两个文件内容相同  [%s] == [%s]\r\n", filepath.Base(file1), filepath.Base(file2)))
		fmt.Println("两个文件相同")
	} else {

		fmt.Println("两个文件不同")
	}
	elapsed := time.Since(start)
	fmt.Printf("耗时: %v\n", elapsed)
}
func (mw *AppMainWindow) compare() {
	oldPath := mw.PatchTab.OldPathEdit.Text()
	newPath := mw.PatchTab.NewPathEdit.Text()

	if oldPath != "" && newPath != "" {
		if mw.getPathType(oldPath) == FileTypeFile && mw.getPathType(newPath) == FileTypeFile {
			mw.BenchmarkCompare(oldPath, newPath)
		}
	}
}

func FastHashCompare(file1, file2 string) (bool, error) {
	// 1. 首先检查是否是同一个文件（路径相同）
	if file1 == file2 {
		return true, nil
	}

	// 2. 快速检查文件大小（避免不必要的哈希计算）
	info1, err := os.Stat(file1)
	if err != nil {

		return false, err
	}

	info2, err := os.Stat(file2)
	if err != nil {
		return false, err
	}

	if info1.Size() != info2.Size() {
		fmt.Println("大小不相同 ")
		return false, nil
	} else {
		fmt.Println("大小相同 ")
	}
	fmt.Println("继续 ")
	// 3. 使用MD5哈希（最快的主流哈希算法）
	hash1, err := fastMD5(file1)
	if err != nil {
		return false, err
	}

	hash2, err := fastMD5(file2)
	if err != nil {
		return false, err
	}
	fmt.Printf("哈希值1: %s\r\n哈希值2: %s", hash1, hash2)

	return hash1 == hash2, nil
}

func fastMD5(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 使用较大缓冲区（1MB）提高读取速度
	buf := make([]byte, 1024*1024)
	hash := md5.New()

	for {
		n, err := file.Read(buf)
		if n > 0 {
			hash.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (mw *AppMainWindow) BenchmarkCompare(file1, file2 string) {
	start := time.Now()
	same, err := FastHashCompare(file1, file2)

	if err != nil {
		fmt.Printf("比较出错: %v\n", err)
		return
	}

	if same {

		mw.log(fmt.Sprintf("\r\n两个文件内容相同  [%s] == [%s]\r\n", filepath.Base(file1), filepath.Base(file2)))
		fmt.Println("两个文件相同")
	} else {

		fmt.Println("两个文件不同")
	}
	elapsed := time.Since(start)
	fmt.Printf("耗时: %v\n", elapsed)
}

func (mw *AppMainWindow) compare() {
	oldPath := mw.PatchTab.OldPathEdit.Text()
	newPath := mw.PatchTab.NewPathEdit.Text()

	if oldPath != "" && newPath != "" {
		if mw.getPathType(oldPath) == FileTypeFile && mw.getPathType(newPath) == FileTypeFile {
			mw.BenchmarkCompare(oldPath, newPath)
		}
	}
}

// GBK -> UTF-8
func GbkToUtf8(s []byte) ([]byte, error) {
	fmt.Println("CP is 936 (GBK)")
	reader := transform.NewReader(bytes.NewReader(s), simplifiedchinese.GBK.NewDecoder())
	all, err := io.ReadAll(reader)
	if err != nil {
		return all, err
	}
	return all, nil
}

func (mw *AppMainWindow) executeCommand(args []string) {
	toolPath := ""
	// 将工作目录切换到可执行文件所在目录，保证双击启动时能找到同目录的 hdiffz.exe
	if exe, err := os.Executable(); err == nil {
		if dir := filepath.Dir(exe); dir != "" {
			fmt.Println("work_dir: " + dir)
			_ = os.Chdir(dir)
		}
		toolPath = filepath.Join(filepath.Dir(exe), "hdiffz.exe")
		fmt.Println("toolPath: ", toolPath)
		if _, err_file_stat := os.Stat(toolPath); err_file_stat == nil {
			// hdiffz.exe 路径
			HdiffzPath = toolPath
			fmt.Println("hdiffz.exe_path: " + HdiffzPath)
		} else if os.IsNotExist(err_file_stat) {
			fmt.Println("错误: 未找到 hdiffz.exe 工具:", err_file_stat)
			mw.log("错误: 未找到 hdiffz.exe 工具")
			return
		} else {
			fmt.Println("错误:", err_file_stat)
			mw.log("错误")
			return
		}
	}

	go func() {
		cmd := exec.Command(HdiffzPath, args...)
		if HdiffzPath == "" {
			mw.log("错误: 当前目录下未找到 hdiffz.exe 可执行文件")
			return
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,       // 隐藏子进程控制台窗口
			CreationFlags: 0x08000000, // CREATE_NO_WINDOW 标志（强制无窗口）
		}

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
		outputRaw, _ := io.ReadAll(stdout)
		errorRaw, _ := io.ReadAll(stderr)
		var output []byte
		var errorOutput []byte
		var decodeErr error
		if Cp == 936 {
			output, decodeErr = GbkToUtf8(outputRaw)
			if decodeErr != nil {
				mw.log(fmt.Sprintf("GBK解码标准输出失败: %v", decodeErr))
				output = outputRaw // 解码失败则用原始字节
			}
			errorOutput, decodeErr = GbkToUtf8(errorRaw)
			if decodeErr != nil {
				mw.log(fmt.Sprintf("GBK解码标准错误失败: %v", decodeErr))
				errorOutput = errorRaw
			}
		} else {
			// Cp≠936：使用原始编码（保留原有逻辑）
			mw.log(fmt.Sprintf("当前编码非GBK（Cp=%d），使用原始编码输出", Cp))
			output = outputRaw
			errorOutput = errorRaw
		}
		if err := cmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					mw.log(fmt.Sprintf("进程退出，返回码: %d", status.ExitStatus()))
				}
			}
		}
		// 显示输出
		if len(output) > 0 {
			mw.log("\r\n===========================================================================\r\n" + strings.TrimSpace(string(output)) + "\r\n===========================================================================")
		}
		if len(errorOutput) > 0 {
			mw.log("\r\n====================================== ERROR ==============================\r\n" + strings.TrimSpace(string(errorOutput)) + "\r\n===========================================================================")
		}
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
	currentPatch := mw.PatchTab.PatchPathEdit.Text()
	if currentPatch == "" || currentPatch != mw.PatchTab.AutoPatchName {
		dir := filepath.Dir(oldPath)
		mw.PatchTab.PatchPathEdit.SetText(filepath.Join(dir, patchName))
	}
}

func (mw *AppMainWindow) updatePatchPathLabels() {
	oldType := mw.getPathType(mw.PatchTab.OldPathEdit.Text())
	switch oldType {
	case FileTypeFile:
		mw.PatchTab.OldPathLabel.SetText("📄 文件")
	case FileTypeDirectory:
		mw.PatchTab.OldPathLabel.SetText("📁 文件夹")
	default:
		mw.PatchTab.OldPathLabel.SetText("❓ 未知")
	}

	newType := mw.getPathType(mw.PatchTab.NewPathEdit.Text())
	switch newType {
	case FileTypeFile:
		mw.PatchTab.NewPathLabel.SetText("📄 文件")
	case FileTypeDirectory:
		mw.PatchTab.NewPathLabel.SetText("📁 文件夹")
	default:
		mw.PatchTab.NewPathLabel.SetText("❓ 未知")
	}
}

func (mw *AppMainWindow) updateApplyPathLabels() {
	oldType := mw.getPathType(mw.ApplyTab.OldPathEdit.Text())
	switch oldType {
	case FileTypeFile:
		mw.ApplyTab.OldPathLabel.SetText("📄 文件")
	case FileTypeDirectory:
		mw.ApplyTab.OldPathLabel.SetText("📁 文件夹")
	default:
		mw.ApplyTab.OldPathLabel.SetText("❓ 未知")
	}

	newType := mw.getPathType(mw.ApplyTab.NewPathEdit.Text())
	switch newType {
	case FileTypeFile:
		mw.ApplyTab.NewPathLabel.SetText("📄 文件")
	case FileTypeDirectory:
		mw.ApplyTab.NewPathLabel.SetText("📁 文件夹")
	default:
		mw.ApplyTab.NewPathLabel.SetText("❓ 未知")
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

	currentNew := mw.ApplyTab.NewPathEdit.Text()
	if currentNew == "" || currentNew == mw.ApplyTab.AutoPatchName {
		dir := filepath.Dir(oldPath)
		mw.ApplyTab.NewPathEdit.SetText(filepath.Join(dir, newName))
	}
}

func (mw *AppMainWindow) createPatch() {
	oldPath := mw.PatchTab.OldPathEdit.Text()
	newPath := mw.PatchTab.NewPathEdit.Text()
	patchPath := mw.PatchTab.PatchPathEdit.Text()

	if oldPath == "" {
		mw.log("错误: 请选择旧文件/文件夹路径")
		return
	}
	if newPath == "" {
		mw.log("错误: 请选择新文件/文件夹路径")
		return
	}
	if patchPath == "" {
		mw.log("错误: 请指定补丁文件输出路径")
		return
	}
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		mw.log("错误: 旧路径不存在 - " + oldPath)
		return
	}
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		mw.log("错误: 新路径不存在 - " + newPath)
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
	oldPath := mw.ApplyTab.OldPathEdit.Text()
	patchPath := mw.ApplyTab.PatchPathEdit.Text()
	newPath := mw.ApplyTab.NewPathEdit.Text()

	if oldPath == "" || patchPath == "" {
		mw.log("错误: 请选择旧文件和补丁文件路径")
		return
	}
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
	// 显式关联主窗口句柄（修复旧版walk兼容）
	if ok, _ := dlg.ShowOpen(mw.MainWindow); ok {
		if dlg.FilePath != "" && edit != nil {
			edit.SetText(dlg.FilePath)
		}
	}
}

func (mw *AppMainWindow) selectFolder(edit *walk.LineEdit, title string) {
	shell32 := syscall.NewLazyDLL("shell32.dll")
	procSHBrowseForFolder := shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDList := shell32.NewProc("SHGetPathFromIDListW")
	ole32 := syscall.NewLazyDLL("ole32.dll")
	procCoTaskMemFree := ole32.NewProc("CoTaskMemFree")

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
		ulFlags:        0x00000001 | 0x00000040,
		lpfn:           0,
		lParam:         0,
	}

	ret, _, _ := procSHBrowseForFolder.Call(uintptr(unsafe.Pointer(&bi)))
	if ret == 0 {
		return
	}
	pidl := ret

	var pathBuf [syscall.MAX_PATH]uint16
	ok, _, _ := procSHGetPathFromIDList.Call(pidl, uintptr(unsafe.Pointer(&pathBuf[0])))
	if ok == 0 {
		procCoTaskMemFree.Call(pidl)
		mw.log("错误: 无法从 IDList 获取路径")
		return
	}

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
	// 显式关联主窗口句柄
	if ok, _ := dlg.ShowSave(mw.MainWindow); ok {
		if dlg.FilePath != "" && edit != nil {
			edit.SetText(dlg.FilePath)
		}
	}
}

func (mw *AppMainWindow) handleDropFiles(files []string) {
	if len(files) == 0 {
		return
	}
	mw.Synchronize(func() {
		currentIndex := mw.TabWidget.CurrentIndex()
		path := files[0]

		user32 := syscall.NewLazyDLL("user32.dll")
		procGetCursorPos := user32.NewProc("GetCursorPos")
		procGetWindowRect := user32.NewProc("GetWindowRect")

		var pt struct{ X, Y int32 }
		r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
		if r != 0 {
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

			if currentIndex == 0 {
				if isPointInWindow(mw.PatchTab.OldPathEdit) {
					mw.PatchTab.OldPathEdit.SetText(path)
					fmt.Printf("拖放文件: %s -> 旧路径", path)
					return
				}
				if isPointInWindow(mw.PatchTab.NewPathEdit) {
					mw.PatchTab.NewPathEdit.SetText(path)
					fmt.Printf("拖放文件: %s -> 新路径", path)
					return
				}
				if isPointInWindow(mw.PatchTab.PatchPathEdit) {
					mw.PatchTab.PatchPathEdit.SetText(path)
					fmt.Printf("拖放文件: %s -> 补丁路径", path)
					return
				}
			} else {
				if isPointInWindow(mw.ApplyTab.OldPathEdit) {
					mw.ApplyTab.OldPathEdit.SetText(path)
					fmt.Printf("拖放文件: %s -> 旧路径", path)
					return
				}
				if isPointInWindow(mw.ApplyTab.PatchPathEdit) {
					mw.ApplyTab.PatchPathEdit.SetText(path)
					fmt.Printf("拖放文件: %s -> 补丁路径", path)
					return
				}
				if isPointInWindow(mw.ApplyTab.NewPathEdit) {
					mw.ApplyTab.NewPathEdit.SetText(path)
					fmt.Printf("拖放文件: %s -> 新路径", path)
					return
				}
			}
		}
	})
}

func main() {
	// 创建窗口实例
	mw := &AppMainWindow{}
	mw.PatchTab = &PatchTab{}
	mw.ApplyTab = &ApplyTab{}

	// ========== 获取系统默认ANSI编码 ==========
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procGetACP := kernel32.NewProc("GetACP")
	Cp, _, _ = procGetACP.Call()
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
									Label{Text: "旧文件/文件夹:"},
									LineEdit{
										AssignTo: &mw.PatchTab.OldPathEdit,
										OnTextChanged: func() {
											mw.updatePatchName()
											mw.updatePatchPathLabels()
											mw.compare()
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
									Label{AssignTo: &mw.PatchTab.OldPathLabel, Text: ""},

									Label{Text: "新文件/文件夹:"},
									LineEdit{
										AssignTo: &mw.PatchTab.NewPathEdit,
										OnTextChanged: func() {
											mw.updatePatchName()
											mw.updatePatchPathLabels()
											mw.compare()
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
									Label{AssignTo: &mw.PatchTab.NewPathLabel, Text: ""},

									Label{Text: "补丁文件:"},
									LineEdit{AssignTo: &mw.PatchTab.PatchPathEdit},
									PushButton{
										AssignTo: &mw.PatchTab.SelectPatchBtn,
										Text:     "选择...",
										OnClicked: func() {
											mw.selectSaveFile(mw.PatchTab.PatchPathEdit, "选择补丁文件", "补丁文件 (*.diff)|*.diff")
										},
									},
									Label{AssignTo: &mw.PatchTab.PatchPathLabel, Text: ""},
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

					TabPage{
						Title:  "应用补丁",
						Layout: VBox{},
						Children: []Widget{
							Composite{
								Layout: Grid{Columns: 4, Spacing: 10},
								Children: []Widget{
									Label{Text: "旧文件/文件夹:"},
									LineEdit{
										AssignTo: &mw.ApplyTab.OldPathEdit,
										OnTextChanged: func() {
											mw.updateApplyName()
											mw.updateApplyPathLabels()
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
									Label{AssignTo: &mw.ApplyTab.OldPathLabel, Text: ""},

									Label{Text: "补丁文件:"},
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
											mw.selectFile(mw.ApplyTab.PatchPathEdit, "选择补丁文件", "全部文件(*.*)|*.*|补丁文件 (*.diff)|*.diff")
										},
									},
									Label{AssignTo: &mw.ApplyTab.PatchPathLabel, Text: ""},

									Label{Text: "新文件/文件夹:"},
									LineEdit{
										AssignTo: &mw.ApplyTab.NewPathEdit,
										OnTextChanged: func() {
											mw.updateApplyPathLabels()
										},
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
									Label{AssignTo: &mw.ApplyTab.NewPathLabel, Text: ""},
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

	fmt.Println("Starting Run()...")
	ret, err := w.Run()
	fmt.Println("Run() returned code:", ret, "error:>>>", err, "<<<")
}
