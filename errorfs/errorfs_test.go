package errorfs

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glycerine/vfs/internal/datadriven"
)

func TestErrorFS(t *testing.T) {
	var sb strings.Builder
	datadriven.RunTest(t, "testdata/errorfs", func(t *testing.T, td *datadriven.TestData) string {
		sb.Reset()
		switch td.Cmd {
		case "parse-dsl":
			for l := range strings.SplitSeq(td.Input, "\n") {
				inj, err := ParseDSL(l)
				if err != nil {
					fmt.Fprintf(&sb, "parsing err: %s\n", err)
				} else {
					fmt.Fprintf(&sb, "%s\n", inj.String())
				}
			}
			return sb.String()
		default:
			return fmt.Sprintf("unrecognized command %q", td.Cmd)
		}
	})
}
