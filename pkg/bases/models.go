package bases

type Note struct {
	FilePath   string
	Properties map[string]any
	Content    string
}

type FilterCondition struct {
	Property string `json:"property" yaml:"property"`
	Operator string `json:"operator" yaml:"operator"` // eq, ne, gt, lt, contains
	Value    any    `json:"value" yaml:"value"`
}

type HostPermissions struct {
	ReadOtherNotes bool `json:"read_other_notes" yaml:"read_other_notes"`
	AccessEnv      bool `json:"access_env" yaml:"access_env"`
}

type BaseConfig struct {
	Filters         []FilterCondition `json:"filters" yaml:"filters"`
	ViewType        string            `json:"view_type" yaml:"view_type"` // table, card, list
	Formulas        map[string]string `json:"formulas" yaml:"formulas"`   // colName -> wasmFuncName
	HostPermissions HostPermissions   `json:"host_permissions" yaml:"host_permissions"`
}
