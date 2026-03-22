package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/itchyny/gojq"
)

func jqFlagUsage(fields []string) string {
	return fmt.Sprintf("Filter JSON output using a jq expression (fields: %s)", strings.Join(fields, ", "))
}

func WriteJSON(w io.Writer, data any, jqExpr string, fields []string) error {
	if jqExpr == "" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}

	query, err := gojq.Parse(jqExpr)
	if err != nil {
		return fmt.Errorf("jq parse error: %w", err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		return fmt.Errorf("jq compile error: %w", err)
	}

	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	var normalized any
	if err := json.Unmarshal(b, &normalized); err != nil {
		return fmt.Errorf("json unmarshal: %w", err)
	}

	iter := code.Run(normalized)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := v.(error); isErr {
			if len(fields) > 0 {
				return fmt.Errorf("jq error: %w (available fields: %s)", err, strings.Join(fields, ", "))
			}
			return fmt.Errorf("jq error: %w", err)
		}
		switch val := v.(type) {
		case string:
			if _, err := fmt.Fprintln(w, val); err != nil {
				return err
			}
		case nil:
			if _, err := fmt.Fprintln(w, "null"); err != nil {
				return err
			}
		default:
			b, err := json.Marshal(val)
			if err != nil {
				return fmt.Errorf("json marshal: %w", err)
			}
			if _, err := fmt.Fprintln(w, string(b)); err != nil {
				return err
			}
		}
	}
	return nil
}
