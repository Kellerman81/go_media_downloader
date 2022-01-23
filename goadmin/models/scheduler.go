package models

import (
	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/parameter"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	"github.com/Kellerman81/go_media_downloader/tasks"
)

func GetSchedulerTable(ctx *context.Context) (userTable table.Table) {

	userTable = table.NewDefaultTable(table.Config{})
	userTable.GetOnlyInfo()
	var queue []map[string]interface{}
	i := 0
	for _, value := range tasks.GlobalSchedules {
		queue = append(queue, map[string]interface{}{
			"id":        i,
			"job":       value.Schedule.JobName,
			"lastrun":   value.Schedule.LastRun.Format("2006-01-02 15:04:05"),
			"nextrun":   value.Schedule.NextRun.Format("2006-01-02 15:04:05"),
			"isrunning": value.Schedule.IsRunning,
		})
		i += 1
	}

	info := userTable.GetInfo().SetDefaultPageSize(100)
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
	info.AddField("Is Running", "isrunning", db.Bool)

	info.SetTable("Scheduler").
		SetTitle("Scheduler").
		SetDescription("Scheduler").
		SetGetDataFn(func(param parameter.Parameters) (data []map[string]interface{}, size int) {
			param.PK()
			return queue, len(queue)
		})
	return userTable
}
