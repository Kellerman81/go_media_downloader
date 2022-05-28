package models

import (
	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	"github.com/GoAdminGroup/go-admin/template/types"
	"github.com/GoAdminGroup/go-admin/template/types/form"
)

func GetQualitiesTable(ctx *context.Context) table.Table {

	qualities := table.NewDefaultTable(table.DefaultConfigWithDriverAndConnection("sqlite", "media"))

	info := qualities.GetInfo().HideFilterArea()

	info.AddField("Id", "id", db.Integer).FieldSortable()
	//info.AddField("Created_at", "created_at", db.Datetime)
	//info.AddField("Updated_at", "updated_at", db.Datetime)
	info.AddField("Type", "type", db.Integer).FieldDisplay(func(value types.FieldModel) interface{} {
		stringvar := ""
		switch value.Value {
		case "1":
			stringvar = "Resolution"
		case "2":
			stringvar = "Quality"
		case "3":
			stringvar = "Codec"
		case "4":
			stringvar = "Audio"
		}
		return stringvar
	}).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptions(types.FieldOptions{
		{Value: "1", Text: "Resolution"},
		{Value: "2", Text: "Quality"},
		{Value: "3", Text: "Codec"},
		{Value: "4", Text: "Audio"},
	}).FieldFilterOptionExt(map[string]interface{}{"allowClear": true}).FieldSortable()
	info.AddField("Name", "name", db.Text).FieldFilterable().FieldSortable()
	info.AddField("Regex", "regex", db.Text).FieldFilterable().FieldSortable()
	info.AddField("Strings", "strings", db.Text).FieldFilterable().FieldSortable()
	info.AddField("Priority", "priority", db.Integer).FieldFilterable().FieldSortable()
	info.AddField("Use_regex", "use_regex", db.Integer).FieldFilterable(types.FilterType{FormType: form.SelectSingle}).FieldFilterOptions(types.FieldOptions{
		{Value: "0", Text: "No"},
		{Value: "1", Text: "Yes"},
	}).FieldFilterOptionExt(map[string]interface{}{"allowClear": true}).FieldSortable().FieldBool("1", "0")

	info.SetTable("qualities").SetTitle("Qualities").SetDescription("Qualities")

	formList := qualities.GetForm()
	formList.AddField("Id", "id", db.Integer, form.Default).FieldDisplayButCanNotEditWhenCreate().FieldDisableWhenUpdate()
	//formList.AddField("Created_at", "created_at", db.Datetime, form.Datetime)
	//formList.AddField("Updated_at", "updated_at", db.Datetime, form.Datetime)
	formList.AddField("Type", "type", db.Integer, form.SelectSingle).
		FieldOptions(types.FieldOptions{
			{Text: "Resolution", Value: "1"},
			{Text: "Quality", Value: "2"},
			{Text: "Codec", Value: "3"},
			{Text: "Audio", Value: "4"},
		})
	formList.AddField("Name", "name", db.Text, form.Text)
	formList.AddField("Regex", "regex", db.Text, form.Text)
	formList.AddField("Strings", "strings", db.Text, form.Text)
	formList.AddField("Priority", "priority", db.Integer, form.Number)
	formList.AddField("Use_regex", "use_regex", db.Integer, form.Switch).FieldOptions(types.FieldOptions{
		{Text: "Yes", Value: "1"},
		{Text: "No", Value: "0"},
	})

	formList.SetTable("qualities").SetTitle("Qualities").SetDescription("Qualities")

	return qualities
}
