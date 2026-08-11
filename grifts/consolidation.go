package grifts

import (
	"creaves/actions"
	"creaves/models"
	"fmt"

	"github.com/gobuffalo/grift/grift"
)

var _ = grift.Namespace("consolidation", func() {
	grift.Desc("process", "Process unprocessed events into consolidated view")
	grift.Add("process", func(c *grift.Context) error {
		return processEvents(c, false)
	})

	grift.Desc("rebuild", "Rebuild the consolidated view from scratch")
	grift.Add("rebuild", func(c *grift.Context) error {
		return processEvents(c, true)
	})

	grift.Desc("stats", "Show consolidation statistics")
	grift.Add("stats", func(c *grift.Context) error {
		return showStats(c)
	})
})

func processEvents(c *grift.Context, rebuild bool) error {
	processor := actions.NewEventProcessor(models.DB)

	var processedCount int
	var err error

	if rebuild {
		fmt.Println("Rebuilding consolidated view from scratch...")
		processedCount, err = processor.ProcessAllEvents()
	} else {
		fmt.Println("Processing unprocessed events...")
		processedCount, err = processor.ProcessUnprocessedEvents()
	}

	if err != nil {
		return fmt.Errorf("failed to process events: %v", err)
	}

	fmt.Printf("Processed %d events\n", processedCount)

	// Show stats
	stats, err := processor.GetConsolidatedStats()
	if err != nil {
		return fmt.Errorf("failed to get stats: %v", err)
	}

	fmt.Printf("\nConsolidated View Statistics:\n")
	fmt.Printf("  Total animals: %d\n", stats["total_animals"])
	fmt.Printf("  Unprocessed events: %d\n", stats["unprocessed_events"])

	if byStatus, ok := stats["by_status"].(map[string]int); ok {
		fmt.Printf("  By status:\n")
		for status, count := range byStatus {
			fmt.Printf("    %s: %d\n", status, count)
		}
	}

	if byInstance, ok := stats["by_instance"].(map[string]int); ok {
		fmt.Printf("  By instance:\n")
		for instance, count := range byInstance {
			fmt.Printf("    %s: %d\n", instance, count)
		}
	}

	return nil
}

func showStats(c *grift.Context) error {
	processor := actions.NewEventProcessor(models.DB)

	stats, err := processor.GetConsolidatedStats()
	if err != nil {
		return fmt.Errorf("failed to get stats: %v", err)
	}

	fmt.Printf("Consolidated View Statistics:\n")
	fmt.Printf("  Total animals: %d\n", stats["total_animals"])
	fmt.Printf("  Unprocessed events: %d\n", stats["unprocessed_events"])

	if byStatus, ok := stats["by_status"].(map[string]int); ok {
		fmt.Printf("  By status:\n")
		for status, count := range byStatus {
			fmt.Printf("    %s: %d\n", status, count)
		}
	}

	if byInstance, ok := stats["by_instance"].(map[string]int); ok {
		fmt.Printf("  By instance:\n")
		for instance, count := range byInstance {
			fmt.Printf("    %s: %d\n", instance, count)
		}
	}

	return nil
}
