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

	WS_OVERLAPPED     = 0x00000000
	WS_CAPTION        = 0x00C00000
	WS_SYSMENU        = 0x00080000
	WS_MINIMIZEBOX    = 0x00020000
	WS_VISIBLE        = 0x10000000
	WS_CHILD          = 0x40000000
	WS_TABSTOP        = 0x00010000
	WS_VSCROLL        = 0x00200000
	WS_BORDER         = 0x00800000
	WS_EX_CLIENTEDGE  = 0x00000200
	WS_EX_APPWINDOW   = 0x00040000
	CBS_DROPDOWNLIST  = 0x0003
	CBS_HASSTRINGS    = 0x0200
	BS_PUSHBUTTON     = 0x00000000
	BS_DEFPUSHBUTTON  = 0x00000001
	BS_AUTOCHECKBOX   = 0x00000003
	SS_LEFT           = 0x00000000
	ES_AUTOHSCROLL    = 0x0080
	ES_NUMBER         = 0x2000
	SW_RESTORE        = 9
	SW_MINIMIZE       = 6
	SW_SHOW           = 5
	GA_ROOT           = 2
	GW_OWNER          = 4
	WM_DESTROY        = 0x0002
	WM_CLOSE          = 0x0010
	WM_COMMAND        = 0x0111
	WM_TIMER          = 0x0113
	WM_SETFONT        = 0x0030
	WM_SETICON        = 0x0080
	ICON_SMALL        = 0
	ICON_BIG          = 1
	BM_GETCHECK       = 0x00F0
	BST_CHECKED       = 1
	CBN_SELCHANGE     = 1
	CB_ADDSTRING      = 0x0143
	CB_GETCURSEL      = 0x0147
	CB_SETCURSEL      = 0x014E
	CB_RESETCONTENT   = 0x014B
	EM_SETLIMITTEXT   = 0x00C5
	MB_OK             = 0x00000000
	MB_ICONWARNING    = 0x00000030
	MB_ICONERROR      = 0x00000010
	MB_YESNO           = 0x00000004
	IDYES              = 6
	VK_RETURN          = 0x0D
	INPUT_KEYBOARD     = 1
	KEYEVENTF_KEYUP    = 0x0002
	KEYEVENTF_UNICODE  = 0x0004
	SPI_GETWORKAREA    = 0x0030
	SM_CXSCREEN        = 0
	SM_CYSCREEN        = 1
	COLOR_WINDOW       = 5
	IDC_ARROW          = 32512
	IDI_APPLICATION    = 32512
	IMAGE_ICON         = 1
	LR_LOADFROMFILE    = 0x0010
	LR_DEFAULTSIZE     = 0x0040
	timerID            = 1
	timerPeriodMs      = 10
	maxDelayMs         = 2147483647 // ~24.8 days
	maxLoopHours       = 100000
	maxTextUTF16Units  = 4096
)

const (
	idTargetCombo = 1001 + iota
	idRefresh
	idHourCombo
	idMinuteCombo
	idPreDelay
	idText
	idPostDelay
	idLoopHours
	idLoopMinutes
	idLoopSeconds
	idLoopMillis
	idStart
	idCancel
	idTest
	idMinimize
)

const (
	phaseWaitingTrigger = iota
	phaseWaitingText
	phaseWaitingEnter
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
type RunConfig struct {
	preDelay     time.Duration
	text         string
	postDelay    time.Duration
	loopInterval time.Duration
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

	hwndMain uintptr

	hTarget, hHour, hMinute                         uintptr
	hPreDelay, hText, hPostDelay                    uintptr
	hLoopHours, hLoopMinutes, hLoopSeconds, hLoopMs uintptr
	hRefresh, hStart, hCancel, hTest, hMinimize     uintptr
	hSelected, hCountdown, hStatus                  uintptr

	windowsList  []WindowItem
	lockedTarget WindowItem
	runConfig    RunConfig
	nextActionAt time.Time
	runActive    bool
	testMode     bool
	phase        int
	runCount     int
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
func createControlEx(exStyle uint32, class, text string, style uint32, x, y, w, h int32, id int) uintptr {
	r, _, _ := pCreateWindowExW.Call(uintptr(exStyle),
		uintptr(unsafe.Pointer(utf16Ptr(class))), uintptr(unsafe.Pointer(utf16Ptr(text))),
		uintptr(style|WS_CHILD|WS_VISIBLE), uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		hwndMain, uintptr(id), 0, 0)
	return r
}
func createControl(class, text string, style uint32, x, y, w, h int32, id int) uintptr {
	return createControlEx(0, class, text, style, x, y, w, h, id)
}
func createEdit(text string, numeric bool, x, y, w, h int32, id int, limit uintptr) uintptr {
	style := uint32(WS_TABSTOP | WS_BORDER | ES_AUTOHSCROLL)
	if numeric {
		style |= ES_NUMBER
	}
	hwnd := createControlEx(WS_EX_CLIENTEDGE, "EDIT", text, style, x, y, w, h, id)
	if limit > 0 {
		pSendMessageW.Call(hwnd, EM_SETLIMITTEXT, limit, 0)
	}
	return hwnd
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
func getControlText(hwnd uintptr) string {
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
	title := strings.TrimSpace(getControlText(hwnd))
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
func sendUnicodeText(text string) bool {
	if text == "" {
		return true
	}
	units, err := syscall.UTF16FromString(text)
	if err != nil {
		return false
	}
	units = units[:len(units)-1] // strip terminating NUL
	if len(units) == 0 {
		return true
	}
	inputs := make([]INPUT, 0, len(units)*2)
	for _, unit := range units {
		inputs = append(inputs,
			INPUT{Type: INPUT_KEYBOARD, Ki: KEYBDINPUT{WScan: unit, DwFlags: KEYEVENTF_UNICODE}},
			INPUT{Type: INPUT_KEYBOARD, Ki: KEYBDINPUT{WScan: unit, DwFlags: KEYEVENTF_UNICODE | KEYEVENTF_KEYUP}},
		)
	}
	r, _, _ := pSendInput.Call(uintptr(len(inputs)), uintptr(unsafe.Pointer(&inputs[0])), unsafe.Sizeof(inputs[0]))
	return r == uintptr(len(inputs))
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
func parseIntField(hwnd uintptr, label string, min, max int64) (int64, bool) {
	s := strings.TrimSpace(getControlText(hwnd))
	if s == "" {
		s = "0"
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < min || v > max {
		showMessage(appTitle, fmt.Sprintf("Το πεδίο «%s» πρέπει να είναι αριθμός από %d έως %d.", label, min, max), MB_OK|MB_ICONWARNING)
		return 0, false
	}
	return v, true
}
func readRunConfig() (RunConfig, bool) {
	preMs, ok := parseIntField(hPreDelay, "Delay πριν από κείμενο", 0, maxDelayMs)
	if !ok {
		return RunConfig{}, false
	}
	postMs, ok := parseIntField(hPostDelay, "Delay κειμένου → Enter", 0, maxDelayMs)
	if !ok {
		return RunConfig{}, false
	}
	lh, ok := parseIntField(hLoopHours, "Loop ώρες", 0, maxLoopHours)
	if !ok {
		return RunConfig{}, false
	}
	lm, ok := parseIntField(hLoopMinutes, "Loop λεπτά", 0, 59)
	if !ok {
		return RunConfig{}, false
	}
	ls, ok := parseIntField(hLoopSeconds, "Loop δευτερόλεπτα", 0, 59)
	if !ok {
		return RunConfig{}, false
	}
	lms, ok := parseIntField(hLoopMs, "Loop ms", 0, 999)
	if !ok {
		return RunConfig{}, false
	}
	loop := time.Duration(lh)*time.Hour + time.Duration(lm)*time.Minute + time.Duration(ls)*time.Second + time.Duration(lms)*time.Millisecond
	return RunConfig{
		preDelay:     time.Duration(preMs) * time.Millisecond,
		text:         getControlText(hText),
		postDelay:    time.Duration(postMs) * time.Millisecond,
		loopInterval: loop,
	}, true
}
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	ms := d.Round(time.Millisecond).Milliseconds()
	if ms < 0 {
		ms = 0
	}
	days := ms / 86400000
	ms %= 86400000
	hours := ms / 3600000
	ms %= 3600000
	mins := ms / 60000
	ms %= 60000
	secs := ms / 1000
	millis := ms % 1000
	if days > 0 {
		return fmt.Sprintf("%d ημ. %02d:%02d:%02d.%03d", days, hours, mins, secs, millis)
	}
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, mins, secs, millis)
}
func setRunControls(active bool) {
	for _, h := range []uintptr{
		hTarget, hRefresh, hHour, hMinute,
		hPreDelay, hText, hPostDelay,
		hLoopHours, hLoopMinutes, hLoopSeconds, hLoopMs,
		hStart, hTest,
	} {
		setEnabled(h, !active)
	}
	setEnabled(hCancel, active)
}
func failRun(message string) {
	cancelRun(false)
	showMessage(appTitle, message, MB_OK|MB_ICONWARNING)
}
func finishRun(message string) {
	pKillTimer.Call(hwndMain, timerID)
	runActive = false
	testMode = false
	setRunControls(false)
	updateTargetPreview()
	setText(hCountdown, "Απομένει: —")
	setText(hStatus, message)
}
func beginSequence(now time.Time) {
	phase = phaseWaitingText
	nextActionAt = now.Add(runConfig.preDelay)
	if runConfig.preDelay == 0 {
		performTextStep()
	}
}
func performTextStep() {
	if !runActive {
		return
	}
	if !isValidWindow(lockedTarget.hwnd) {
		failRun("Το επιλεγμένο παράθυρο έχει κλείσει. Η ακολουθία ακυρώθηκε.")
		return
	}
	setText(hStatus, "Ενεργοποίηση στόχου και εισαγωγή κειμένου…")
	if !activateTarget(lockedTarget.hwnd) {
		failRun("Τα Windows δεν επέτρεψαν την ενεργοποίηση του επιλεγμένου παραθύρου. Δεν στάλθηκε κείμενο ή Enter. Αν η εφαρμογή-στόχος εκτελείται ως διαχειριστής, εκτέλεσε και το Enter Scheduler ως διαχειριστής.")
		return
	}
	if runConfig.text != "" && !sendUnicodeText(runConfig.text) {
		failRun("Η αποστολή του κειμένου απέτυχε. Το Enter δεν στάλθηκε.")
		return
	}
	phase = phaseWaitingEnter
	nextActionAt = time.Now().Add(runConfig.postDelay)
	if runConfig.postDelay == 0 {
		performEnterStep()
	}
}
func performEnterStep() {
	if !runActive {
		return
	}
	if !isValidWindow(lockedTarget.hwnd) {
		failRun("Το επιλεγμένο παράθυρο έχει κλείσει. Το Enter δεν στάλθηκε.")
		return
	}
	setText(hStatus, "Επαλήθευση στόχου και αποστολή Enter…")
	if !activateTarget(lockedTarget.hwnd) {
		failRun("Το σωστό παράθυρο δεν μπόρεσε να ενεργοποιηθεί πριν από το Enter. Το Enter δεν στάλθηκε αλλού.")
		return
	}
	if !sendKeyboardEnter() {
		cancelRun(false)
		showMessage(appTitle, "Το παράθυρο ενεργοποιήθηκε, αλλά η αποστολή του Enter απέτυχε.", MB_OK|MB_ICONERROR)
		return
	}
	runCount++
	if testMode {
		finishRun(fmt.Sprintf("Η δοκιμή ολοκληρώθηκε στο: %s", lockedTarget.title))
		return
	}
	if runConfig.loopInterval <= 0 {
		finishRun(fmt.Sprintf("Η ακολουθία ολοκληρώθηκε στο: %s", lockedTarget.title))
		return
	}
	phase = phaseWaitingTrigger
	nextActionAt = time.Now().Add(runConfig.loopInterval)
	setText(hStatus, fmt.Sprintf("Εκτέλεση #%d ολοκληρώθηκε. Επόμενο loop σε %s.", runCount, formatDuration(runConfig.loopInterval)))
}
func startSchedule() {
	target, ok := currentTarget()
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
	cfg, ok := readRunConfig()
	if !ok {
		return
	}
	lockedTarget = target
	runConfig = cfg
	nextActionAt = nextExecution(h, m)
	phase = phaseWaitingTrigger
	runCount = 0
	testMode = false
	runActive = true
	setRunControls(true)
	setText(hSelected, "Κλειδωμένος στόχος: "+target.title)
	loopText := "μία εκτέλεση"
	if cfg.loopInterval > 0 {
		loopText = "loop κάθε " + formatDuration(cfg.loopInterval)
	}
	setText(hStatus, fmt.Sprintf("Προγραμματίστηκε για %s στις %s — %s", nextActionAt.Format("02/01/2006"), nextActionAt.Format("15:04"), loopText))
	pSetTimer.Call(hwndMain, timerID, timerPeriodMs, 0)
	updateRunner()
	checked, _, _ := pSendMessageW.Call(hMinimize, BM_GETCHECK, 0, 0)
	if checked == BST_CHECKED {
		pShowWindow.Call(hwndMain, SW_MINIMIZE)
	}
}
func startTest() {
	target, ok := currentTarget()
	if !ok {
		showMessage(appTitle, "Επίλεξε πρώτα ένα παράθυρο-στόχο.", MB_OK|MB_ICONWARNING)
		return
	}
	cfg, ok := readRunConfig()
	if !ok {
		return
	}
	cfg.loopInterval = 0 // A test always runs exactly once.
	lockedTarget = target
	runConfig = cfg
	nextActionAt = time.Now()
	phase = phaseWaitingTrigger
	runCount = 0
	testMode = true
	runActive = true
	setRunControls(true)
	setText(hSelected, "Δοκιμή στον στόχο: "+target.title)
	setText(hStatus, "Έναρξη δοκιμής της πλήρους ακολουθίας…")
	pSetTimer.Call(hwndMain, timerID, timerPeriodMs, 0)
	updateRunner()
}
func cancelRun(notify bool) {
	if runActive {
		pKillTimer.Call(hwndMain, timerID)
	}
	runActive = false
	testMode = false
	setRunControls(false)
	updateTargetPreview()
	setText(hCountdown, "Απομένει: —")
	if notify {
		setText(hStatus, "Ο προγραμματισμός ακυρώθηκε.")
	}
}
func updateRunner() {
	if !runActive {
		return
	}
	if !isValidWindow(lockedTarget.hwnd) {
		cancelRun(false)
		showMessage(appTitle, "Το επιλεγμένο παράθυρο δεν υπάρχει πλέον. Πάτησε «Ανανέωση» και επίλεξέ το ξανά.", MB_OK|MB_ICONWARNING)
		return
	}
	d := time.Until(nextActionAt)
	if d > 0 {
		switch phase {
		case phaseWaitingTrigger:
			if runCount > 0 {
				setText(hCountdown, "Επόμενο loop σε: "+formatDuration(d))
			} else {
				setText(hCountdown, "Απομένει: "+formatDuration(d))
			}
		case phaseWaitingText:
			setText(hCountdown, "Delay πριν από κείμενο: "+formatDuration(d))
		case phaseWaitingEnter:
			setText(hCountdown, "Delay πριν από Enter: "+formatDuration(d))
		}
		return
	}
	switch phase {
	case phaseWaitingTrigger:
		beginSequence(time.Now())
	case phaseWaitingText:
		performTextStep()
	case phaseWaitingEnter:
		performEnterStep()
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
			cancelRun(true)
		case idTest:
			startTest()
		}
		return 0
	case WM_TIMER:
		if wParam == timerID {
			updateRunner()
			return 0
		}
	case WM_CLOSE:
		if runActive {
			if showMessage(appTitle, "Υπάρχει ενεργός προγραμματισμός/δοκιμή. Θέλεις να κλείσεις το πρόγραμμα και να τον ακυρώσεις;", MB_YESNO|MB_ICONWARNING) != IDYES {
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
	createControl("STATIC", "Παράθυρο-στόχος:", SS_LEFT, 24, 18, 170, 24, 0)
	hTarget = createControl("COMBOBOX", "", WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST|CBS_HASSTRINGS, 24, 44, 495, 300, idTargetCombo)
	hRefresh = createControl("BUTTON", "Ανανέωση", WS_TABSTOP|BS_PUSHBUTTON, 531, 43, 145, 30, idRefresh)
	hSelected = createControl("STATIC", "Επιλεγμένο: —", SS_LEFT, 24, 80, 652, 22, 0)

	createControl("STATIC", "Ώρα πρώτης εκτέλεσης:", SS_LEFT, 24, 112, 180, 24, 0)
	createControl("STATIC", "Ώρα", SS_LEFT, 210, 112, 36, 22, 0)
	hHour = createControl("COMBOBOX", "", WS_TABSTOP|CBS_DROPDOWNLIST|CBS_HASSTRINGS, 247, 108, 70, 300, idHourCombo)
	createControl("STATIC", ":", SS_LEFT, 322, 112, 12, 22, 0)
	hMinute = createControl("COMBOBOX", "", WS_TABSTOP|CBS_DROPDOWNLIST|CBS_HASSTRINGS, 336, 108, 70, 500, idMinuteCombo)
	for i := 0; i < 24; i++ {
		addComboItem(hHour, fmt.Sprintf("%02d", i))
	}
	for i := 0; i < 60; i++ {
		addComboItem(hMinute, fmt.Sprintf("%02d", i))
	}
	now := time.Now().Add(time.Minute)
	selectCombo(hHour, now.Hour())
	selectCombo(hMinute, now.Minute())

	createControl("STATIC", "Delay πριν από κείμενο:", SS_LEFT, 24, 154, 185, 24, 0)
	hPreDelay = createEdit("0", true, 210, 149, 120, 28, idPreDelay, 10)
	createControl("STATIC", "ms", SS_LEFT, 338, 154, 35, 22, 0)

	createControl("STATIC", "Κείμενο:", SS_LEFT, 24, 195, 85, 24, 0)
	hText = createEdit("", false, 110, 190, 566, 28, idText, maxTextUTF16Units)

	createControl("STATIC", "Delay κειμένου → Enter:", SS_LEFT, 24, 236, 185, 24, 0)
	hPostDelay = createEdit("250", true, 210, 231, 120, 28, idPostDelay, 10)
	createControl("STATIC", "ms", SS_LEFT, 338, 236, 35, 22, 0)

	createControl("STATIC", "Loop κάθε:", SS_LEFT, 24, 278, 82, 24, 0)
	hLoopHours = createEdit("0", true, 110, 273, 68, 28, idLoopHours, 6)
	createControl("STATIC", "ώ", SS_LEFT, 183, 278, 20, 22, 0)
	hLoopMinutes = createEdit("0", true, 210, 273, 58, 28, idLoopMinutes, 2)
	createControl("STATIC", "λ", SS_LEFT, 273, 278, 20, 22, 0)
	hLoopSeconds = createEdit("0", true, 300, 273, 58, 28, idLoopSeconds, 2)
	createControl("STATIC", "δ", SS_LEFT, 363, 278, 20, 22, 0)
	hLoopMs = createEdit("0", true, 390, 273, 72, 28, idLoopMillis, 3)
	createControl("STATIC", "ms", SS_LEFT, 468, 278, 28, 22, 0)
	createControl("STATIC", "(όλα 0 = μία εκτέλεση)", SS_LEFT, 506, 278, 170, 22, 0)

	hCountdown = createControl("STATIC", "Απομένει: —", SS_LEFT, 24, 321, 652, 26, 0)
	hStart = createControl("BUTTON", "Έναρξη", WS_TABSTOP|BS_DEFPUSHBUTTON, 24, 358, 210, 42, idStart)
	hCancel = createControl("BUTTON", "Ακύρωση", WS_TABSTOP|BS_PUSHBUTTON, 245, 358, 210, 42, idCancel)
	hTest = createControl("BUTTON", "Δοκιμή ακολουθίας", WS_TABSTOP|BS_PUSHBUTTON, 466, 358, 210, 42, idTest)
	hMinimize = createControl("BUTTON", "Ελαχιστοποίηση μετά την έναρξη", WS_TABSTOP|BS_AUTOCHECKBOX, 24, 416, 330, 27, idMinimize)
	hStatus = createControl("STATIC", "Φόρτωση παραθύρων…", SS_LEFT, 24, 460, 652, 58, 0)

	applyFont(
		hTarget, hRefresh, hSelected, hHour, hMinute,
		hPreDelay, hText, hPostDelay,
		hLoopHours, hLoopMinutes, hLoopSeconds, hLoopMs,
		hCountdown, hStart, hCancel, hTest, hMinimize, hStatus,
	)
	setRunControls(false)
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
	text := fmt.Sprintf("Enter Scheduler v4 panic: %v\nTime: %s\n\n%s\n", v, time.Now().Format(time.RFC3339), debug.Stack())
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

	width, height := int32(720), int32(570)
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
