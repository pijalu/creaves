package grifts

import (
	"creaves/actions"
	"creaves/models"
	"fmt"
	"time"

	"github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// eventSnapshot creates events for all animals in the database
// This is useful for initial export or reprocessing after errors
var _ = grift.Desc("event:snapshot", "Creates event stream snapshot for all animals (past and present)")
var _ = grift.Add("event:snapshot", func(c *grift.Context) error {
	return models.DB.Transaction(func(tx *pop.Connection) error {
		// Load config to ensure event stream is enabled
		if actions.CurrentConfigGet() == nil {
			if _, err := actions.LoadConfig(tx); err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
		}

		if !actions.IsEventStreamEnabled() {
			fmt.Println("Event stream is disabled. Enable it in Configuration to create snapshot.")
			return nil
		}

		fmt.Printf("Creating event snapshot for instance: %s\n", actions.GetInstanceID())
		fmt.Println("This will create events for all animals in the database...")

		// Get all animals with their relationships
		animals := &models.Animals{}
		if err := tx.Eager().All(animals); err != nil {
			return fmt.Errorf("failed to load animals: %w", err)
		}

		fmt.Printf("Found %d animals to process\n", len(*animals))

		created := 0
		skipped := 0
		errors := 0

		for i, animal := range *animals {
			if i%100 == 0 && i > 0 {
				fmt.Printf("Processed %d/%d animals...\n", i, len(*animals))
			}

			// Check if events already exist for this animal
			exists, err := tx.Where("animal_id = ? AND instance_id = ?", animal.ID, actions.GetInstanceID()).
				Exists(&models.EventStream{})
			if err != nil {
				fmt.Printf("Error checking existing events for animal %d: %v\n", animal.ID, err)
				errors++
				continue
			}

			if exists {
				skipped++
				continue
			}

			// Create discovery event
			if err := actions.PublishAnimalDiscoveredEvent(tx, &animal, nil); err != nil {
				fmt.Printf("Error creating discovery event for animal %d: %v\n", animal.ID, err)
				errors++
				continue
			}

			// If animal has outtake, create appropriate outtake event
			if animal.OuttakeID.Valid {
				// Load outtake to determine type
				outtake := &models.Outtake{}
				if err := tx.Find(outtake, animal.OuttakeID); err != nil {
					fmt.Printf("Error loading outtake for animal %d: %v\n", animal.ID, err)
					errors++
					continue
				}

				// Load outtake type
				if outtake.Type.ID != uuid.Nil {
					if err := tx.Load(outtake, "Type"); err != nil {
						fmt.Printf("Error loading outtake type for animal %d: %v\n", animal.ID, err)
						errors++
						continue
					}
				}

				// Determine if released or died based on outtake type
				if outtake.Type.Dead {
					if err := actions.PublishAnimalDiedEvent(tx, &animal, nil); err != nil {
						fmt.Printf("Error creating died event for animal %d: %v\n", animal.ID, err)
						errors++
						continue
					}
				} else {
					if err := actions.PublishAnimalReleasedEvent(tx, &animal, nil); err != nil {
						fmt.Printf("Error creating released event for animal %d: %v\n", animal.ID, err)
						errors++
						continue
					}
				}
			}

			created++
		}

		fmt.Println("\nSnapshot complete!")
		fmt.Printf("  Created: %d events\n", created)
		fmt.Printf("  Skipped: %d animals (already had events)\n", skipped)
		fmt.Printf("  Errors:  %d\n", errors)

		return nil
	})
})

// eventSnapshotForce creates events for all animals, even if they already have events
// Use with caution - may create duplicate events
var _ = grift.Desc("event:snapshot:force", "Force creates events for all animals (may create duplicates)")
var _ = grift.Add("event:snapshot:force", func(c *grift.Context) error {
	return models.DB.Transaction(func(tx *pop.Connection) error {
		// Load config to ensure event stream is enabled
		if actions.CurrentConfigGet() == nil {
			if _, err := actions.LoadConfig(tx); err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
		}

		if !actions.IsEventStreamEnabled() {
			fmt.Println("Event stream is disabled. Enable it in Configuration to create snapshot.")
			return nil
		}

		fmt.Printf("Creating FORCED event snapshot for instance: %s\n", actions.GetInstanceID())
		fmt.Println("WARNING: This will create events even if they already exist!")
		fmt.Println("This is useful for reprocessing after errors.")
		fmt.Println()

		// Get all animals with their relationships
		animals := &models.Animals{}
		if err := tx.Eager().All(animals); err != nil {
			return fmt.Errorf("failed to load animals: %w", err)
		}

		fmt.Printf("Found %d animals to process\n", len(*animals))

		created := 0
		errors := 0

		for i, animal := range *animals {
			if i%100 == 0 && i > 0 {
				fmt.Printf("Processed %d/%d animals...\n", i, len(*animals))
			}

			// Always create discovery event (may create duplicate)
			if err := actions.PublishAnimalDiscoveredEvent(tx, &animal, nil); err != nil {
				fmt.Printf("Error creating discovery event for animal %d: %v\n", animal.ID, err)
				errors++
				continue
			}

			// If animal has outtake, create appropriate outtake event
			if animal.OuttakeID.Valid {
				// Load outtake to determine type
				outtake := &models.Outtake{}
				if err := tx.Find(outtake, animal.OuttakeID); err != nil {
					fmt.Printf("Error loading outtake for animal %d: %v\n", animal.ID, err)
					errors++
					continue
				}

				// Load outtake type
				if outtake.Type.ID != uuid.Nil {
					if err := tx.Load(outtake, "Type"); err != nil {
						fmt.Printf("Error loading outtake type for animal %d: %v\n", animal.ID, err)
						errors++
						continue
					}
				}

				// Determine if released or died based on outtake type
				if outtake.Type.Dead {
					if err := actions.PublishAnimalDiedEvent(tx, &animal, nil); err != nil {
						fmt.Printf("Error creating died event for animal %d: %v\n", animal.ID, err)
						errors++
						continue
					}
				} else {
					if err := actions.PublishAnimalReleasedEvent(tx, &animal, nil); err != nil {
						fmt.Printf("Error creating released event for animal %d: %v\n", animal.ID, err)
						errors++
						continue
					}
				}
			}

			created++
		}

		fmt.Println("\nForced snapshot complete!")
		fmt.Printf("  Created: %d events\n", created)
		fmt.Printf("  Errors:  %d\n", errors)
		fmt.Println("\nNOTE: Duplicates may have been created. Use event:snapshot:clean to remove duplicates.")

		return nil
	})
})

// eventSnapshotClean removes duplicate events for the same animal
var _ = grift.Desc("event:snapshot:clean", "Removes duplicate events keeping only the most recent per animal")
var _ = grift.Add("event:snapshot:clean", func(c *grift.Context) error {
	return models.DB.Transaction(func(tx *pop.Connection) error {
		fmt.Println("Cleaning duplicate events...")
		fmt.Println("Keeping only the most recent event per animal per event type.")

		// This is a placeholder - actual implementation would require raw SQL
		// to identify and remove duplicates based on animal_id, event_type, and timestamp
		fmt.Println("Not yet implemented - requires raw SQL to handle duplicates properly")

		return nil
	})
})

// eventSnapshotStats shows statistics about the event stream
var _ = grift.Desc("event:snapshot:stats", "Shows statistics about the event stream")
var _ = grift.Add("event:snapshot:stats", func(c *grift.Context) error {
	return models.DB.Transaction(func(tx *pop.Connection) error {
		// Load config
		if actions.CurrentConfigGet() == nil {
			if _, err := actions.LoadConfig(tx); err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
		}

		fmt.Printf("Event Stream Statistics for Instance: %s\n", actions.GetInstanceID())
		fmt.Printf("Event Stream Enabled: %v\n\n", actions.IsEventStreamEnabled())

		// Count total events
		var totalEvents int
		if err := tx.RawQuery("SELECT COUNT(*) FROM event_streams WHERE instance_id = ?", actions.GetInstanceID()).First(&totalEvents); err != nil {
			return fmt.Errorf("failed to count events: %w", err)
		}
		fmt.Printf("Total Events: %d\n", totalEvents)

		// Count by event type
		eventTypes := []string{
			string(models.EventTypeAnimalDiscovered),
			string(models.EventTypeAnimalStatusChanged),
			string(models.EventTypeAnimalReleased),
			string(models.EventTypeAnimalDied),
		}

		fmt.Println("\nEvents by Type:")
		for _, eventType := range eventTypes {
			var count int
			if err := tx.RawQuery("SELECT COUNT(*) FROM event_streams WHERE instance_id = ? AND event_type = ?", actions.GetInstanceID(), eventType).First(&count); err != nil {
				fmt.Printf("  %s: error counting\n", eventType)
				continue
			}
			fmt.Printf("  %s: %d\n", eventType, count)
		}

		// Count unique animals with events
		var uniqueAnimals int
		if err := tx.RawQuery("SELECT COUNT(DISTINCT animal_id) FROM event_streams WHERE instance_id = ?", actions.GetInstanceID()).First(&uniqueAnimals); err != nil {
			fmt.Printf("\nUnique Animals: error counting\n")
		} else {
			fmt.Printf("\nUnique Animals with Events: %d\n", uniqueAnimals)
		}

		// Get date range
		var oldestEvent time.Time
		var newestEvent time.Time
		if err := tx.RawQuery("SELECT MIN(created_at) FROM event_streams WHERE instance_id = ?", actions.GetInstanceID()).First(&oldestEvent); err == nil {
			if err := tx.RawQuery("SELECT MAX(created_at) FROM event_streams WHERE instance_id = ?", actions.GetInstanceID()).First(&newestEvent); err == nil {
				fmt.Printf("Date Range: %s to %s\n", oldestEvent.Format(time.RFC3339), newestEvent.Format(time.RFC3339))
			}
		}

		return nil
	})
})
