//go:build cli

package main

import (
	"flag"
	"log"
)

// cliMain 是纯命令行入口（不含 Wails GUI），用于 VPS 上跑批量/测试。
func main() {
	testOne := flag.Bool("test-one", false, "run 1 account test")
	batch := flag.Bool("batch", false, "run batch in CLI mode")
	count := flag.Int("count", 0, "batch target success count")
	workers := flag.Int("workers", 0, "batch parallel workers")
	importPath := flag.String("import", "", "import hotmail accounts from a text file into the pool")
	preflight := flag.Bool("preflight", false, "check every account in the pool and exit")
	checkinAll := flag.Bool("checkin-all", false, "claim the daily check-in for every saved account and exit")
	mailScan := flag.Bool("mail-scan", false, "bulk-fetch inboxes")
	mailScanWorkers := flag.Int("mail-scan-workers", 0, "parallel workers for mail-scan")
	mailScanLimit := flag.Int("mail-scan-limit", 0, "only scan the first N pool accounts")
	supervised := flag.Bool("supervised", false, "supervised batch")
	testCursor := flag.Bool("test-cursor", false, "run one Cursor registration test")
	cursorBatch := flag.Bool("cursor-batch", false, "run Cursor binding in batch from the hotmail pool")
	cursorCount := flag.Int("cursor-count", 1, "how many Cursor accounts to bind")
	cursorPanel := flag.String("cursor-panel", "", "panel base url for cursor test")
	cursorKey := flag.String("cursor-key", "", "panel key for cursor test")
	cursorEmail := flag.String("cursor-email", "", "use a specific hotmail account for the cursor test")
	cursorProxy := flag.String("cursor-proxy", "", "proxy used by the Cursor browser (empty = use config proxy)")
	platform := flag.String("platform", "", "run platform: minimax | cursor | both")
	cursorTargetCount := flag.Int("cursor-target", 0, "cursor target count when -platform=cursor")
	flag.Parse()
	if *cursorProxy != "" {
		cursorBrowserProxy = *cursorProxy
	}
	if *platform != "" {
		cliPlatform = *platform
	}
	if *cursorTargetCount > 0 {
		cliCursorCount = *cursorTargetCount
	}

	switch {
	case *cursorBatch:
		runCLICursorBatch(*cursorPanel, *cursorKey, *cursorCount)
	case *testCursor:
		runCLITestCursor(*cursorPanel, *cursorKey, *cursorEmail)
	case *importPath != "":
		runCLIImport(*importPath)
	case *preflight:
		runCLIPreflight(*workers)
	case *checkinAll:
		runCLICheckinAll()
	case *mailScan:
		runCLIMailScanFull(*mailScanWorkers, *mailScanLimit)
	case *supervised:
		if cliPlatform == "cursor" {
			runCLICursorBatch("", "", cliCursorCount)
			return
		}
		runCLISupervised(*count, *workers)
	case *batch:
		if cliPlatform == "cursor" {
			runCLICursorBatch("", "", cliCursorCount)
			return
		}
		runCLIBatch(*count, *workers)
	case *testOne:
		runCLITestOne()
	default:
		log.Fatal("没有指定要跑的动作，试试 -batch / -supervised / -test-cursor")
	}
}
