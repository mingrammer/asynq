// Copyright 2020 Kentaro Hibino. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cronCmd)
	cronCmd.AddCommand(cronListCmd)
	cronCmd.AddCommand(cronHistoryCmd)
}

var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Manage cron",
}

var cronListCmd = &cobra.Command{
	Use:   "ls",
	Short: "List cron entries",
	Run:   cronList,
}

var cronHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Show history of each cron tasks",
	Args:  cobra.MinimumNArgs(1),
	Run:   cronHistory,
}

func cronList(cmd *cobra.Command, args []string) {
	r := createRDB()

	entries, err := r.ListSchedulerEntries()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if len(entries) == 0 {
		fmt.Println("No scheduler entries")
		return
	}

	// Sort entries by spec.
	sort.Slice(entries, func(i, j int) bool {
		x, y := entries[i], entries[j]
		return x.Spec < y.Spec
	})

	cols := []string{"EntryID", "Spec", "Type", "Payload", "Options", "Next", "Prev"}
	printRows := func(w io.Writer, tmpl string) {
		for _, e := range entries {
			fmt.Fprintf(w, tmpl, e.ID, e.Spec, e.Type, e.Payload, e.Opts, e.Next, e.Prev)
		}
	}
	printTable(cols, printRows)
}

func cronHistory(cmd *cobra.Command, args []string) {
	r := createRDB()
	for i, entryID := range args {
		if i > 0 {
			fmt.Printf("\n%s\n", separator)
		}
		fmt.Println()

		fmt.Printf("Entry: %s\n\n", entryID)

		events, err := r.ListSchedulerEnqueueEvents(entryID)
		if err != nil {
			fmt.Printf("error: %v\n", err)
			continue
		}
		if len(events) == 0 {
			fmt.Printf("No scheduler enqueue events found for entry: %s\n", entryID)
			continue
		}

		// Sort entries by enqueuedAt timestamp.
		sort.Slice(events, func(i, j int) bool {
			x, y := events[i], events[j]
			return x.EnqueuedAt.Unix() > y.EnqueuedAt.Unix()
		})

		cols := []string{"TaskID", "EnqueuedAt"}
		printRows := func(w io.Writer, tmpl string) {
			for _, e := range events {
				fmt.Fprintf(w, tmpl, e.TaskID, e.EnqueuedAt)
			}
		}
		printTable(cols, printRows)
	}
}