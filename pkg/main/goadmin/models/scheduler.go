package models

import (
	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/parameter"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
)

func getSchedulerTable(ctx *context.Context) (userTable table.Table) {
	userTable = table.NewDefaultTable(ctx, table.Config{})
	userTable.GetOnlyInfo()
	var queue []map[string]any
	i := 0
	for _, value := range worker.GetSchedules() {
		queue = append(queue, map[string]any{
			"id":        i,
			"job":       value.JobName,
			"lastrun":   value.LastRun.Format("2006-01-02 15:04:05"),
			"nextrun":   value.NextRun.Format("2006-01-02 15:04:05"),
			"isrunning": value.IsRunning,
		})
		i++
	}

	info := userTable.GetInfo().SetDefaultPageSize(100)
	info.SetAutoRefresh(60)
	info.HideDeleteButton().
		HideDetailButton().
		HideEditButton().
		HideExportButton().
		HideFilterArea().
		HideFilterButton().
		HideNewButton().
		HidePagination()
	info.AddField("ID", "id", db.Numeric)
	info.AddField("Job", "job", db.Varchar)
	info.AddField("Last Run", "lastrun", db.Datetime)
	info.AddField("Next Run", "nextrun", db.Datetime)
	info.AddField("Is Running", "isrunning", db.Bool).FieldBool("true", "false")

	info.SetTable("Scheduler").
		SetTitle("Scheduler").
		SetDescription("Scheduler").
		SetGetDataFn(func(param parameter.Parameters) (data []map[string]any, size int) {
			param.PK()
			return queue, len(queue)
		})
	return userTable
}
