//go:build !web

package main
import (
	"embed"
	"flag"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	testOne := flag.Bool("test-one", false, "run 1 account test")
	batch := flag.Bool("batch", false, "run batch in CLI mode")
	count := flag.Int("count", 0, "batch target success count")
	workers := flag.Int("workers", 0, "batch parallel workers")
	importPath := flag.String("import", "", "import hotmail accounts from a text file into the pool")
	preflight := flag.Bool("preflight", false, "check every account in the pool and exit")
	checkinAll := flag.Bool("checkin-all", false, "claim the daily check-in for every saved account and exit")
	mailScan := flag.Bool("mail-scan", false, "bulk-fetch inboxes: detect Cursor-registered mailboxes and pull latest messages")
	mailScanWorkers := flag.Int("mail-scan-workers", 0, "parallel workers for mail-scan")
	mailScanLimit := flag.Int("mail-scan-limit", 0, "only scan the first N pool accounts (0 = all)")
	supervised := flag.Bool("supervised", false, "supervised batch: auto-restart on failure until target reached or pool exhausted")
	flag.Parse()
	if *testOne {
		runCLITestOne()
		return
	}
	if *importPath != "" {
		runCLIImport(*importPath)
		return
	}
	if *preflight {
		runCLIPreflight(*workers)
		return
	}
	if *checkinAll {
		runCLICheckinAll()
		return
	}
	if *mailScan {
		runCLIMailScanFull(*mailScanWorkers, *mailScanLimit)
		return
	}
	if *supervised {
		runCLISupervised(*count, *workers)
		return
	}
	if *batch {
		runCLIBatch(*count, *workers)
		return
	}
	app := NewApp()
	err := wails.Run(&options.App{
		Title:  "MiniMax 纯 Go 协议注册器",
		Width:  1440,
		Height: 900,
		MinWidth:  1180,
		MinHeight: 720,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 20, G: 20, B: 23, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []any{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
