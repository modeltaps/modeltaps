package controller

import (
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"
	paymentService "github.com/modeltaps/modeltaps/payment"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetPaymentList(c *gin.Context) {
	var params model.SearchPaymentParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	payments, err := model.GetPanymentList(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if payments.Data != nil {
		for _, payment := range *payments.Data {
			paymentService.HidePaymentSecrets(payment)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    payments,
	})
}

func GetPayment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	payment, err := model.GetPaymentByID(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	paymentService.HidePaymentSecrets(payment)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    payment,
	})
}

func AddPayment(c *gin.Context) {
	payment := model.Payment{}
	err := c.ShouldBindJSON(&payment)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if err := payment.Insert(); err != nil {
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}

	ps, err := paymentService.NewPaymentService(payment.UUID)
	if err != nil {
		if deleteErr := payment.Delete(); deleteErr != nil {
			log.Printf("Failed to delete payment after service creation error: %v", deleteErr)
		}
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}

	if err := ps.CreatedPay(); err != nil {
		if deleteErr := payment.Delete(); deleteErr != nil {
			log.Printf("Failed to delete payment after creation error: %v", deleteErr)
		}
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}

	paymentService.HidePaymentSecrets(&payment)
	c.JSON(http.StatusCreated, common.WithAPICode(gin.H{
		"success": true,
		"message": "Payment added successfully",
		"data":    payment,
	}, common.APICodePaymentAdded, nil))
}

func UpdatePayment(c *gin.Context) {
	payment := model.Payment{}
	err := c.ShouldBindJSON(&payment)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	overwrite := true

	if payment.UUID == "" {
		overwrite = false
	}

	// 读接口下发的是脱敏回显，表单里密钥留空或原样回传都表示不改：用库里的值补齐后再写。
	if payment.Config != "" || overwrite {
		stored, err := model.GetPaymentByID(payment.ID)
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
		payment.Config = paymentService.MergeConfigSecrets(payment.Type, payment.Config, stored.Config)
	}

	err = payment.Update(overwrite)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	paymentService.HidePaymentSecrets(&payment)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    payment,
	})
}

func DeletePayment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	payment := model.Payment{ID: id}
	err = payment.Delete()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func GetUserPaymentList(c *gin.Context) {
	payments, err := model.GetUserPaymentList()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    payments,
	})
}
