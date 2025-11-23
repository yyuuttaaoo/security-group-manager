package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/alibabacloud-go/tea/tea"
	"github.com/yyuuttaaoo/security-group-manager/pkg/config"
	"github.com/yyuuttaaoo/security-group-manager/pkg/logger"
	"github.com/yyuuttaaoo/security-group-manager/pkg/manager"
	"github.com/yyuuttaaoo/security-group-manager/pkg/utils"
)

func _main(args []*string) (_err error) {
	// Load configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		if cfg == nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}
	}

	// Setup logger
	logger.Setup(cfg.Log)
	slog.Info("Starting demo...", "config", cfg)

	currentIP, _err := utils.GetCurrentIP()
	if _err != nil {
		return _err
	}

	regions := []string{"cn-hongkong", "ap-northeast-1", "us-west-1"}

	// Parse flags
	groupName := "default"
	if len(args) > 0 && *args[0] == "-group" {
		if len(args) > 1 {
			groupName = *args[1]
		}
	}

	for _, region := range regions {
		slog.Info("Processing Region", "region", region, "group", groupName)

		// For demo, we still output to stdout (which is what logger.LogWriter is by default, or file)
		// But ProcessRegion writes to the writer.
		// If we want to see output in console AND file (if configured), we might need MultiWriter.
		// But if config is stdout, MultiWriter(stdout, stdout) duplicates.
		// Let's just use logger.LogWriter. If it's file, demo output goes to file.
		// If user wants both, they can use `tee`.

		// We need to create a logger that writes to logger.LogWriter
		// slog.Default() writes to stdout/stderr usually, but we want to respect config.
		// logger.Setup sets the default logger, so slog.Default() should be correct IF logger.Setup was called.
		// Yes, logger.Setup is called above.
		err := manager.ProcessRegion(region, currentIP, groupName, slog.Default())

		if err != nil {
			// Log error but continue to next region
			slog.Error("Error processing region", "region", region, "error", err)

			// Try to parse SDK error for more details if possible
			var sdkError = &tea.SDKError{}
			if _t, ok := err.(*tea.SDKError); ok {
				sdkError = _t
				fmt.Println(tea.StringValue(sdkError.Message))
				var data interface{}
				d := json.NewDecoder(strings.NewReader(tea.StringValue(sdkError.Data)))
				d.Decode(&data)
				if m, ok := data.(map[string]interface{}); ok {
					recommend, _ := m["Recommend"]
					fmt.Println(recommend)
				}
			}
		}
	}

	return nil
}

func main() {
	err := _main(tea.StringSlice(os.Args[1:]))
	if err != nil {
		panic(err)
	}
}
