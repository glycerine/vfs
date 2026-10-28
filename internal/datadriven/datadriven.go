package datadriven

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

type CmdArg struct {
	Key  string
	Vals []string
}

func (a CmdArg) String() string {
	if len(a.Vals) == 0 {
		return a.Key
	}
	if len(a.Vals) == 1 {
		return a.Key + "=" + a.Vals[0]
	}
	return a.Key + "=(" + strings.Join(a.Vals, ", ") + ")"
}

type TestData struct {
	Pos      string
	Cmd      string
	CmdArgs  []CmdArg
	Input    string
	Expected string
}

func (td *TestData) Arg(key string) (CmdArg, bool) {
	for _, arg := range td.CmdArgs {
		if arg.Key == key {
			return arg, true
		}
	}
	return CmdArg{}, false
}

func (td *TestData) ScanArgs(t testing.TB, key string, dests ...any) {
	t.Helper()
	arg, ok := td.Arg(key)
	if !ok {
		td.Fatalf(t, "missing argument: %s", key)
	}
	if len(arg.Vals) != len(dests) {
		td.Fatalf(t, "argument %q has %d values, expected %d", key, len(arg.Vals), len(dests))
	}
	for i := range dests {
		if err := scanValue(arg.Vals[i], dests[i]); err != nil {
			td.Fatalf(t, "argument %q: %v", key, err)
		}
	}
}

func scanValue(value string, dest any) error {
	switch d := dest.(type) {
	case *string:
		*d = value
	case *int:
		v, err := strconv.Atoi(value)
		if err != nil {
			return err
		}
		*d = v
	case *int64:
		v, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		*d = v
	case *bool:
		v, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		*d = v
	default:
		return fmt.Errorf("unsupported destination %T", dest)
	}
	return nil
}

func (td *TestData) Fatalf(t testing.TB, format string, args ...any) {
	t.Helper()
	t.Fatalf("%s: %s", td.Pos, fmt.Sprintf(format, args...))
}

func RunTest(t *testing.T, path string, fn func(*testing.T, *TestData) string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	var lines []string
	for s.Scan() {
		lines = append(lines, s.Text())
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(lines); {
		for i < len(lines) && (strings.TrimSpace(lines[i]) == "" || strings.HasPrefix(strings.TrimSpace(lines[i]), "#")) {
			i++
		}
		if i == len(lines) {
			break
		}
		lineNo := i + 1
		cmd, args, err := parseLine(lines[i])
		if err != nil {
			t.Fatalf("%s:%d: %v", path, lineNo, err)
		}
		i++
		var input []string
		for i < len(lines) && lines[i] != "----" {
			input = append(input, lines[i])
			i++
		}
		if i < len(lines) {
			i++
		}
		var expected []string
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" {
			expected = append(expected, lines[i])
			i++
		}
		td := &TestData{
			Pos: fmt.Sprintf("%s:%d", path, lineNo), Cmd: cmd, CmdArgs: args,
			Input: strings.TrimSpace(strings.Join(input, "\n")),
		}
		if len(expected) > 0 {
			td.Expected = strings.Join(expected, "\n") + "\n"
		}
		actual := fn(t, td)
		if actual != "" && !strings.HasSuffix(actual, "\n") {
			actual += "\n"
		}
		if actual != td.Expected {
			t.Fatalf("%s: %s\nexpected:\n%sfound:\n%s", td.Pos, td.Cmd, td.Expected, actual)
		}
	}
}

func parseLine(line string) (string, []CmdArg, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", nil, nil
	}
	args := make([]CmdArg, 0, len(fields)-1)
	for _, field := range fields[1:] {
		key, value, found := strings.Cut(field, "=")
		if !found {
			args = append(args, CmdArg{Key: field})
			continue
		}
		value = strings.Trim(value, "()")
		vals := []string{value}
		if strings.Contains(value, ",") {
			vals = strings.Split(value, ",")
		}
		for i := range vals {
			vals[i] = strings.TrimSpace(vals[i])
		}
		args = append(args, CmdArg{Key: key, Vals: vals})
	}
	return fields[0], args, nil
}
