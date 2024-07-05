package models

import (
	"fmt"
	"strings"

	"github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/utils"
	"github.com/GoAdminGroup/go-admin/template/types"
	"github.com/GoAdminGroup/go-admin/template/types/action"
)

func myPopUpWithIframe(id, title string, data action.IframeData, width, height string) *action.PopUpAction {
	if id == "" {
		panic("wrong popup action parameter, empty id")
	}
	if data.Width == "" {
		data.Width = "100%"
	}
	if data.Height == "" {
		data.Height = "100%"
	}
	//data.Src = utils.ReplaceAll(data.Src, "{%id}", "{{.Id}}", "{%ids}", "{{.Ids}}")
	//
	//fmt.Println(ctx.Request.Form)
	if strings.ContainsRune(data.Src, '?') {
		data.Src = data.Src + "&"
	} else {
		data.Src = data.Src + "?"
	}
	modalID := "info-popup-model-" + utils.Uuid(10)
	var handler types.Handler = func(ctx *context.Context) (success bool, msg string, res any) {
		param := ""
		if data.AddParameterFn != nil {
			param = data.AddParameterFn(ctx)
		}
		data.Src = strings.Replace(data.Src, "{{.Id}}", ctx.FormValue("id"), 1)
		return true, "ok", fmt.Sprintf(`<iframe style="width:%s;height:%s;" 
			scrolling="auto" 
			allowtransparency="true" 
			frameborder="0"
			src="%s__goadmin_iframe=true&__go_admin_no_animation_=true&__goadmin_iframe_id=%s`+param+`"><iframe>`,
			data.Width, data.Height, data.Src, modalID)
	}
	return &action.PopUpAction{
		Url:        action.URL(id),
		Title:      title,
		Method:     "post",
		BtnTitle:   "",
		Height:     height,
		HasIframe:  true,
		HideFooter: true,
		Width:      width,
		Draggable:  true,
		Data:       action.NewAjaxData(),
		Id:         modalID,
		Handlers:   context.Handlers{handler.Wrap()},
		Event:      action.EventClick,
	}
}
