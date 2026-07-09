package bases

import (
	"fmt"
	"strings"
)

// A2UIResponse represents the declarative JSON structure defined by A2UI protocol.
type A2UIResponse struct {
	Type    string           `json:"type"`              // "table", "card-grid", "list"
	Columns []A2UIColumn     `json:"columns,omitempty"` // For tables
	Rows    []map[string]any `json:"rows,omitempty"`    // For tables
	Items   []A2UIItem       `json:"items,omitempty"`   // For cards and lists
}

type A2UIColumn struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Type  string `json:"type"`
}

type A2UIItem struct {
	Title      string         `json:"title"`
	Subtitle   string         `json:"subtitle,omitempty"`
	Body       string         `json:"body,omitempty"`
	Properties []A2UIProperty `json:"properties,omitempty"`
}

type A2UIProperty struct {
	Label string `json:"label"`
	Value any    `json:"value"`
}

// Execute filters the input notes based on BaseConfig filters, executes custom formula 
// callbacks for computed columns/properties, and returns an A2UIResponse.
func Execute(notes []*Note, config *BaseConfig, runFormula func(funcName string, properties map[string]any) (any, error)) (*A2UIResponse, error) {
	// 1. Filter notes
	var filteredNotes []*Note
	for _, note := range notes {
		if matchFilters(note, config.Filters) {
			filteredNotes = append(filteredNotes, note)
		}
	}

	viewType := config.ViewType
	if viewType == "" {
		viewType = "table"
	}

	resp := &A2UIResponse{
		Type: viewType,
	}

	if viewType == "table" {
		// Identify unique columns
		columnKeysMap := make(map[string]bool)
		var columnOrder []string

		// Add standard property keys from matching notes
		for _, note := range filteredNotes {
			for k := range note.Properties {
				if !columnKeysMap[k] {
					columnKeysMap[k] = true
					columnOrder = append(columnOrder, k)
				}
			}
		}

		// Add formula column keys
		for k := range config.Formulas {
			if !columnKeysMap[k] {
				columnKeysMap[k] = true
				columnOrder = append(columnOrder, k)
			}
		}

		// Build columns definition
		for _, key := range columnOrder {
			label := strings.Title(strings.ReplaceAll(key, "_", " "))
			colType := "string" // default type
			resp.Columns = append(resp.Columns, A2UIColumn{
				Key:   key,
				Label: label,
				Type:  colType,
			})
		}

		// Build rows
		for _, note := range filteredNotes {
			row := make(map[string]any)
			// Copy original properties
			for k, v := range note.Properties {
				row[k] = v
			}
			// Add file path as a default column if not shadowed
			if _, ok := row["file_path"]; !ok {
				row["file_path"] = note.FilePath
			}

			// Evaluate formulas
			for colName, funcName := range config.Formulas {
				val, err := runFormula(funcName, note.Properties)
				if err != nil {
					return nil, fmt.Errorf("failed to evaluate formula '%s': %w", colName, err)
				}
				row[colName] = val
			}

			resp.Rows = append(resp.Rows, row)
		}
	} else {
		// card-grid or list view
		for _, note := range filteredNotes {
			// Title resolution
			title := note.FilePath
			if tVal, ok := note.Properties["title"].(string); ok {
				title = tVal
			} else if tVal, ok := note.Properties["name"].(string); ok {
				title = tVal
			}

			// Subtitle resolution
			var subtitle string
			if sVal, ok := note.Properties["status"].(string); ok {
				subtitle = sVal
			} else if sVal, ok := note.Properties["subtitle"].(string); ok {
				subtitle = sVal
			}

			// Collect properties
			var itemProps []A2UIProperty
			for k, v := range note.Properties {
				itemProps = append(itemProps, A2UIProperty{
					Label: k,
					Value: v,
				})
			}

			// Run formulas
			for colName, funcName := range config.Formulas {
				val, err := runFormula(funcName, note.Properties)
				if err != nil {
					return nil, fmt.Errorf("failed to evaluate formula '%s': %w", colName, err)
				}
				itemProps = append(itemProps, A2UIProperty{
					Label: colName,
					Value: val,
				})
			}

			resp.Items = append(resp.Items, A2UIItem{
				Title:      title,
				Subtitle:   subtitle,
				Body:       note.Content,
				Properties: itemProps,
			})
		}
	}

	return resp, nil
}

func matchFilters(note *Note, filters []FilterCondition) bool {
	for _, cond := range filters {
		val, ok := note.Properties[cond.Property]
		if !ok {
			return false // Property missing means it doesn't match
		}

		if !evaluateCondition(val, cond.Operator, cond.Value) {
			return false
		}
	}
	return true
}

func evaluateCondition(actual any, operator string, expected any) bool {
	switch operator {
	case "eq":
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
	case "ne":
		return fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected)
	case "gt":
		actNum, ok1 := toFloat64(actual)
		expNum, ok2 := toFloat64(expected)
		if ok1 && ok2 {
			return actNum > expNum
		}
		return false
	case "lt":
		actNum, ok1 := toFloat64(actual)
		expNum, ok2 := toFloat64(expected)
		if ok1 && ok2 {
			return actNum < expNum
		}
		return false
	case "contains":
		actStr, ok1 := actual.(string)
		expStr, ok2 := expected.(string)
		if ok1 && ok2 {
			return strings.Contains(strings.ToLower(actStr), strings.ToLower(expStr))
		}
		// If array
		if actArr, ok := actual.([]any); ok {
			for _, item := range actArr {
				if fmt.Sprintf("%v", item) == fmt.Sprintf("%v", expected) {
					return true
				}
			}
		}
		return false
	}
	return false
}

func toFloat64(val any) (float64, bool) {
	switch v := val.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case float32:
		return float64(v), true
	}
	return 0, false
}
