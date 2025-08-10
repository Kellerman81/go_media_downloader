package api

// createFormFieldDef creates a FormFieldDefinition with common defaults
func createFormFieldDef(name, fieldType string, value any, options map[string][]string) FormFieldDefinition {
	return FormFieldDefinition{
		Name:    name,
		Type:    fieldType,
		Value:   value,
		Options: options,
	}
}

// createTextFieldDef creates a text field definition
func createTextFieldDef(name string, value string) FormFieldDefinition {
	return createFormFieldDef(name, FieldTypeText, value, nil)
}

// createSelectFieldDef creates a select field definition
func createSelectFieldDef(name string, value string, options []string) FormFieldDefinition {
	return createFormFieldDef(name, FieldTypeSelect, value, map[string][]string{"options": options})
}

// createCheckboxFieldDef creates a checkbox field definition
func createCheckboxFieldDef(name string, value bool) FormFieldDefinition {
	return createFormFieldDef(name, FieldTypeCheckbox, value, nil)
}

// createArrayFieldDef creates an array field definition
func createArrayFieldDef(name string, value any) FormFieldDefinition {
	return createFormFieldDef(name, FieldTypeArray, value, nil)
}

// createNumberFieldDef creates a number field definition
func createNumberFieldDef(name string, value any) FormFieldDefinition {
	return createFormFieldDef(name, FieldTypeNumber, value, nil)
}

// Optimized field builder with pre-allocated capacity
type OptimizedFieldBuilder struct {
	fields []FormFieldDefinition
}

// NewOptimizedFieldBuilder creates a new field builder with pre-allocated capacity
func NewOptimizedFieldBuilder(capacity int) *OptimizedFieldBuilder {
	return &OptimizedFieldBuilder{
		fields: make([]FormFieldDefinition, 0, capacity),
	}
}

// AddText adds a text field
func (b *OptimizedFieldBuilder) AddText(name, value string) *OptimizedFieldBuilder {
	b.fields = append(b.fields, createTextFieldDef(name, value))
	return b
}

// AddSelect adds a select field
func (b *OptimizedFieldBuilder) AddSelect(name, value string, options []string) *OptimizedFieldBuilder {
	b.fields = append(b.fields, createSelectFieldDef(name, value, options))
	return b
}

// AddSelectCached adds a select field with cached templates
func (b *OptimizedFieldBuilder) AddSelectCached(name, value, templateType string) *OptimizedFieldBuilder {
	b.fields = append(b.fields, createSelectFieldDef(name, value, getTemplatesWithCache(templateType)))
	return b
}

// AddCheckbox adds a checkbox field
func (b *OptimizedFieldBuilder) AddCheckbox(name string, value bool) *OptimizedFieldBuilder {
	b.fields = append(b.fields, createCheckboxFieldDef(name, value))
	return b
}

// AddNumber adds a number field
func (b *OptimizedFieldBuilder) AddNumber(name string, value any) *OptimizedFieldBuilder {
	b.fields = append(b.fields, createNumberFieldDef(name, value))
	return b
}

// AddArray adds an array field
func (b *OptimizedFieldBuilder) AddArray(name string, value any) *OptimizedFieldBuilder {
	b.fields = append(b.fields, createArrayFieldDef(name, value))
	return b
}

// Build returns the completed field slice
func (b *OptimizedFieldBuilder) Build() []FormFieldDefinition {
	return b.fields
}
