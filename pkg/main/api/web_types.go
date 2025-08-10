package api

// TableInfo holds information about database tables
type TableInfo struct {
	Name      string           `json:"name"`
	Columns   []ColumnInfo     `json:"columns"`
	Rows      []map[string]any `json:"rows"`
	RowsTyped any              `json:"rowsTyped"`
	DeleteURL string           `json:"deleteURL"`
}

// ColumnInfo holds information about table columns
type ColumnInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	DisplayName string `json:"displayName"`
}

// ConfigSection represents a configuration section for display
type ConfigSection struct {
	Name string         `json:"name"`
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

// DataTables response structure
type DataTablesResponse struct {
	Echo                int        `json:"draw"`
	TotalRecords        int        `json:"recordsTotal"`
	TotalDisplayRecords int        `json:"recordsFiltered"`
	Data                [][]string `json:"data"`
}

// DataTables request parameters
type DataTablesRequest struct {
	Echo           int
	DisplayStart   int
	DisplayLength  int
	Search         string
	SortingCols    int
	SortColumns    []int
	SortDirections []string
	Searchable     []bool
	ColumnSearches []string
}

// Mdata represents metadata for form fields
type Mdata struct {
	Mdata any `json:"aaData"`
}

// FilterFieldDef defines a filter field configuration
type FilterFieldDef struct {
	Field        string
	Label        string
	Type         string // text, number, select
	Placeholder  string
	Options      []string // for select type
	OptionLabels []string // labels for select options
}

// FilterMapping defines how filters are mapped to database queries
type FilterMapping struct {
	Column   string
	Operator string // LIKE, =, >=, <=, etc.
}
