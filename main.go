//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	appTitle = "Enter Scheduler"

	WS_OVERLAPPED      = 0x00000000
	WS_CAPTION         = 0x00C00000
	WS_SYSMENU         = 0x00080000
	WS_MINIMIZEBOX     = 0x00020000
	WS_VISIBLE         = 0x10000000
	WS_CHILD           = 0x40000000
	WS_TABSTOP         = 0x00010000
	WS_VSCROLL         = 0x00200000
	WS_EX_CLIENTEDGE   = 0x00000200
	WS_EX_APPWINDOW    = 0x00040000
	CBS_DROPDOWNLIST   = 0x0003
	CBS_HASSTRINGS     = 0x0200
	BS_PUSHBUTTON      = 0x00000000
	BS_DEFPUSHBUTTON   = 0x00000001
	BS_AUTOCHECKBOX    = 0x00000003
	SS_LEFT            = 0x00000000
	SW_RESTORE         = 9
	SW_MINIMIZE        = 6
	SW_SHOW            = 5
	GA_ROOT            = 2
	GW_OWNER           = 4
	WM_CREATE          = 0x0001
	WM_DESTROY         = 0x0002
	WM_CLOSE           = 0x0010
	WM_COMMAND         = 0x0111
	WM_TIMER           = 0x0113
	WM_SETFONT         = 0x0030
	WM_SETICON         = 0x0080
	ICON_SMALL         = 0
	ICON_BIG           = 1
	BM_GETCHECK        = 0x00F0
	BST_CHECKED        = 1
	CBN_SELCHANGE      = 1
	CB_ADDSTRING       = 0x0143
	CB_GETCURSEL       = 0x0147
	CB_SETCURSEL       = 0x014E
	CB_RESETCONTENT    = 0x014B
	MB_OK              = 0x00000000
	MB_ICONINFORMATION = 0x00000040
	MB_ICONWARNING     = 0x00000030
	MB_ICONERROR       = 0x00000010
	MB_YESNO           = 0x00000004
	IDYES              = 6
	VK_RETURN          = 0x0D
	INPUT_KEYBOARD     = 1
	KEYEVENTF_KEYUP    = 0x0002
	SPI_GETWORKAREA    = 0x0030
	SM_CXSCREEN        = 0
	SM_CYSCREEN        = 1
	COLOR_WINDOW       = 5
	IDC_ARROW          = 32512
	IDI_APPLICATION    = 32512
	IMAGE_ICON         = 1
	LR_LOADFROMFILE    = 0x0010
	LR_DEFAULTSIZE     = 0x0040
	ERROR_SUCCESS      = 0
	timerID            = 1
)

const (
	idTargetCombo = 1001 + iota
	idRefresh
	idHourCombo
	idMinuteCombo
	idStart
	idCancel
	idTest
	idMinimize
)

type POINT struct{ X, Y int32 }
type MSG struct {
	Hwnd           uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             POINT
	LPrivate       uint32
}
type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}
type RECT struct{ Left, Top, Right, Bottom int32 }
type KEYBDINPUT struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}
type INPUT struct {
	Type uint32
	_    uint32
	Ki   KEYBDINPUT
	_pad [8]byte // INPUT is 40 bytes on Windows amd64.
}
type WindowItem struct {
	hwnd  uintptr
	title string
}

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")

	pRegisterClassExW         = user32.NewProc("RegisterClassExW")
	pCreateWindowExW          = user32.NewProc("CreateWindowExW")
	pDefWindowProcW           = user32.NewProc("DefWindowProcW")
	pDestroyWindow            = user32.NewProc("DestroyWindow")
	pPostQuitMessage          = user32.NewProc("PostQuitMessage")
	pGetMessageW              = user32.NewProc("GetMessageW")
	pTranslateMessage         = user32.NewProc("TranslateMessage")
	pDispatchMessageW         = user32.NewProc("DispatchMessageW")
	pShowWindow               = user32.NewProc("ShowWindow")
	pUpdateWindow             = user32.NewProc("UpdateWindow")
	pSendMessageW             = user32.NewProc("SendMessageW")
	pSetWindowTextW           = user32.NewProc("SetWindowTextW")
	pEnableWindow             = user32.NewProc("EnableWindow")
	pMessageBoxW              = user32.NewProc("MessageBoxW")
	pEnumWindows              = user32.NewProc("EnumWindows")
	pIsWindowVisible          = user32.NewProc("IsWindowVisible")
	pIsWindow                 = user32.NewProc("IsWindow")
	pGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	pGetWindowTextW           = user32.NewProc("GetWindowTextW")
	pGetWindow                = user32.NewProc("GetWindow")
	pGetAncestor              = user32.NewProc("GetAncestor")
	pSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	pGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	pShowWindowAsync          = user32.NewProc("ShowWindowAsync")
	pBringWindowToTop         = user32.NewProc("BringWindowToTop")
	pGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	pAttachThreadInput        = user32.NewProc("AttachThreadInput")
	pSendInput                = user32.NewProc("SendInput")
	pSetTimer                 = user32.NewProc("SetTimer")
	pKillTimer                = user32.NewProc("KillTimer")
	pLoadCursorW              = user32.NewProc("LoadCursorW")
	pLoadIconW                = user32.NewProc("LoadIconW")
	pLoadImageW               = user32.NewProc("LoadImageW")
	pSystemParametersInfoW    = user32.NewProc("SystemParametersInfoW")
	pGetSystemMetrics         = user32.NewProc("GetSystemMetrics")

	pGetModuleHandleW   = kernel32.NewProc("GetModuleHandleW")
	pGetCurrentThreadId = kernel32.NewProc("GetCurrentThreadId")
	pGetModuleFileNameW = kernel32.NewProc("GetModuleFileNameW")
	pGetTempPathW       = kernel32.NewProc("GetTempPathW")
	pGetStockObject     = gdi32.NewProc("GetStockObject")

	hwndMain                                    uintptr
	hTarget, hHour, hMinute                     uintptr
	hRefresh, hStart, hCancel, hTest, hMinimize uintptr
	hSelected, hCountdown, hStatus              uintptr
	windowsList                                 []WindowItem
	lockedTarget                                WindowItem
	scheduledAt                                 time.Time
	scheduling                                  bool
)

func utf16Ptr(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func loWord(v uintptr) uint16   { return uint16(v & 0xffff) }
func hiWord(v uintptr) uint16   { return uint16((v >> 16) & 0xffff) }

func setText(hwnd uintptr, s string) {
	pSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(utf16Ptr(s))))
}
func showMessage(title, text string, flags uintptr) int {
	r, _, _ := pMessageBoxW.Call(hwndMain, uintptr(unsafe.Pointer(utf16Ptr(text))), uintptr(unsafe.Pointer(utf16Ptr(title))), flags)
	return int(r)
}

func createControl(class, text string, style uint32, x, y, w, h int32, id int) uintptr {
	r, _, _ := pCreateWindowExW.Call(0,
		uintptr(unsafe.Pointer(utf16Ptr(class))), uintptr(unsafe.Pointer(utf16Ptr(text))),
		uintptr(style|WS_CHILD|WS_VISIBLE), uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		hwndMain, uintptr(id), 0, 0)
	return r
}
func addComboItem(hwnd uintptr, text string) {
	pSendMessageW.Call(hwnd, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(utf16Ptr(text))))
}
func getComboSelection(hwnd uintptr) int {
	r, _, _ := pSendMessageW.Call(hwnd, CB_GETCURSEL, 0, 0)
	if int64(r) < 0 {
		return -1
	}
	return int(r)
}
func resetCombo(hwnd uintptr)             { pSendMessageW.Call(hwnd, CB_RESETCONTENT, 0, 0) }
func selectCombo(hwnd uintptr, index int) { pSendMessageW.Call(hwnd, CB_SETCURSEL, uintptr(index), 0) }
func setEnabled(hwnd uintptr, on bool) {
	var v uintptr
	if on {
		v = 1
	}
	pEnableWindow.Call(hwnd, v)
}

func getWindowTitle(hwnd uintptr) string {
	n, _, _ := pGetWindowTextLengthW.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	pGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return syscall.UTF16ToString(buf)
}
func enumWindowsProc(hwnd, lparam uintptr) uintptr {
	if hwnd == hwndMain {
		return 1
	}
	v, _, _ := pIsWindowVisible.Call(hwnd)
	if v == 0 {
		return 1
	}
	root, _, _ := pGetAncestor.Call(hwnd, GA_ROOT)
	if root != hwnd {
		return 1
	}
	owner, _, _ := pGetWindow.Call(hwnd, GW_OWNER)
	if owner != 0 {
		return 1
	}
	title := strings.TrimSpace(getWindowTitle(hwnd))
	if title == "" {
		return 1
	}
	windowsList = append(windowsList, WindowItem{hwnd: hwnd, title: title})
	return 1
}
func refreshWindowList(notify bool) {
	old := uintptr(0)
	if idx := getComboSelection(hTarget); idx >= 0 && idx < len(windowsList) {
		old = windowsList[idx].hwnd
	}
	windowsList = windowsList[:0]
	resetCombo(hTarget)
	pEnumWindows.Call(syscall.NewCallback(enumWindowsProc), 0)
	selected := -1
	for i, w := range windowsList {
		addComboItem(hTarget, fmt.Sprintf("[%d] %s", i+1, w.title))
		if w.hwnd == old {
			selected = i
		}
	}
	if selected < 0 && len(windowsList) > 0 {
		selected = 0
	}
	if selected >= 0 {
		selectCombo(hTarget, selected)
	}
	updateTargetPreview()
	if len(windowsList) == 0 {
		setText(hStatus, "Δεν βρέθηκαν διαθέσιμα παράθυρα.")
	} else if notify {
		setText(hStatus, "Η λίστα παραθύρων ανανεώθηκε.")
	}
}
func updateTargetPreview() {
	idx := getComboSelection(hTarget)
	if idx < 0 || idx >= len(windowsList) {
		setText(hSelected, "Επιλεγμένο: —")
		return
	}
	setText(hSelected, "Επιλεγμένο: "+windowsList[idx].title)
}
func currentTarget() (WindowItem, bool) {
	idx := getComboSelection(hTarget)
	if idx < 0 || idx >= len(windowsList) {
		return WindowItem{}, false
	}
	return windowsList[idx], true
}
func isValidWindow(hwnd uintptr) bool { r, _, _ := pIsWindow.Call(hwnd); return r != 0 }

func activateTarget(hwnd uintptr) bool {
	if !isValidWindow(hwnd) {
		return false
	}
	pShowWindowAsync.Call(hwnd, SW_RESTORE)
	targetThread, _, _ := pGetWindowThreadProcessId.Call(hwnd, 0)
	currentThread, _, _ := pGetCurrentThreadId.Call()
	attached := false
	if targetThread != 0 && targetThread != currentThread {
		r, _, _ := pAttachThreadInput.Call(currentThread, targetThread, 1)
		attached = r != 0
	}
	pBringWindowToTop.Call(hwnd)
	pSetForegroundWindow.Call(hwnd)
	if attached {
		pAttachThreadInput.Call(currentThread, targetThread, 0)
	}
	deadline := time.Now().Add(800 * time.Millisecond)
	for time.Now().Before(deadline) {
		fg, _, _ := pGetForegroundWindow.Call()
		if fg == hwnd {
			return true
		}
		time.Sleep(25 * time.Millisecond)
		pSetForegroundWindow.Call(hwnd)
	}
	return false
}
func sendKeyboardEnter() bool {
	inputs := [2]INPUT{
		{Type: INPUT_KEYBOARD, Ki: KEYBDINPUT{WVk: VK_RETURN}},
		{Type: INPUT_KEYBOARD, Ki: KEYBDINPUT{WVk: VK_RETURN, DwFlags: KEYEVENTF_KEYUP}},
	}
	r, _, _ := pSendInput.Call(2, uintptr(unsafe.Pointer(&inputs[0])), unsafe.Sizeof(inputs[0]))
	return r == 2
}
func executeEnter(test bool) {
	target := lockedTarget
	if !scheduling || test {
		var ok bool
		target, ok = currentTarget()
		if !ok {
			showMessage(appTitle, "Επίλεξε πρώτα ένα παράθυρο-στόχο.", MB_OK|MB_ICONWARNING)
			return
		}
	}
	if !isValidWindow(target.hwnd) {
		showMessage(appTitle, "Το επιλεγμένο παράθυρο έχει κλείσει. Το Enter δεν στάλθηκε.", MB_OK|MB_ICONWARNING)
		if scheduling {
			cancelSchedule(false)
		}
		return
	}
	setText(hStatus, "Ενεργοποίηση του επιλεγμένου παραθύρου…")
	if !activateTarget(target.hwnd) {
		showMessage(appTitle, "Τα Windows δεν επέτρεψαν την ενεργοποίηση του επιλεγμένου παραθύρου. Το Enter δεν στάλθηκε αλλού. Αν η εφαρμογή-στόχος εκτελείται ως διαχειριστής, εκτέλεσε και το Enter Scheduler ως διαχειριστής.", MB_OK|MB_ICONWARNING)
		if scheduling {
			cancelSchedule(false)
		}
		return
	}
	if !sendKeyboardEnter() {
		showMessage(appTitle, "Το παράθυρο ενεργοποιήθηκε, αλλά η αποστολή του Enter απέτυχε.", MB_OK|MB_ICONERROR)
		if scheduling {
			cancelSchedule(false)
		}
		return
	}
	if test {
		setText(hStatus, "Η δοκιμή Enter στάλθηκε στο: "+target.title)
	} else {
		setText(hStatus, "Το Enter στάλθηκε στο: "+target.title)
		pKillTimer.Call(hwndMain, timerID)
		scheduling = false
		setSchedulingControls(false)
		setText(hCountdown, "Απομένει: —")
	}
}

func comboInt(hwnd uintptr) (int, bool) {
	i := getComboSelection(hwnd)
	if i < 0 {
		return 0, false
	}
	return i, true
}
func nextExecution(hour, minute int) time.Time {
	now := time.Now()
	t := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !t.After(now) {
		t = t.AddDate(0, 0, 1)
	}
	return t
}
func setSchedulingControls(on bool) {
	setEnabled(hTarget, !on)
	setEnabled(hRefresh, !on)
	setEnabled(hHour, !on)
	setEnabled(hMinute, !on)
	setEnabled(hStart, !on)
	setEnabled(hCancel, on)
	setEnabled(hTest, !on)
}
func startSchedule() {
	t, ok := currentTarget()
	if !ok {
		showMessage(appTitle, "Δεν έχει επιλεγεί παράθυρο.", MB_OK|MB_ICONWARNING)
		return
	}
	h, okH := comboInt(hHour)
	m, okM := comboInt(hMinute)
	if !okH || !okM {
		showMessage(appTitle, "Επίλεξε παράθυρο, ώρα και λεπτά.", MB_OK|MB_ICONWARNING)
		return
	}
	lockedTarget = t
	scheduledAt = nextExecution(h, m)
	scheduling = true
	setSchedulingControls(true)
	setText(hSelected, "Κλειδωμένος στόχος: "+t.title)
	setText(hStatus, fmt.Sprintf("Προγραμματίστηκε για %s στις %s", scheduledAt.Format("02/01/2006"), scheduledAt.Format("15:04")))
	pSetTimer.Call(hwndMain, timerID, 250, 0)
	updateCountdown()
	checked, _, _ := pSendMessageW.Call(hMinimize, BM_GETCHECK, 0, 0)
	if checked == BST_CHECKED {
		pShowWindow.Call(hwndMain, SW_MINIMIZE)
	}
}
func cancelSchedule(notify bool) {
	if scheduling {
		pKillTimer.Call(hwndMain, timerID)
	}
	scheduling = false
	setSchedulingControls(false)
	updateTargetPreview()
	setText(hCountdown, "Απομένει: —")
	if notify {
		setText(hStatus, "Ο προγραμματισμός ακυρώθηκε.")
	}
}
func updateCountdown() {
	if !scheduling {
		return
	}
	if !isValidWindow(lockedTarget.hwnd) {
		cancelSchedule(false)
		showMessage(appTitle, "Το επιλεγμένο παράθυρο δεν υπάρχει πλέον. Πάτησε «Ανανέωση» και επίλεξέ το ξανά.", MB_OK|MB_ICONWARNING)
		return
	}
	d := time.Until(scheduledAt)
	if d <= 0 {
		pKillTimer.Call(hwndMain, timerID)
		setText(hCountdown, "Η ώρα έφτασε. Αποστολή Enter…")
		executeEnter(false)
		return
	}
	total := int64(d.Round(time.Second).Seconds())
	if total < 0 {
		total = 0
	}
	days := total / 86400
	total %= 86400
	hours := total / 3600
	total %= 3600
	mins := total / 60
	secs := total % 60
	if days > 0 {
		setText(hCountdown, fmt.Sprintf("Απομένει: %d ημ. %02d:%02d:%02d", days, hours, mins, secs))
	} else {
		setText(hCountdown, fmt.Sprintf("Απομένει: %02d:%02d:%02d", hours, mins, secs))
	}
}

func wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_COMMAND:
		id := int(loWord(wParam))
		notify := hiWord(wParam)
		switch id {
		case idTargetCombo:
			if notify == CBN_SELCHANGE {
				updateTargetPreview()
			}
		case idRefresh:
			refreshWindowList(true)
		case idStart:
			startSchedule()
		case idCancel:
			cancelSchedule(true)
		case idTest:
			executeEnter(true)
		}
		return 0
	case WM_TIMER:
		if wParam == timerID {
			updateCountdown()
			return 0
		}
	case WM_CLOSE:
		if scheduling {
			if showMessage(appTitle, "Υπάρχει ενεργός προγραμματισμός. Θέλεις να κλείσεις το πρόγραμμα και να τον ακυρώσεις;", MB_YESNO|MB_ICONWARNING) != IDYES {
				return 0
			}
		}
		pDestroyWindow.Call(hwnd)
		return 0
	case WM_DESTROY:
		pKillTimer.Call(hwnd, timerID)
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func loadExternalIcon() uintptr {
	exeBuf := make([]uint16, 32768)
	n, _, _ := pGetModuleFileNameW.Call(0, uintptr(unsafe.Pointer(&exeBuf[0])), uintptr(len(exeBuf)))
	if n == 0 {
		return 0
	}
	exe := syscall.UTF16ToString(exeBuf[:n])
	iconPath := filepath.Join(filepath.Dir(exe), "assets", "enter_scheduler.ico")
	r, _, _ := pLoadImageW.Call(0, uintptr(unsafe.Pointer(utf16Ptr(iconPath))), IMAGE_ICON, 0, 0, LR_LOADFROMFILE|LR_DEFAULTSIZE)
	return r
}
func stockFont() uintptr { r, _, _ := pGetStockObject.Call(17); return r }
func applyFont(handles ...uintptr) {
	f := stockFont()
	for _, h := range handles {
		if h != 0 {
			pSendMessageW.Call(h, WM_SETFONT, f, 1)
		}
	}
}

func initUI() {
	createControl("STATIC", "Παράθυρο-στόχος:", SS_LEFT, 24, 20, 170, 24, 0)
	hTarget = createControl("COMBOBOX", "", WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST|CBS_HASSTRINGS, 24, 48, 395, 300, idTargetCombo)
	hRefresh = createControl("BUTTON", "Ανανέωση", WS_TABSTOP|BS_PUSHBUTTON, 430, 47, 118, 30, idRefresh)
	hSelected = createControl("STATIC", "Επιλεγμένο: —", SS_LEFT, 24, 84, 520, 22, 0)

	createControl("STATIC", "Επιλογή ώρας:", SS_LEFT, 24, 117, 170, 24, 0)
	createControl("STATIC", "Ώρα", SS_LEFT, 38, 149, 50, 22, 0)
	hHour = createControl("COMBOBOX", "", WS_TABSTOP|CBS_DROPDOWNLIST|CBS_HASSTRINGS, 89, 145, 74, 300, idHourCombo)
	createControl("STATIC", ":", SS_LEFT, 169, 149, 12, 22, 0)
	hMinute = createControl("COMBOBOX", "", WS_TABSTOP|CBS_DROPDOWNLIST|CBS_HASSTRINGS, 183, 145, 74, 500, idMinuteCombo)
	for i := 0; i < 24; i++ {
		addComboItem(hHour, fmt.Sprintf("%02d", i))
	}
	for i := 0; i < 60; i++ {
		addComboItem(hMinute, fmt.Sprintf("%02d", i))
	}
	now := time.Now().Add(time.Minute)
	selectCombo(hHour, now.Hour())
	selectCombo(hMinute, now.Minute())

	hCountdown = createControl("STATIC", "Απομένει: —", SS_LEFT, 38, 183, 500, 24, 0)
	hStart = createControl("BUTTON", "Έναρξη", WS_TABSTOP|BS_DEFPUSHBUTTON, 24, 219, 165, 40, idStart)
	hCancel = createControl("BUTTON", "Ακύρωση", WS_TABSTOP|BS_PUSHBUTTON, 202, 219, 165, 40, idCancel)
	hTest = createControl("BUTTON", "Δοκιμή Enter", WS_TABSTOP|BS_PUSHBUTTON, 380, 219, 168, 40, idTest)
	hMinimize = createControl("BUTTON", "Ελαχιστοποίηση μετά την έναρξη", WS_TABSTOP|BS_AUTOCHECKBOX, 24, 273, 330, 27, idMinimize)
	hStatus = createControl("STATIC", "Φόρτωση παραθύρων…", SS_LEFT, 24, 315, 524, 42, 0)

	applyFont(hTarget, hRefresh, hSelected, hHour, hMinute, hCountdown, hStart, hCancel, hTest, hMinimize, hStatus)
	setSchedulingControls(false)
	refreshWindowList(false)
}

func writeCrashLog(v any) string {
	var temp [32768]uint16
	n, _, _ := pGetTempPathW.Call(uintptr(len(temp)), uintptr(unsafe.Pointer(&temp[0])))
	dir := os.TempDir()
	if n > 0 && n < uintptr(len(temp)) {
		dir = syscall.UTF16ToString(temp[:n])
	}
	path := filepath.Join(dir, "Enter_Scheduler_error.txt")
	text := fmt.Sprintf("Enter Scheduler v3 panic: %v\nTime: %s\n\n%s\n", v, time.Now().Format(time.RFC3339), debug.Stack())
	_ = os.WriteFile(path, []byte(text), 0644)
	return path
}

func main() {
	defer func() {
		if v := recover(); v != nil {
			path := writeCrashLog(v)
			showMessage(appTitle, "Το πρόγραμμα τερματίστηκε από εσωτερικό σφάλμα. Αρχείο διάγνωσης:\n"+path, MB_OK|MB_ICONERROR)
		}
	}()
	hInst, _, _ := pGetModuleHandleW.Call(0)
	className := utf16Ptr("EnterSchedulerWindowClass")
	cursor, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
	icon, _, _ := pLoadIconW.Call(hInst, 1)
	if icon == 0 {
		icon = loadExternalIcon()
	}
	if icon == 0 {
		icon, _, _ = pLoadIconW.Call(0, IDI_APPLICATION)
	}
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(wndProc), HInstance: hInst, HIcon: icon, HCursor: cursor, HbrBackground: COLOR_WINDOW + 1, LpszClassName: className, HIconSm: icon}
	if r, _, _ := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		showMessage(appTitle, "Δεν ήταν δυνατή η αρχικοποίηση του παραθύρου.", MB_OK|MB_ICONERROR)
		return
	}

	width, height := int32(590), int32(410)
	var wa RECT
	pSystemParametersInfoW.Call(SPI_GETWORKAREA, 0, uintptr(unsafe.Pointer(&wa)), 0)
	screenW, screenH := int32(wa.Right-wa.Left), int32(wa.Bottom-wa.Top)
	if screenW <= 0 {
		r, _, _ := pGetSystemMetrics.Call(SM_CXSCREEN)
		screenW = int32(r)
	}
	if screenH <= 0 {
		r, _, _ := pGetSystemMetrics.Call(SM_CYSCREEN)
		screenH = int32(r)
	}
	x := wa.Left + (screenW-width)/2
	y := wa.Top + (screenH-height)/2

	hwndMain, _, _ = pCreateWindowExW.Call(WS_EX_APPWINDOW, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(utf16Ptr(appTitle))),
		WS_OVERLAPPED|WS_CAPTION|WS_SYSMENU|WS_MINIMIZEBOX, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0, 0, hInst, 0)
	if hwndMain == 0 {
		showMessage(appTitle, "Δεν ήταν δυνατή η δημιουργία του κύριου παραθύρου.", MB_OK|MB_ICONERROR)
		return
	}
	if icon != 0 {
		pSendMessageW.Call(hwndMain, WM_SETICON, ICON_BIG, icon)
		pSendMessageW.Call(hwndMain, WM_SETICON, ICON_SMALL, icon)
	}
	initUI()
	pShowWindow.Call(hwndMain, SW_SHOW)
	pUpdateWindow.Call(hwndMain)

	var msg MSG
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) == -1 {
			showMessage(appTitle, "Παρουσιάστηκε σφάλμα στον βρόχο μηνυμάτων των Windows.", MB_OK|MB_ICONERROR)
			return
		}
		if r == 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

var _ = strconv.IntSize
