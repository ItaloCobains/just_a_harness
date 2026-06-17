package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"harness"
)

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}

func main() {
	tools := []harness.Tool{
		{
			Name:        "add",
			Description: "Add two integers and return their sum",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"a": map[string]any{"type": "integer"},
					"b": map[string]any{"type": "integer"},
				},
				"required": []string{"a", "b"},
			},
			Func: func(input string) string {
				fmt.Fprintf(os.Stderr, "[tool add called] input=%s\n", input)
				var args map[string]any
				if err := json.Unmarshal([]byte(input), &args); err != nil {
					return "error: " + err.Error()
				}
				return fmt.Sprintf("%d", toInt(args["a"])+toInt(args["b"]))
			},
		},
	}

	model := harness.OllamaModel{Model: "llama3.2:3b", Endpoint: "http://localhost:11434"}

	answer, err := harness.Run(model, tools, "Use the add tool to compute 48211 + 91347. Report exactly what the tool returns.")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("answer:", answer)
}
