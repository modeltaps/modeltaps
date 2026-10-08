package controller

import (
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/telegram"
	"github.com/modeltaps/modeltaps/model"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func TelegramBotWebHook(c *gin.Context) {
	handlerFunc := telegram.TGupdater.GetHandlerFunc("/")

	handlerFunc(c.Writer, c.Request)
}

func GetTelegramMenuList(c *gin.Context) {
	var params model.GenericParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	list, err := model.GetTelegramMenusList(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    list,
	})
}

func GetTelegramMenu(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	menu, err := model.GetTelegramMenuById(id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    menu,
	})
}

func AddOrUpdateTelegramMenu(c *gin.Context) {
	menu := model.TelegramMenu{}
	err := c.ShouldBindJSON(&menu)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	defaultMenu := telegram.GetDefaultMenu()
	// 遍历， 禁止有相同的command
	for _, v := range defaultMenu {
		if v.Command == menu.Command {
			common.APIRespondWithError(c, http.StatusOK, errors.New("command already exists"))
			return
		}
	}

	if model.IsTelegramCommandAlreadyTaken(menu.Command, menu.Id) {
		common.APIRespondWithError(c, http.StatusOK, errors.New("command already exists"))
		return
	}

	message := "Added successfully"
	if menu.Id == 0 {
		err = menu.Insert()
	} else {
		err = menu.Update()
		message = "Updated successfully"
	}

	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
	})
}

func DeleteTelegramMenu(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	menu := model.TelegramMenu{Id: id}
	err := menu.Delete()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Deleted successfully",
	})
}

func GetTelegramBotStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"status":     telegram.TGEnabled,
			"is_webhook": telegram.TGWebHookSecret != "",
		},
	})
}

func ReloadTelegramBot(c *gin.Context) {
	err := telegram.ReloadMenuAndCommands()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Reloaded successfully",
	})
}
