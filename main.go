// Compares the base.yaml and head.yaml of every case in ./cases with libopenapi's
// what-changed and prints how many breaking changes it reports.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pb33f/libopenapi"
)

// What a direction-aware classifier should report for each case: in every case a property is
// added to a schema that is only used by a response and listed in `required`. Adding a required
// property to a response never breaks a client, so the expected number is always 0.
const expectedBreaking = 0

func main() {
	dirs, err := filepath.Glob("cases/*")
	if err != nil || len(dirs) == 0 {
		fmt.Fprintln(os.Stderr, "no cases found, run this from the repository root")
		os.Exit(2)
	}
	sort.Strings(dirs)

	mismatches := 0
	fmt.Printf("%-34s %-8s %-9s %s\n", "case", "changes", "breaking", "expected")
	for _, dir := range dirs {
		base := mustLoad(filepath.Join(dir, "base.yaml"))
		head := mustLoad(filepath.Join(dir, "head.yaml"))

		changes, err := libopenapi.CompareDocuments(base, head)
		if err != nil || changes == nil {
			fmt.Fprintf(os.Stderr, "%s: compare failed: %v\n", dir, err)
			os.Exit(2)
		}

		got := changes.TotalBreakingChanges()
		verdict := "ok"
		if got != expectedBreaking {
			verdict = "UNEXPECTED"
			mismatches++
		}
		fmt.Printf("%-34s %-8d %-9d %d  %s\n", filepath.Base(dir), changes.TotalChanges(), got, expectedBreaking, verdict)
	}

	if mismatches > 0 {
		fmt.Printf("\n%d of %d cases are reported as breaking although only a response gained a required property.\n", mismatches, len(dirs))
		os.Exit(1)
	}
}

func mustLoad(path string) libopenapi.Document {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	doc, err := libopenapi.NewDocument(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
		os.Exit(2)
	}
	return doc
}
